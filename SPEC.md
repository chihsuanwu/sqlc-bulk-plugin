# sqlc Bulk Operation Plugin — 開發規格文件

> 版本 0.8 · 2026-03-30
>
> 變更紀錄：
> - v0.1 → v0.2：根據 spike 驗證結果調整標記機制與 struct mapping 策略
> - v0.2 → v0.3：確立 input struct 複用規則——全欄位時複用 sqlc model struct，部分欄位時生成獨立 Item struct，不跨 query 共用（與 sqlc 原生行為一致）
> - v0.3 → v0.4：SQL 範例統一使用標準 `$N` positional parameter 語法；新增 FR-7 UNNEST alias 解析機制，由 plugin 從 SQL 文字中提取欄位名稱
> - v0.4 → v0.5：修正 nullable 判斷來源——params 的 `not_null` 永遠為 `true`（反映 UNNEST 表達式特性），nullable 必須從 catalog 比對取得
> - v0.5 → v0.6：新增 FR-8 目標 table 解析；修正 adapter code 範例中的 params field 命名（`$N` 語法對應 `ColumnN`）；新增 Params field 引用規則；附錄補充 sqlc 實際生成的 struct 與 Settings 結構
> - v0.6 → v0.7：確立第一階段僅支援 sqlc 預設設定；`rename`、`overrides`、`emit_pointers_for_null_types` 等非預設設定列入已知限制
> - v0.7 → v0.8：Phase 1 實作完成；修正 FR-7——sqlc 會將 `@param` 轉為 `$N` in Query.Text，UNNEST alias 解析永遠需要執行

---

## 1. 背景與問題

### 1.1 現狀

在使用 sqlc 進行 PostgreSQL 開發時，bulk UPDATE 與 bulk Upsert 是常見的資料操作需求。sqlc 目前對這兩種操作的 code generation 支援不完整，開發者必須手動處理 type 轉換。

以下是一個典型的 bulk UPDATE SQL：

```sql
-- name: BulkUpdateTRATrain :exec
UPDATE tra_daily_timetable AS t SET
    direction = u.direction,
    type_id   = u.type_id,
    ...
FROM (
    SELECT
        UNNEST($1::int[])      AS id,
        UNNEST($2::smallint[]) AS direction,
        UNNEST($3::text[])     AS type_id,
        ...                    -- 13 個欄位
) AS u
WHERE t.id = u.id;
```

sqlc 會為這個 query 生成以下簽章：

```go
type BulkUpdateTRATrainParams struct {
    Column1 []int32
    Column2 []int16
    Column3 []string
    // ... 每個欄位對應一個 slice
}
```

### 1.2 核心問題

sqlc 生成的 Params struct 將每個欄位拆成獨立的 slice，與應用程式使用的 data model struct 格式不同，需要額外撰寫 adapter 做轉換：

| 應用程式使用的 Data Model | sqlc 生成的 Params（現狀） |
|---|---|
| `[]TraDailyTimetable{{ID: 1, Direction: 0, TypeID: "123", ...}}` | `BulkUpdateTRATrainParams{IDs: []int32{1}, Directions: []int16{0}, TypeIDs: []string{"123"}, ...}` |

每新增一個 bulk operation 就要維護一份轉換 code，欄位異動時容易漏改。

### 1.3 為什麼選擇 UNNEST 策略

評估過三種策略後，UNNEST 最適合本專案：

| 策略 | DB 端執行次數 | Parameter 上限 | 備註 |
|---|---|---|---|
| **UNNEST（選擇）** | 1 次 | 不會碰到 | SQL 不變；只需生成 adapter |
| SendBatch | N 次 | 13 欄 → 約 2,500 筆 | 筆數多時有上限風險 |
| CopyFrom + Temp Table | 2 次 query | 不會碰到 | 僅支援 INSERT，不支援 UPDATE |

PostgreSQL 有 32,767 個 parameterized variable 的上限。以 13 欄的 UPDATE 為例，使用 SendBatch 最多只能批次約 2,500 筆；UNNEST 每欄只佔一個 parameter，完全不受此限制。

---

## 2. 需求

### 2.1 功能需求

#### FR-1　自動生成 bulk adapter

- 針對標記的 sqlc query，自動生成對應的 Go adapter function
- Adapter 接受 sqlc 從 catalog 推導的 row struct slice，不需要手動拆欄
- 生成的 code 與 sqlc 原本的輸出並列，不覆蓋現有檔案

