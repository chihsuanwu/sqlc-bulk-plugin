# sqlc Bulk Operation Plugin — 設計規格

> 版本 1.0
>
> 歷史變更紀錄見 [docs/changelog.md](docs/changelog.md)
> Spike 驗證結果見 [docs/spike-results.md](docs/spike-results.md)

---

## 1. 背景與問題

sqlc 對 bulk UPDATE / Upsert 使用 UNNEST 語法時，生成的 Params struct 是 column-oriented（每個欄位一個 slice），與應用程式常用的 row-oriented `[]Struct` 格式不同。每個 bulk query 都需要手動撰寫轉換 adapter，欄位異動時容易漏改。

本 plugin 自動生成這些 adapter function。

**為什麼選擇 UNNEST：** PostgreSQL 有 32,767 個 parameterized variable 上限，SendBatch（每列一次 query）在欄位多時會碰到上限。UNNEST 每欄只佔一個 parameter，無此限制。純 bulk INSERT 可使用 sqlc 內建的 `:copyfrom`。

---

## 2. 功能需求

### FR-1　自動生成 bulk adapter

- 針對標記的 sqlc query，自動生成 Go adapter function
- Adapter 接受 row-oriented struct slice，轉換為 column-oriented Params
- 生成的 code 輸出至 `bulk.go`，與 sqlc 原本的輸出並列

### FR-2　Bulk UPDATE

對應 SQL 使用 `UPDATE ... FROM (SELECT UNNEST(...) AS alias)` 語法。

### FR-3　Bulk Upsert

對應 SQL 使用 `INSERT ... VALUES (UNNEST(...)) ON CONFLICT ...` 語法。

**與 UPDATE 的解析差異：**

| | UPDATE | Upsert (VALUES) |
|---|---|---|
| 欄位名稱來源 | UNNEST alias（FR-7） | INSERT column list（FR-9） |
| 目標 table 來源 | regex 從 `Query.Text` 解析（FR-8） | `Query.InsertIntoTable.Name` |
| Params / Struct / 轉換邏輯 | 共用 | 共用 |

### FR-4　Comment annotation 標記

使用 `-- @bulk` annotation 搭配 `:exec` command：

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
VALUES (
    UNNEST($1::int[]),
    UNNEST($2::text[]),
    UNNEST($3::int[])
)
ON CONFLICT (id) DO UPDATE SET
    name  = EXCLUDED.name,
    price = EXCLUDED.price;
```

### FR-5　Input struct 策略

| 情境 | Plugin 行為 | 範例 |
|---|---|---|
| Query params 涵蓋 table 全部欄位 | 複用 sqlc 的 model struct | `items []Product` |
| Query params 只涵蓋部分欄位 | 生成獨立的 `XxxItem` struct | `items []BulkUpdateProductsItem` |

每個 query 獨立生成 struct，不跨 query 共用（與 sqlc 行為一致）。

**Params field 引用規則：**

| SQL 語法 | Params field | 判斷方式 |
|---|---|---|
| `$1`, `$2`, ... | `Column1`, `Column2`, ... | `Query.Params[0].column.name` 為空 |
| `@ids`, `@names`, ... | `Ids`, `Names`, ... | `Query.Params[0].column.name` 有值 |

**Nullable 判斷：** 必須從 `Catalog` 中的 table column 定義取得（`not_null` 欄位）。`Query.Params[].column.not_null` 對 UNNEST 參數永遠為 `true`，不可使用。

**Nullable 型別轉換：** sqlc 的 Params 中 nullable 欄位可能用基本型別（如 `[]string`），但 model struct 用 `pgtype.Text`。複用 model struct 時，adapter 透過 pgtype accessor（如 `.String`、`.Int32`）處理轉換。

### FR-6　生成模式（`style` 參數）

```yaml
options:
  style: function  # "function"（預設）| "method" | "interface"
```

| `style` | 生成內容 | 適用情境 |
|---|---|---|
| `function`（預設） | Standalone function（接受 `Querier`） | 使用 `emit_interface: true` |
| `method` | `*Queries` method | 直接操作 `*Queries` |
| `interface` | Method + `BulkQuerier` interface + compile-time check | 願意改用組合 interface |

### FR-7　UNNEST Alias 解析（UPDATE 用）

從 `Query.Text` 以 regex 提取 `$N` → column alias 對應（永遠需要執行，即使 `@param` 語法）：

```
UNNEST\(\$(\d+)::\w+(?:\[\])?\)\s+AS\s+(\w+)
```

### FR-8　UPDATE 目標 Table 解析

從 `Query.Text` 以 regex 提取 table name：

```
UPDATE\s+(\w+)
```

### FR-9　INSERT Column List 解析（Upsert 用）

Upsert 的 UNNEST 在 VALUES 中沒有 `AS alias`，欄位名稱從 INSERT column list 提取：

```
INSERT\s+INTO\s+\w+\s*\(([^)]+)\)
```

提取 column list 後，按位置與 VALUES 中 UNNEST 的 `$N` 對應。

VALUES 中的 UNNEST 用以下 regex 提取（不要求 `AS alias`）：

```
UNNEST\(\$(\d+)::\w+(?:\[\])?\)
```

函式包裝（如 `NULLIF(UNNEST(...), 0)`）不影響匹配，regex 匹配內部 UNNEST 子字串。

---

## 3. 非功能需求

- 生成的 code 可直接通過 `go build` 與 `go vet`
- Process plugin（standalone binary），不需 WASM
- PostgreSQL + pgx/v5 only
- Nullable 欄位使用 `pgtype.*`
- Plugin output 必須與 sqlc Go codegen output 在同一 package

---

## 4. 已知限制

- `rename`、`overrides`、`emit_pointers_for_null_types` 等非預設 sqlc 設定不支援（plugin 無法感知 `gen.go` 設定）
- `:many` + `RETURNING` 的 bulk query 不支援（目前僅處理 `:exec`）

---

## 5. 風險

| 風險 | 等級 | 對應方式 |
|---|---|---|
| sqlc protobuf schema 版本更新 | 低 | pin plugin-sdk-go 版本；升級時跑 golden test 確認 |
| PG type mapping 不完整 | 中 | 先支援常見型別，其餘遇到再補，README 列出支援清單 |
| UNNEST alias 解析失敗（UPDATE） | 中 | SQL 不符合 regex 格式時報錯，提示調整 SQL |
| INSERT column list 解析失敗（Upsert） | 低 | 省略 column list 時報錯 |
| Param name → table column mapping 失敗 | 中 | 無法 match 時 fallback 為 NOT NULL |
| BulkQuerier 與未來 sqlc 官方支援衝突 | 低 | 命名加 `Bulk` prefix 區隔 |
