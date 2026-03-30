# Spike 驗證結果

> 2026-03-30 執行，sqlc v1.29.0 + plugin-sdk-go v1.23.0

## A.1 自訂 command tag

```
$ sqlc generate
# package bulk
testdata/query.sql:22:1: invalid query type: :bulkupdate
```

**結論：** sqlc 不接受非標準的 command type，必須使用 `:exec` 搭配 comment annotation。

## A.2 Plugin 收到的 Query.Params 結構

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

`column.name` 為空（欄位不存在），但 type、is_array 仍完整。欄位名稱需從 `Query.Text` 中的 UNNEST alias 提取。

**`not_null` 陷阱：** 所有 UNNEST 參數的 `not_null` 均為 `true`，包括對應 nullable column 的參數（如 `category TEXT` 無 NOT NULL 約束）。這是因為 `not_null` 反映 UNNEST 表達式特性，而非 table column 約束。Nullable 判斷必須從 Catalog 取得。

## A.3 Catalog 中的 Table Schema

Plugin 收到完整的 table 定義，包含每個 column 的 name、type、not_null。User table 位於 `public` schema（`default_schema: "public"`）下，可與 `pg_catalog`（139 tables）、`information_schema`（69 tables）區隔。

## A.4 sqlc 生成的 Struct 實際範例

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

## A.5 Settings 結構

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

## A.6 UPDATE vs Upsert 的目標 Table 資訊

| Query 類型 | protobuf 中的 table 資訊 |
|---|---|
| Upsert | `insert_into_table: {"name": "products"}` |
| UPDATE | 無任何 table 欄位 — 需從 `Query.Text` 解析 |

## A.7 sqlc.yaml 設定對 Plugin 的影響

> 2026-03-30 驗證

測試了三種非預設設定對 model struct、params struct、plugin 可見性的影響：

| 設定 | Model struct 影響 | Params struct 影響 | Plugin 能否感知 |
|---|---|---|---|
| `emit_pointers_for_null_types: true` | `pgtype.Text` → `*string` | 不變（`[]string`） | 否 |
| `rename: {id: "ProductID"}` | `ID` → `ProductID` | 不變（`ColumnN`） | 否 |
| `overrides: [{column: "products.category", go_type: "string"}]` | `pgtype.Text` → `string` | 不變（`[]string`） | 否 |

**結論：**
- 這些設定只影響 sqlc 的 Go codegen 輸出（model struct），不影響 Params struct
- Plugin 的 `Settings`、`global_options`、`plugin_options` 在所有測試中均為空，完全無法感知 `gen.go` 下的設定
- Model struct 複用在非預設設定下可能產生 field name 或 type 不匹配