#### FR-2　支援 Bulk UPDATE

- 對應 SQL 使用 `UPDATE ... FROM (SELECT UNNEST(...))` 語法
- 生成 adapter 將 `[]Struct` 轉換為各欄位對應的 `[]Type`，並呼叫 sqlc 生成的原始 function

#### FR-3　支援 Bulk Upsert

- 對應 SQL 使用 `INSERT ... ON CONFLICT DO UPDATE` 搭配 UNNEST 語法
- 同 FR-2，生成 adapter 負責欄位拆解
- **第一階段先實作 FR-2（Bulk UPDATE），FR-3 在後續 iteration 補上**

#### FR-4　透過 SQL comment annotation 標記觸發

> **v0.2 變更：** 原設計使用自訂 `:bulkupdate` command tag，經 spike 驗證 sqlc 不接受非標準的 command type（報 `invalid query type`）。改為使用 comment annotation。

使用 `-- @bulk` comment annotation 搭配標準 `:exec` command 標記需要生成 adapter 的 query：

```sql
-- @bulk update
-- name: BulkUpdateProducts :exec
UPDATE products AS p SET
    name  = u.name,
    price = u.price
FROM (
    SELECT
        UNNEST($1::int[])  AS id,
        UNNEST($2::text[]) AS name,
        UNNEST($3::int[])  AS price
) AS u
WHERE p.id = u.id;
```

```sql
-- @bulk upsert
-- name: UpsertProducts :exec
INSERT INTO products (id, name, price)
SELECT * FROM UNNEST($1::int[], $2::text[], $3::int[])
ON CONFLICT (id) DO UPDATE SET
    name  = EXCLUDED.name,
    price = EXCLUDED.price;
```

**標記規則：**
- `-- @bulk update` → 生成 bulk UPDATE adapter
- `-- @bulk upsert` → 生成 bulk Upsert adapter（第二階段）
- 未標記的 query 不受影響，plugin 直接略過

**Spike 驗證結果：** sqlc 會將 query 上方的 comments 原封不動傳入 `Query.Comments` 陣列，plugin 可從中掃描 `@bulk` 標記。

#### FR-7　UNNEST Alias 解析（從 SQL 文字提取欄位名稱）

> **v0.4 新增：** 經 spike 驗證，使用 `$N` positional parameter 時，sqlc 傳給 plugin 的 `Query.Params` 不包含 `column.name`（欄位為空）。但 SQL 文字中的 `UNNEST($1::int[]) AS id` alias 仍然存在於 `Query.Text`。Plugin 需自行解析。

**問題：** 使用標準 `$N` 語法時，sqlc 的 `Query.Params[].column.name` 為空：

| SQL 語法 | `Query.Params` 中的 `column.name` |
|---|---|
| `UNNEST(@ids::int[]) AS id` | `"ids"` ✅ sqlc 提供 |
| `UNNEST($1::int[]) AS id` | 空 ❌ sqlc 不提供 |

**解法：** Plugin 從 `Query.Text` 中以 regex 提取 `$N` → column alias 的對應：

```
UNNEST\(\$(\d+)::\w+\[\]\)\s+AS\s+(\w+)
```

範例：對 SQL `UNNEST($1::int[]) AS id, UNNEST($2::text[]) AS name`，提取出：

| Parameter | Alias |
|---|---|
| `$1` | `id` |
| `$2` | `name` |

再以 alias 去 `Catalog` 比對 table column，取得 `not_null` 等資訊。

**實作補充（v0.8）：**
- sqlc 會將 `@param_name` 轉換為 `$N` 後再傳入 `Query.Text`，因此 UNNEST alias regex 對兩種語法都適用
- UNNEST alias 解析**永遠**需要執行（不只是 `$N` 語法），因為 `@param` 語法的 `column.name` 是 param name（如 `"ids"`），而非 table column name（如 `"id"`）。Catalog lookup 需要的是 column name
- `column.name` 僅用於決定 Params struct 的 field 命名（`ColumnN` vs `PascalCase(name)`）

#### FR-8　目標 Table 解析

> **v0.6 新增：** Plugin 需要知道 query 的目標 table 才能從 Catalog 查詢 column 定義（判斷 nullable、比對全欄位等）。Spike 驗證發現 UPDATE 和 Upsert 的取得方式不同。

