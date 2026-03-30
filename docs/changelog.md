# SPEC 變更紀錄

- **v0.1 → v0.2**：根據 spike 驗證結果調整標記機制與 struct mapping 策略
- **v0.2 → v0.3**：確立 input struct 複用規則——全欄位時複用 sqlc model struct，部分欄位時生成獨立 Item struct，不跨 query 共用（與 sqlc 原生行為一致）
- **v0.3 → v0.4**：SQL 範例統一使用標準 `$N` positional parameter 語法；新增 FR-7 UNNEST alias 解析機制，由 plugin 從 SQL 文字中提取欄位名稱
- **v0.4 → v0.5**：修正 nullable 判斷來源——params 的 `not_null` 永遠為 `true`（反映 UNNEST 表達式特性），nullable 必須從 catalog 比對取得
- **v0.5 → v0.6**：新增 FR-8 目標 table 解析；修正 adapter code 範例中的 params field 命名（`$N` 語法對應 `ColumnN`）；新增 Params field 引用規則；附錄補充 sqlc 實際生成的 struct 與 Settings 結構
- **v0.6 → v0.7**：確立僅支援 sqlc 預設設定；`rename`、`overrides`、`emit_pointers_for_null_types` 等非預設設定列入已知限制
- **v0.7 → v0.8**：Phase 1 實作完成；修正 FR-7——sqlc 會將 `@param` 轉為 `$N` in Query.Text，UNNEST alias 解析永遠需要執行
- **v0.8 → v0.9**：重新設計 FR-6——新增 `style` 參數，支援三種生成模式（`function`/`method`/`interface`），解決 `Querier` interface 整合問題
- **v0.9 → v1.0**：新增 FR-3 Bulk Upsert 實作規格——根據實際專案 SQL 調查確立 `VALUES (UNNEST(...))` 為主要支援格式；新增 FR-9 INSERT column list 解析；`SELECT * FROM UNNEST(...)` 格式列為已知限制；確認 `NULLIF(UNNEST(...))` 等函式包裝在 VALUES 格式中可正常運作
