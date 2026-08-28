---
title: "Game Terminology"
doc_type: glossary
last_reviewed: 2026-08-27
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
| [Action](#action) | - | 行動 | [REQ-003](../requirements/REQ-003.md)、[REQ-004](../requirements/REQ-004.md)、[REQ-005](../requirements/REQ-005.md)、[REQ-006](../requirements/REQ-006.md)、[REQ-008](../requirements/REQ-008.md)、[REQ-009](../requirements/REQ-009.md)、[REQ-010](../requirements/REQ-010.md) |
| [Action: rest](#action-rest) | - | 休息行動 | [REQ-004](../requirements/REQ-004.md) |
| [Action: move](#action-move) | - | 移動行動 | [REQ-005](../requirements/REQ-005.md) |
| [Action: gather](#action-gather) | - | 採集行動 | [REQ-006](../requirements/REQ-006.md) |
| [Action: convert](#action-convert) | - | 轉換行動 | [REQ-008](../requirements/REQ-008.md) |
| [Action: craft](#action-craft) | - | 製作行動 | [REQ-009](../requirements/REQ-009.md) |
| [Action: build](#action-build) | - | 開始施工 | [REQ-010](../requirements/REQ-010.md) |
| [Action: contribute construction](#action-contribute-construction) | - | 投入施工 AP | [REQ-010](../requirements/REQ-010.md) |
| [Action Points](#action-points-ap) | AP | 行動力 | [REQ-003](../requirements/REQ-003.md) |
| [Building](#building) | - | 建築 | [REQ-010](../requirements/REQ-010.md) |
| [Building recipe](#building-recipe) | - | 建築配方 | [REQ-010](../requirements/REQ-010.md) |
| [Construction progress](#construction-progress) | - | 施工進度 | [REQ-010](../requirements/REQ-010.md) |
| [Extension slot](#extension-slot) | - | 擴充建築欄位 | [REQ-010](../requirements/REQ-010.md) |
| [Inventory](#inventory) | - | 物品欄 | [REQ-006](../requirements/REQ-006.md) |
| [Item](#item) | - | 物品 | [REQ-006](../requirements/REQ-006.md)、[REQ-008](../requirements/REQ-008.md)、[REQ-009](../requirements/REQ-009.md)、[REQ-010](../requirements/REQ-010.md) |
| [Location](#location) | - | 位置 | [REQ-005](../requirements/REQ-005.md)、[REQ-010](../requirements/REQ-010.md) |
| [Route](#route) | - | 路徑 | [REQ-005](../requirements/REQ-005.md) |
| [Resource](#resource) | - | 資源 | [REQ-007](../requirements/REQ-007.md) |
| [Recipe](#recipe) | - | 配方 | [REQ-009](../requirements/REQ-009.md) |
| [Wood item](#wood-item) | - | 木材物品 | [REQ-006](../requirements/REQ-006.md)、[REQ-008](../requirements/REQ-008.md) |
| [Wood Resource](#wood-resource) | - | 木材資源 | [REQ-007](../requirements/REQ-007.md)、[REQ-008](../requirements/REQ-008.md) |
| [Wood Component](#wood-component) | - | 木質加工品 | [REQ-009](../requirements/REQ-009.md) |

## 名詞定義

### Action

- 正式英文名稱：Action
- 中文名稱：行動
- 定義：玩家要求後端執行的遊戲狀態變更。後端只執行明確允許的 Action。
- 對應行為：[REQ-003 - AP 計算](../requirements/REQ-003.md)、[REQ-004 - Action: rest](../requirements/REQ-004.md)、[REQ-005 - Action: move](../requirements/REQ-005.md)、[REQ-006 - Gathering and inventory](../requirements/REQ-006.md)、[REQ-008 - Action: convert](../requirements/REQ-008.md)、[REQ-009 - Action: craft](../requirements/REQ-009.md)、[REQ-010 - Building construction](../requirements/REQ-010.md)
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

### Action: convert

- 正式英文名稱：Action: convert
- 中文名稱：轉換行動
- 定義：玩家在 `camp` 消耗後端指定的 AP 與 Inventory item，取得 Resource 的 Action。
- 對應行為：[REQ-008 - Action: convert](../requirements/REQ-008.md)
- 與相似名詞的差異：`convert` 消耗 Item 並產出 Resource。`gather` 產出 Item。

### Action: craft

- 正式英文名稱：Action: craft
- 中文名稱：製作行動
- 定義：玩家消耗 recipe 定義的基本 AP、Resource inputs 與 Item inputs，取得明確 output Item 的 Action。
- 對應行為：[REQ-009 - Action: craft](../requirements/REQ-009.md)
- 與相似名詞的差異：`craft` 消耗 Resource 與可選 Item inputs，產出 Item。`convert` 消耗 Item 並產出 Resource。

### Action: build

- 正式英文名稱：Action: build
- 中文名稱：開始施工
- 定義：玩家消耗後端 Building recipe 的 inputs，在目前 Location 建立自己施工中 Building 的 Action。
- 對應行為：[REQ-010 - Building construction](../requirements/REQ-010.md)
- 與相似名詞的差異：`build` 建立 progress 0 的 Building。`contribute construction` 消耗 AP 增加既有 Building 的 progress。

### Action: contribute construction

- 正式英文名稱：Action: contribute construction
- 中文名稱：投入施工 AP
- 定義：玩家在 Building 所在 Location 消耗自己的 AP，增加該 Building 施工進度的 Action。
- 對應行為：[REQ-010 - Building construction](../requirements/REQ-010.md)
- 與相似名詞的差異：`contribute construction` 不消耗 recipe inputs，也不建立新 Building。

### Action Points (AP)

- 正式英文名稱：Action Points
- 縮寫：AP
- 中文名稱：行動力
- 定義：玩家角色執行行動時使用的資源，每分鐘恢復一點。
- 對應行為：[REQ-003 - AP 計算](../requirements/REQ-003.md)
- 與相似名詞的差異：AP 是目前可使用的行動力。`full_timestamp` 是後端計算 AP 的持久化資料，不是玩家持有的另一種資源。

### Building

- 正式英文名稱：Building
- 中文名稱：建築
- 定義：玩家在 Location 建立的持久化世界物件。它具有持有者、level、施工狀態、AP 需求與 extension slot 數量。
- 對應行為：[REQ-010 - Building construction](../requirements/REQ-010.md)
- 與相似名詞的差異：Building 是玩家建立的世界狀態。Building recipe 是建立規則。

### Building recipe

- 正式英文名稱：Building recipe
- 中文名稱：建築配方
- 定義：後端定義的 Building 建立規則。它包含 inputs、施工 AP 需求、Building level 與 extension slot 數量。
- 對應行為：[REQ-010 - Building construction](../requirements/REQ-010.md)
- 與相似名詞的差異：Building recipe 建立世界物件。Crafting Recipe 產出 Inventory Item。

### Construction progress

- 正式英文名稱：Construction progress
- 中文名稱：施工進度
- 定義：同一 Location 玩家投入 Building 的累計 AP，相對於建立時保存的 required AP。
- 對應行為：[REQ-010 - Building construction](../requirements/REQ-010.md)
- 與相似名詞的差異：Construction progress 屬於 Building。玩家 AP 屬於貢獻者。

### Extension slot

- 正式英文名稱：Extension slot
- 中文名稱：擴充建築欄位
- 定義：完成 Building 後可安裝 extension 的欄位。Building Lv1 具有一個空欄位。
- 對應行為：[REQ-010 - Building construction](../requirements/REQ-010.md)
- 與相似名詞的差異：Extension slot 是容量。Extension 是未來安裝的功能物件。

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
- 對應行為：[REQ-006 - Gathering and inventory](../requirements/REQ-006.md)、[REQ-008 - Action: convert](../requirements/REQ-008.md)、[REQ-009 - Action: craft](../requirements/REQ-009.md)、[REQ-010 - Building construction](../requirements/REQ-010.md)
- 與相似名詞的差異：Item 以 Inventory quantity 保存。Resource 依 Resource type 保存獨立 quantity。

### Location

- 正式英文名稱：Location
- 中文名稱：位置
- 定義：後端允許玩家停留的遊戲地點。
- 對應行為：[REQ-005 - Action: move](../requirements/REQ-005.md)、[REQ-010 - Building construction](../requirements/REQ-010.md)
- 與相似名詞的差異：Location 是地點。Route 描述兩個 Location 間的允許移動方向。

### Route

- 正式英文名稱：Route
- 中文名稱：路徑
- 定義：由 origin Location 指向 destination Location 的後端通行規則，包含一次移動的 AP 成本。
- 對應行為：[REQ-005 - Action: move](../requirements/REQ-005.md)
- 與相似名詞的差異：Route 是有向規則。Location 是玩家目前停留或可以到達的地點。

### Resource

- 正式英文名稱：Resource
- 中文名稱：資源
- 定義：玩家持有的持久化整數 quantity。每種 Resource type 分別保存，未持有時 quantity 為 0。
- 類型：Food、Wood、Stone、Metal、Fiber、Hide、Medicinal、Arcane。
- 對應行為：[REQ-007 - Typed resources](../requirements/REQ-007.md)
- 與相似名詞的差異：Resource 依 type 累積 quantity。Item 以種類與 quantity 保存在 Inventory。

### Recipe

- 正式英文名稱：Recipe
- 中文名稱：配方
- 定義：後端定義的 crafting 規則。它包含穩定 identifier、顯示名稱、基本 AP 成本、Resource inputs、Item inputs 與明確 output Item。
- 對應行為：[REQ-009 - Action: craft](../requirements/REQ-009.md)
- 與相似名詞的差異：Recipe 定義 `craft` 的成本與結果。Action 是玩家要求執行 recipe 的行為。

### Wood item

- 正式英文名稱：Wood item
- 中文名稱：木材物品
- 定義：玩家在 `forest_edge` 執行 `gather` 時取得的第一種 Item。
- 對應行為：[REQ-006 - Gathering and inventory](../requirements/REQ-006.md)、[REQ-008 - Action: convert](../requirements/REQ-008.md)
- 與相似名詞的差異：Wood item 存在 Inventory。`convert` 會消耗 Wood item 並增加 Wood Resource。

### Wood Resource

- 正式英文名稱：Wood Resource
- 中文名稱：木材資源
- 定義：`convert` 消耗 Wood item 後增加的 typed Resource quantity。
- 對應行為：[REQ-007 - Typed resources](../requirements/REQ-007.md)、[REQ-008 - Action: convert](../requirements/REQ-008.md)
- 與相似名詞的差異：Wood Resource 是 Resource quantity。Wood item 是 Inventory item。

### Wood Component

- 正式英文名稱：Wood Component
- 中文名稱：木質加工品
- 定義：第一種 crafting output Item。玩家消耗 10 Wood Resource 與 10 AP 製作 1 個 Wood Component。
- 對應行為：[REQ-009 - Action: craft](../requirements/REQ-009.md)
- 與相似名詞的差異：Wood Component 是 Inventory Item。Wood Resource 是 crafting input。