| Query 類型 | 目標 table 來源 |
|---|---|
| Upsert（INSERT ON CONFLICT） | `Query.InsertIntoTable.Name` — sqlc 直接提供 |
| UPDATE | sqlc **不提供** — 需從 `Query.Text` 解析 |

**UPDATE table 解析：** 從 SQL 文字中以 regex 提取：

```
UPDATE\s+(\w+)
```

範例：`UPDATE products AS p SET ...` → `products`

取得 table name 後，在 `Catalog` 的 `public` schema（`default_schema`）中查找對應的 table 定義。

#### FR-5　Input Struct 策略與生成的 function 簽章

> **v0.2 變更：** 原設計假設使用應用程式自訂的 data model struct。經 spike 確認 plugin 收到的 `GenerateRequest` 包含完整的 catalog（table schema），但不包含 sqlc 生成的 model struct 定義。改為由 plugin 自行從 catalog 推導 input struct。
>
> **v0.3 補充：** 經實際檢驗 sqlc 在 TransTaiwan 專案中的行為，確認 sqlc 不會跨 query 共用 struct——即使多個 query 的欄位完全相同，也各自生成獨立的 `XxxRow` / `XxxParams`。Plugin 的 struct 複用策略依循相同原則。

**Struct 複用規則：**

| 情境 | Plugin 行為 | 範例 |
|---|---|---|
| Query params 涵蓋 table 全部欄位 | 複用 sqlc 的 model struct（`models.go` 中的 table struct） | `items []Product` |
| Query params 只涵蓋部分欄位 | 生成獨立的 `XxxItem` struct | `items []BulkUpdateProductsItem` |
| 多個 bulk query 使用相同的部分欄位 | 各自生成獨立的 `XxxItem` struct，不跨 query 共用 | 與 sqlc 行為一致 |

此設計與 sqlc 原生行為一致——sqlc 自身在 partial SELECT 和 params 上也是每個 query 獨立生成 struct，不做跨 query 複用。

> **⚠️ 第一階段限制（v0.7）：** Model struct 的複用假設使用者的 sqlc 設定為預設值。Spike 驗證發現 `rename`、`overrides`、`emit_pointers_for_null_types` 等設定會改變 model struct 的 field name 或 type，但 plugin 完全無法從 `GenerateRequest` 中感知這些設定（Settings 不包含 `gen.go` 的設定）。因此：
> - 若使用者有 `rename` 規則，model struct 的 field name 可能與 plugin 預期不同（如 `ID` → `ProductID`）
> - 若使用者有 `overrides` 或 `emit_pointers_for_null_types`，model struct 的 field type 可能不同（如 `pgtype.Text` → `*string` 或 `string`）
>
> **第一階段不處理這些情境。** 使用非預設設定的使用者需自行確認相容性，或改用 `XxxItem` struct（不依賴 model struct 複用）。後續可透過 `plugin_options` 讓使用者傳入必要的設定資訊。

**前提條件：** Plugin 的 output 目錄必須與 sqlc 的 Go codegen output 在同一個 package 內，才能直接引用 model struct 與 `XxxParams` struct。`sqlc.yaml` 設定範例：

```yaml
sql:
  - schema: schema.sql
    queries: query.sql
    engine: postgresql
    gen:
      go:
        package: db
        out: gen
    codegen:
      - plugin: bulk
        out: gen          # 與 gen.go.out 相同 → 同一個 package
```

**情境一：全欄位 + `$N` 語法 — 複用 model struct**

sqlc 生成的 Params struct（`$N` 語法）：
```go
// sqlc 生成（query.sql.go）
type BulkUpdateProductsParams struct {
    Column1 []int32              // $1 → id
    Column2 []string             // $2 → name
    Column3 []int32              // $3 → price
    Column4 []string             // $4 → category（nullable 但 sqlc 用 []string）
    Column5 []bool               // $5 → is_active
    Column6 []pgtype.Timestamptz // $6 → updated_at
}
```

