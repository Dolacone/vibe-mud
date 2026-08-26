---
title: "Game Terminology"
doc_type: glossary
last_reviewed: 2026-08-26
source_paths: []
scope: "Canonical game terms used by the application."
---

# Game Terminology

## 更新規則

本文件是遊戲名詞的正式索引。新增、重新命名或重新定義遊戲名詞時，必須在同一 commit 更新本文件。

本文件只定義名詞。可檢查的遊戲行為由對應 REQ 定義，技術資料結構由 [SQLite Schemas](schemas.md) 定義。

## 名詞索引

| 英文名稱 | 縮寫 | 中文名稱 | 對應 REQ |
|---|---|---|---|
| [Action](#action) | - | 行動 | [REQ-003](../requirements/REQ-003.md)、[REQ-004](../requirements/REQ-004.md)、[REQ-005](../requirements/REQ-005.md)、[REQ-006](../requirements/REQ-006.md) |
| [Action: rest](#action-rest) | - | 休息行動 | [REQ-004](../requirements/REQ-004.md) |
| [Action: move](#action-move) | - | 移動行動 | [REQ-005](../requirements/REQ-005.md) |
| [Action: gather](#action-gather) | - | 採集行動 | [REQ-006](../requirements/REQ-006.md) |
| [Action Points](#action-points-ap) | AP | 行動力 | [REQ-003](../requirements/REQ-003.md) |
| [Inventory](#inventory) | - | 物品欄 | [REQ-006](../requirements/REQ-006.md) |
| [Item](#item) | - | 物品 | [REQ-006](../requirements/REQ-006.md) |
| [Location](#location) | - | 位置 | [REQ-005](../requirements/REQ-005.md) |
| [Route](#route) | - | 路徑 | [REQ-005](../requirements/REQ-005.md) |
| [Wood](#wood) | - | 木材物品 | [REQ-006](../requirements/REQ-006.md) |

## 名詞定義

### Action

- 正式英文名稱：Action
- 中文名稱：行動
- 定義：玩家要求後端執行的遊戲狀態變更。後端只執行明確允許的 Action。
- 對應行為：[REQ-003 - AP 計算](../requirements/REQ-003.md)、[REQ-004 - Action: rest](../requirements/REQ-004.md)、[REQ-005 - Action: move](../requirements/REQ-005.md)、[REQ-006 - Gathering and inventory](../requirements/REQ-006.md)
- 與相似名詞的差異：Action 是行為種類。AP 是執行 Action 可以消耗的資源。

### Action: rest

- 正式英文名稱：Action: rest
- 中文名稱：休息行動
- 定義：消耗 1 AP，但不改變玩家位置的 Action。
- 對應行為：[REQ-004 - Action: rest](../requirements/REQ-004.md)
- 與相似名詞的差異：`rest` 使用固定成本。`move` 的成本由後端 Route 決定。

### Action: move

- 正式英文名稱：Action: move
- 中文名稱：移動行動
- 定義：玩家沿後端允許的 Route 前往 target Location，並消耗該 Route 的 AP 成本。
- 對應行為：[REQ-005 - Action: move](../requirements/REQ-005.md)
- 與相似名詞的差異：`move` 是 Action。Route 是允許這個 Action 的有向通行規則。

### Action: gather

- 正式英文名稱：Action: gather
- 中文名稱：採集行動
- 定義：玩家在後端允許的 Location 消耗 AP，並將後端決定的 item quantity 加入 Inventory。
- 對應行為：[REQ-006 - Gathering and inventory](../requirements/REQ-006.md)
- 與相似名詞的差異：`gather` 是取得 item 的 Action。Inventory 是保存取得結果的玩家狀態。

### Action Points (AP)

- 正式英文名稱：Action Points
- 縮寫：AP
- 中文名稱：行動力
- 定義：玩家角色執行行動時使用的資源，每分鐘恢復一點。
- 對應行為：[REQ-003 - AP 計算](../requirements/REQ-003.md)
- 與相似名詞的差異：AP 是目前可使用的行動力。`full_timestamp` 是後端計算 AP 的持久化資料，不是玩家持有的另一種資源。

### Inventory

- 正式英文名稱：Inventory
- 中文名稱：物品欄
- 定義：保存玩家目前持有 item 與 quantity 的持久化玩家狀態。
- 對應行為：[REQ-006 - Gathering and inventory](../requirements/REQ-006.md)
- 與相似名詞的差異：Inventory 保存持有狀態。Item 定義可持有的物品種類。

### Item

- 正式英文名稱：Item
- 中文名稱：物品
- 定義：玩家可以取得並保存在 Inventory 的離散物品種類。
- 對應行為：[REQ-006 - Gathering and inventory](../requirements/REQ-006.md)
- 與相似名詞的差異：Item 以 quantity 保存。未來的 Resource conversion 不屬於本次行為。

### Location

- 正式英文名稱：Location
- 中文名稱：位置
- 定義：後端允許玩家停留的遊戲地點。
- 對應行為：[REQ-005 - Action: move](../requirements/REQ-005.md)
- 與相似名詞的差異：Location 是地點。Route 描述兩個 Location 間的允許移動方向。

### Route

- 正式英文名稱：Route
- 中文名稱：路徑
- 定義：由 origin Location 指向 destination Location 的後端通行規則，包含一次移動的 AP 成本。
- 對應行為：[REQ-005 - Action: move](../requirements/REQ-005.md)
- 與相似名詞的差異：Route 是有向規則。Location 是玩家目前停留或可以到達的地點。

### Wood

- 正式英文名稱：Wood
- 中文名稱：木材物品
- 定義：玩家在 `forest_edge` 執行 `gather` 時取得的第一種 Item。
- 對應行為：[REQ-006 - Gathering and inventory](../requirements/REQ-006.md)
- 與相似名詞的差異：Wood 是 Item。它尚未轉換為 Resource，也不是官方 currency。
