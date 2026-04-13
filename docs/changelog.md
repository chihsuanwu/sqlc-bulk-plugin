# Changelog

## v0.4.0 (unreleased)

根據實際專案導入經驗調整行為：

- 生成的 `XxxBatch` 加入 empty-input guard —— `if len(items) == 0` 直接 early return（`:exec` 回 `nil`，`:many` 回 `nil, nil`），避免呼叫端沒做防禦時產生無意義的 DB round trip
- 新增限制：單欄 bulk insert/upsert 被 plugin 拒絕並回傳 graceful error，因為生成的 adapter 會是對 sqlc method 的 thin pass-through，加 `@bulk` 沒有任何好處。UPDATE 不受此限制（結構上至少需要 id + value 兩欄）
- 鎖定既有行為：`:many` 多欄 RETURNING 的 graceful error 訊息、`INSERT ... SELECT UNNEST` 形式的 unit + e2e 雙層覆蓋，都透過強化斷言和 cross-reference 註解鎖定，防止未來 regress
- 新增 Querier doc-comment 汙染為 SPEC 已知限制

## v0.3.4

- fix: 改為從 `Catalog.Schema.Enums` 正向識別 custom enum type，取代原本「不在 pgTypeMap 裡就當 enum」的負向邏輯

## v0.3.3

- fix: Item struct 欄位一律使用 base type（與 sqlc params element type 一致），不再受 catalog nullability 影響

## v0.3.2

- fix: 生成的 doc comment 中 method name 加上 `Querier.` / `Queries.` 前綴，改善 IDE 導覽

## v0.3.1

- feat: 新增 `--version` flag（使用 `runtime/debug` build info）
- refactor: 整合 type map，移除 dead state，抽出 `resolveGoType`
- feat: 為生成的 adapter function 加入 doc comment

## v0.3.0

- feat: 新增 `emit_interface` plugin option，控制 `function` style 使用 `Querier`（true）或 `*Queries`（false）

## v0.2.1

- fix: 強化 regex 解析、type 覆蓋率、`singularize` edge case 處理
- fix: 改用 type name 而非 schema 來偵測 custom type
- test: 新增 e2e test（實際執行 `sqlc generate` 驗證編譯）
- docs: 更新 supported types 表格與 design decisions

## v0.2.0

- feat: 支援 custom PostgreSQL enum type
- feat: 簡化 annotation 為 `@bulk`，自動偵測 UPDATE / INSERT / Upsert
- feat: 支援 `:many` + 單欄位 `RETURNING`
- feat: 實作 bulk upsert（`VALUES (UNNEST(...))` 格式）
- feat: 新增 `style` 參數，支援三種生成模式（`function`/`method`/`interface`）
- ci: 新增 GitHub Actions workflow + GoReleaser

## v0.1.0

- 初始發布：Phase 1 bulk UPDATE adapter 生成