Plugin 生成的 adapter：
```go
// Product 是 sqlc 在 models.go 中生成的 model struct，plugin 直接引用
func (q *Queries) BulkUpdateProductsBatch(
    ctx context.Context,
    items []Product,
) error {
    params := BulkUpdateProductsParams{
        Column1: make([]int32, len(items)),
        Column2: make([]string, len(items)),
        Column3: make([]int32, len(items)),
        Column4: make([]string, len(items)),
        Column5: make([]bool, len(items)),
        Column6: make([]pgtype.Timestamptz, len(items)),
    }
    for i, item := range items {
        params.Column1[i] = item.ID
        params.Column2[i] = item.Name
        params.Column3[i] = item.Price
        params.Column4[i] = item.Category.String // pgtype.Text → string 轉換
        params.Column5[i] = item.IsActive
        params.Column6[i] = item.UpdatedAt
    }
    return q.BulkUpdateProducts(ctx, params)
}
```

**情境二：部分欄位 + `$N` 語法 — 生成獨立 Item struct**

```go
// BulkUpdateProductPricesItem 由 plugin 生成，僅包含該 query 使用的欄位
type BulkUpdateProductPricesItem struct {
    ID        int32
    Price     int32
    UpdatedAt pgtype.Timestamptz
}

func (q *Queries) BulkUpdateProductPricesBatch(
    ctx context.Context,
    items []BulkUpdateProductPricesItem,
) error {
    params := BulkUpdateProductPricesParams{
        Column1: make([]int32, len(items)),
        Column2: make([]int32, len(items)),
        Column3: make([]pgtype.Timestamptz, len(items)),
    }
    for i, item := range items {
        params.Column1[i] = item.ID
        params.Column2[i] = item.Price
        params.Column3[i] = item.UpdatedAt
    }
    return q.BulkUpdateProductPrices(ctx, params)
}
```

**情境三：使用 `@param` 語法 — Params field 有意義命名**

```go
// sqlc 生成的 Params struct 有命名（@param 語法）
type BulkUpdateProductNamesParams struct {
    Ids   []int32
    Names []string
}

// Plugin 生成的 adapter
func (q *Queries) BulkUpdateProductNamesBatch(
    ctx context.Context,
    items []BulkUpdateProductNamesItem,
) error {
    params := BulkUpdateProductNamesParams{
        Ids:   make([]int32, len(items)),
        Names: make([]string, len(items)),
    }
    for i, item := range items {
        params.Ids[i]   = item.ID
        params.Names[i] = item.Name
    }
    return q.BulkUpdateProductNames(ctx, params)
}
```

**Struct 推導邏輯：**
1. 取得目標 table：Upsert 用 `Query.InsertIntoTable.Name`；UPDATE 從 `Query.Text` 解析（FR-8）
2. 取得每個 parameter 的欄位名稱：優先使用 `Query.Params[].column.name`（`@param` 語法時有值）；若為空則透過 FR-7 從 `Query.Text` 解析 UNNEST alias
3. 從 `Query.Params` 取得每個 parameter 的 PG type
4. 從 `Catalog`（`public` schema）找到目標 table 的 column 定義，以步驟 2 的欄位名稱比對，取得 `not_null` 資訊（**不可使用 params 自身的 `not_null`**，見下方說明）
5. 比對 query params 與 table columns：若完全一致則複用 model struct name，否則生成 `XxxItem`
6. 將 PG array type 還原為單值 type（`int4[]` → `int32`，`text[]` → `string`）
7. Nullable 欄位使用 `pgtype.*`（如 `pgtype.Int4`、`pgtype.Text`）

**Params field 引用規則：**

Plugin 生成的 adapter 需引用 sqlc 生成的 `XxxParams` struct 的 field。命名規則取決於 SQL 中的 parameter 語法：

| SQL 語法 | sqlc 生成的 Params field | Plugin 引用方式 |
|---|---|---|
| `$1`, `$2`, ... | `Column1`, `Column2`, ... | 以 `Query.Params[].number` 組合 `Column` 前綴 |
| `@ids`, `@names`, ... | `Ids`, `Names`, ... | 以 `Query.Params[].column.name` 做 PascalCase 轉換 |

判斷方式：若 `Query.Params[0].column.name` 為空，則為 `$N` 語法，使用 `ColumnN`；否則為 `@param` 語法，使用 PascalCase(name)。

**Nullable 欄位的 type 轉換注意事項：**

> **v0.6 補充：** 使用 `$N` 語法時，sqlc 生成的 Params struct 中 nullable 欄位的 array 型別可能**不使用** `pgtype.*`。例如 `category TEXT`（nullable）在 Params 中型別為 `[]string` 而非 `[]pgtype.Text`，但在 model struct 中為 `pgtype.Text`。當複用 model struct 時，adapter 需處理 `pgtype.Text` → `string` 的轉換。

> **⚠️ Nullable 判斷注意事項（v0.5）：** Spike 驗證發現，`Query.Params[].column.not_null` 對 UNNEST 參數**永遠為 `true`**，即使對應的 table column 允許 NULL（如 `category TEXT` 無 NOT NULL 約束）。這是因為 `not_null` 反映的是 UNNEST 表達式本身的特性，而非原始 table column 的約束。因此 **nullable 判斷必須從 `Catalog` 中的 table column 定義取得**，不能依賴 params。

**Spike 驗證結果：** 使用 `@param` 語法時，params 帶有完整的 `column.name`；使用 `$N` 語法時 `column.name` 為空，但 `column.type.name`、`column.is_array` 仍完整可用。`Query.Text` 中的 UNNEST alias 可作為 fallback 來源。Catalog 的 `public` schema 下有 user table 定義（系統表在 `pg_catalog` 和 `information_schema` 下），可用於全欄位比對與 nullable 判斷。

#### FR-6　與 sqlc Querier interface 整合

生成對應的 `BulkQuerier` interface，方便 mock 與測試：

```go
type BulkQuerier interface {
    BulkUpdateProductsBatch(ctx context.Context, items []Product) error
    BulkUpdateProductPricesBatch(ctx context.Context, items []BulkUpdateProductPricesItem) error
    // 未來加入 UpsertXxxBatch ...
}

// 確保 *Queries 實作 BulkQuerier
var _ BulkQuerier = (*Queries)(nil)
```

### 2.2 非功能需求

- 生成的 code 可直接通過 `go build` 與 `go vet`，不需要額外人工修改
- Plugin 以 process plugin（standalone binary）形式發布，不需要 WASM 編譯環境
- 僅支援 PostgreSQL + pgx/v5（本專案目前使用的 driver）
- Nullable 欄位以 `pgtype.*` 為主（如 `pgtype.Int4`、`pgtype.Text`）
- 不改動 sqlc 本身的 generate 流程，以 codegen plugin 附加執行
- 發布為獨立的 open-source repository，提供 README、CI 與 release binary

---

## 3. 開發範圍

### 3.1 Plugin 架構

sqlc plugin 的運作方式：sqlc 將解析完的 schema 與 queries 序列化為 protobuf，透過 stdin 傳入 plugin binary；plugin 生成 code 後將結果透過 stdout 回傳。

Plugin 使用 `plugin-sdk-go` 提供的 `codegen.Run()` helper，不需要手動處理 stdin/stdout：

```go
func main() {
    codegen.Run(func(ctx context.Context, req *plugin.GenerateRequest) (*plugin.GenerateResponse, error) {
        resp := generate(req) // 核心邏輯
        return resp, nil
    })
}
```

`GenerateRequest` 包含 `Settings`、`Catalog`（schema 資訊）、`Queries`（所有 query）；`GenerateResponse` 回傳一組 `File`，每個 File 有 name 和 contents（生成的 Go code）。

### 3.2 開發工作項目

#### Task 1　Plugin 骨架

- ~~建立 Go module，引入 `github.com/sqlc-dev/plugin-sdk-go`~~ ✅ Spike 已完成
- ~~實作 stdin → protobuf → stdout 的基本 pipeline~~ ✅ Spike 已完成
- 加入 `sqlc.yaml` 設定範例與本地測試腳本
- 重構 spike code，從 JSON dump 模式改為正式的 generate pipeline

#### Task 2　Query 解析與過濾

- 掃描 `req.Queries`，找出 `Comments` 中包含 `@bulk update`（或 `@bulk upsert`）的 query
- 取得目標 table：Upsert 用 `Query.InsertIntoTable.Name`；UPDATE 從 `Query.Text` 以 regex 解析（FR-8）
- 取得欄位名稱：優先用 `Query.Params[].column.name`；為空時從 `Query.Text` 以 regex 解析 UNNEST alias（FR-7）
- 判斷 parameter 語法（`$N` vs `@param`），決定 Params field 引用方式（`ColumnN` vs PascalCase）
- 從 `Query.Params` 取得每個欄位的 PG type
- 建立 param name → table column 的 mapping（利用 `Catalog` 的 `public` schema）
- 處理 nullable 欄位的識別（透過 `Catalog` 中 column 的 `not_null` 欄位，不可用 params 的 `not_null`）

#### Task 3　Adapter Code 生成

- 設計 PG type → Go type 的 mapping（`int4` → `int32`、`text` → `string`、`bool` → `bool`、`timestamptz` → `pgtype.Timestamptz`，nullable 欄位用 `pgtype.*`）
- 設計 `text/template`，模板接受 query metadata，輸出：
  - Input struct（`XxxItem`）定義
  - Adapter function（`XxxBatch`）
  - `BulkQuerier` interface 與 compile-time check
- 生成正確的 import 清單，避免 unused import 造成編譯錯誤

#### Task 4　整合測試

- 建立 testdata：`schema.sql` + `query.sql` + 預期生成的 `.go` golden file
- 撰寫 golden file test，確保輸出穩定
- 測試案例涵蓋：全 NOT NULL 欄位、含 nullable 欄位、多個 bulk query 共存

#### Task 5　文件與打包

- 撰寫 README：安裝方式、`sqlc.yaml` 設定、`@bulk` annotation 語法、PG type → Go type 對應表
- 設定 GitHub Actions：CI 跑 golden test + release binary 自動發布

### 3.3 不在本次範圍內

- MySQL / SQLite 支援
- WASM plugin 格式
- SendBatch 或 CopyFrom 策略
- 自動偵測 query 語法（不需標記 comment）的智慧模式
- `@bulk upsert` 支援（第一階段暫不實作，介面預留）
- 搭配 `rename`、`overrides`、`emit_pointers_for_null_types` 等非預設 sqlc 設定的支援
- ~~自訂 `:bulkupdate` command tag~~ ← 已驗證不可行

---

## 4. 風險與限制

| 風險 | 等級 | 對應方式 |
|---|---|---|
| sqlc protobuf schema 版本更新 | 低 | pin 特定版本的 plugin-sdk-go；升級時跑 golden test 確認 |
| pgtype 欄位 type mapping 不完整 | 中 | 先支援 `int4 / int2 / int8 / text / bool / timestamptz / float4 / float8`，其餘遇到再補，並在 README 列出支援清單 |
| UNNEST alias 解析失敗 | 中 | SQL 寫法可能不符合預期的 `UNNEST($N::type[]) AS name` 格式。若 regex 無法提取 alias，plugin 報錯並提示使用者改用 `@param_name` 語法或調整 SQL 格式 |
| UPDATE 目標 table 解析失敗 | 低 | SQL 寫法可能有 schema prefix（如 `public.products`）或不常見格式。先支援 `UPDATE table_name` 和 `UPDATE table_name AS alias`，其餘格式報錯 |
| Param name → table column mapping 失敗 | 中 | Alias 名稱可能與 table column name 不一致。若無法 match，fallback 為全部標記 NOT NULL（使用基本 Go type），並在生成的 code 加上 comment 提示 |
| Nullable 欄位的 Params type 與 Model type 不一致 | 中 | 使用 `$N` 語法時，sqlc 的 Params 中 nullable 欄位可能用基本型別（如 `[]string`）而非 `pgtype.*`，但 model struct 用 `pgtype.Text`。複用 model struct 時 adapter 需處理型別轉換 |
| BulkQuerier interface 與未來 sqlc 官方支援衝突 | 低 | interface 命名加 `Bulk` prefix 與官方 `Querier` 區隔 |
| ~~自訂 command tag 不被 sqlc 接受~~ | ~~高~~ | ✅ 已驗證並改為 comment annotation 方案 |

---

## 附錄 A：Spike 驗證結果

> 2026-03-30 執行，sqlc v1.29.0 + plugin-sdk-go v1.23.0

### A.1 自訂 command tag

```
$ sqlc generate
# package bulk
testdata/query.sql:22:1: invalid query type: :bulkupdate
```

**結論：** sqlc 不接受非標準的 command type，必須使用 `:exec` 搭配 comment annotation。

### A.2 Plugin 收到的 Query.Params 結構

**使用 `@param_name` 語法時：**

```json
{
  "number": 1,
  "column": {
    "name": "ids",
    "not_null": true,
    "is_array": true,
    "length": -1,
    "type": { "schema": "pg_catalog", "name": "int4" },
    "array_dims": 1
  }
}
```

`column.name` 有值（`"ids"`），可直接使用。

**使用 `$N` positional 語法時：**

```json
{
  "number": 1,
  "column": {
    "not_null": true,
    "is_array": true,
    "length": -1,
    "type": { "schema": "pg_catalog", "name": "int4" },
    "array_dims": 1
  }
}
```

`column.name` 為空（欄位不存在），但 type、is_array 仍完整。欄位名稱需從 `Query.Text` 中的 UNNEST alias 提取（見 FR-7）。

**⚠️ `not_null` 陷阱：** 所有 UNNEST 參數的 `not_null` 均為 `true`，包括對應 nullable column 的參數（如 `category TEXT` 無 NOT NULL 約束）。這是因為 `not_null` 反映 UNNEST 表達式特性，而非 table column 約束。Nullable 判斷必須從 Catalog 取得。

### A.3 Catalog 中的 Table Schema

Plugin 收到完整的 table 定義，包含每個 column 的 name、type、not_null。User table 位於 `public` schema（`default_schema: "public"`）下，可與 `pg_catalog`（139 tables）、`information_schema`（69 tables）區隔。

### A.4 sqlc 生成的 Struct 實際範例

**Model struct**（`models.go`）— table name `products` → struct name `Product`（單數、PascalCase）：

```go
type Product struct {
    ID        int32
    Name      string
    Price     int32
    Category  pgtype.Text        // nullable → pgtype.Text
    IsActive  bool
    UpdatedAt pgtype.Timestamptz // timestamptz → 不管 nullable 與否都用 pgtype
}
```

**Params struct — `$N` 語法**（`query.sql.go`）：

```go
type BulkUpdateProductsParams struct {
    Column1 []int32              // $1 → id
    Column2 []string             // $2 → name
    Column3 []int32              // $3 → price
    Column4 []string             // $4 → category（nullable 但用 []string，非 []pgtype.Text）
    Column5 []bool               // $5 → is_active
    Column6 []pgtype.Timestamptz // $6 → updated_at
}
```

**Params struct — `@param` 語法**（`query.sql.go`）：

```go
type BulkUpdateProductNamesParams struct {
    Ids   []int32  // @ids
    Names []string // @names
}
```

**關鍵觀察：**
- `$N` → `ColumnN`；`@param_name` → `PascalCase(name)`
- Nullable 欄位在 Params 中不一定用 `pgtype.*`（如 `category` 用 `[]string`），但在 Model 中一定用（`pgtype.Text`）
- `timestamptz` 不管在 Params 或 Model 中都用 `pgtype.Timestamptz`

### A.5 Settings 結構

Plugin 收到的 `Settings` 僅包含與該 codegen block 相關的設定，不包含 `gen.go` 的 package name 或 rename rules：

```json
{
  "version": "2",
  "engine": "postgresql",
  "schema": ["testdata/schema.sql"],
  "queries": ["testdata/query.sql"],
  "codegen": {
    "out": "gen",
    "plugin": "bulk",
    "process": { "cmd": "./sqlc-bulk-plugin" }
  }
}
```

Plugin 無法從 Settings 取得 sqlc Go codegen 的 package name。因此 plugin 需要透過自己的 `plugin_options` 或預設慣例來決定 package name。

### A.6 UPDATE vs Upsert 的目標 Table 資訊

| Query 類型 | protobuf 中的 table 資訊 |
|---|---|
| Upsert | `insert_into_table: {"name": "products"}` ✅ |
| UPDATE | 無任何 table 欄位 ❌ — 需從 `Query.Text` 解析 |

### A.7 sqlc.yaml 設定對 Plugin 的影響

> 2026-03-30 驗證

測試了三種非預設設定對 model struct、params struct、plugin 可見性的影響：

| 設定 | Model struct 影響 | Params struct 影響 | Plugin 能否感知 |
|---|---|---|---|
| `emit_pointers_for_null_types: true` | `pgtype.Text` → `*string` | 不變（`[]string`） | ❌ |
| `rename: {id: "ProductID"}` | `ID` → `ProductID` | 不變（`ColumnN`） | ❌ |
| `overrides: [{column: "products.category", go_type: "string"}]` | `pgtype.Text` → `string` | 不變（`[]string`） | ❌ |

**結論：**
- 這些設定只影響 sqlc 的 Go codegen 輸出（model struct），不影響 Params struct
- Plugin 的 `Settings`、`global_options`、`plugin_options` 在所有測試中均為空，完全無法感知 `gen.go` 下的設定
- Model struct 複用在非預設設定下可能產生 field name 或 type 不匹配，第一階段不處理
