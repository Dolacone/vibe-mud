---
title: "Game Terminology"
doc_type: glossary
last_reviewed: 2026-08-28
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
| [Action](#action) | - | 行動 | [REQ-003](../requirements/REQ-003.md)、[REQ-004](../requirements/REQ-004.md)、[REQ-005](../requirements/REQ-005.md)、[REQ-006](../requirements/REQ-006.md)、[REQ-008](../requirements/REQ-008.md)、[REQ-009](../requirements/REQ-009.md)、[REQ-010](../requirements/REQ-010.md)、[REQ-011](../requirements/REQ-011.md) |
| [Action: rest](#action-rest) | - | 休息行動 | [REQ-004](../requirements/REQ-004.md) |
| [Action: move](#action-move) | - | 移動行動 | [REQ-005](../requirements/REQ-005.md) |
| [Action: gather](#action-gather) | - | 採集行動 | [REQ-006](../requirements/REQ-006.md) |
| [Action: convert](#action-convert) | - | 轉換行動 | [REQ-008](../requirements/REQ-008.md) |
| [Action: craft](#action-craft) | - | 製作行動 | [REQ-009](../requirements/REQ-009.md) |
| [Action: build](#action-build) | - | 開始施工 | [REQ-010](../requirements/REQ-010.md) |
| [Action: contribute construction](#action-contribute-construction) | - | 投入施工 AP | [REQ-010](../requirements/REQ-010.md) |
| [Action: repair Building](#action-repair-building) | - | 維修建築 | [REQ-011](../requirements/REQ-011.md) |
| [Action Points](#action-points-ap) | AP | 行動力 | [REQ-003](../requirements/REQ-003.md) |
| [Building](#building) | - | 建築 | [REQ-010](../requirements/REQ-010.md) |
| [Building recipe](#building-recipe) | - | 建築配方 | [REQ-010](../requirements/REQ-010.md) |
| [Building durability](#building-durability) | - | 建築耐久度 | [REQ-011](../requirements/REQ-011.md) |
| [Carrying weight](#carrying-weight) | - | 攜帶重量 | [REQ-014](../requirements/REQ-014.md) |
| [Construction progress](#construction-progress) | - | 施工進度 | [REQ-010](../requirements/REQ-010.md) |
| [Extension slot](#extension-slot) | - | 擴充建築欄位 | [REQ-010](../requirements/REQ-010.md) |
| [Ground assets](#ground-assets) | - | 地面資產 | [REQ-013](../requirements/REQ-013.md) |
| [Inventory](#inventory) | - | 物品欄 | [REQ-006](../requirements/REQ-006.md) |
| [Item](#item) | - | 物品 | [REQ-006](../requirements/REQ-006.md)、[REQ-008](../requirements/REQ-008.md)、[REQ-009](../requirements/REQ-009.md)、[REQ-010](../requirements/REQ-010.md)、[REQ-013](../requirements/REQ-013.md)、[REQ-014](../requirements/REQ-014.md) |
| [Location](#location) | - | 位置 | [REQ-005](../requirements/REQ-005.md)、[REQ-010](../requirements/REQ-010.md)、[REQ-013](../requirements/REQ-013.md) |
| [Route](#route) | - | 路徑 | [REQ-005](../requirements/REQ-005.md) |
| [Movement weight threshold](#movement-weight-threshold) | - | 移動負重門檻 | [REQ-014](../requirements/REQ-014.md) |
| [Resource](#resource) | - | 資源 | [REQ-007](../requirements/REQ-007.md)、[REQ-013](../requirements/REQ-013.md)、[REQ-014](../requirements/REQ-014.md) |
| [Recipe](#recipe) | - | 配方 | [REQ-009](../requirements/REQ-009.md) |
| [Transfer](#transfer) | - | 轉移 | [REQ-013](../requirements/REQ-013.md) |
| [Transfer: Drop](#transfer-drop) | - | 放置 | [REQ-013](../requirements/REQ-013.md) |
| [Transfer: Pickup](#transfer-pickup) | - | 撿取 | [REQ-013](../requirements/REQ-013.md) |
| [Wood item](#wood-item) | - | 木材物品 | [REQ-006](../requirements/REQ-006.md)、[REQ-008](../requirements/REQ-008.md) |
| [Wood Resource](#wood-resource) | - | 木材資源 | [REQ-007](../requirements/REQ-007.md)、[REQ-008](../requirements/REQ-008.md) |
| [Wood Component](#wood-component) | - | 木質加工品 | [REQ-009](../requirements/REQ-009.md) |

## 名詞定義

### Action

- 正式英文名稱：Action
- 中文名稱：行動
- 定義：玩家消耗 AP，要求後端執行的世界行為。後端只執行明確允許的 Action。
- 對應行為：[REQ-003 - AP 計算](../requirements/REQ-003.md)、[REQ-004 - Action: rest](../requirements/REQ-004.md)、[REQ-005 - Action: move](../requirements/REQ-005.md)、[REQ-006 - Gathering and inventory](../requirements/REQ-006.md)、[REQ-008 - Action: convert](../requirements/REQ-008.md)、[REQ-009 - Action: craft](../requirements/REQ-009.md)、[REQ-010 - Building construction](../requirements/REQ-010.md)、[REQ-011 - Building durability and repair](../requirements/REQ-011.md)
- 與相似名詞的差異：Action 消耗 AP。Transfer 搬移既有資產，不消耗 AP。

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

### Action: repair Building

- 正式英文名稱：Action: repair Building
- 中文名稱：維修建築
- 定義：同一 Location 的玩家消耗 10 AP 與 1 Wood Resource，替完成的 Building 增加最多一小時耐久時間。
- 對應行為：[REQ-011 - Building durability and repair](../requirements/REQ-011.md)
- 與相似名詞的差異：`repair Building` 增加完成 Building 的耐久時間。`contribute construction` 增加施工中 Building 的進度。

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

### Building durability

- 正式英文名稱：Building durability
- 中文名稱：建築耐久度
- 定義：完成 Building 依耐久期限與後端目前時間計算的剩餘可用時間。大於 0 時為 Active。到期後為 Disabled。Disabled 超過三天後永久消失。
- 對應行為：[REQ-011 - Building durability and repair](../requirements/REQ-011.md)
- 與相似名詞的差異：Building durability 隨現實時間降低。Construction progress 只隨玩家投入 AP 增加。

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

### Carrying weight

- 正式英文名稱：Carrying weight
- 中文名稱：攜帶重量
- 定義：玩家持有的每種 Item 與 Resource quantity 乘以對應單位重量後的總和。系統讀取玩家狀態時即時計算，不保存總和。
- 對應行為：[REQ-014 - 玩家攜帶重量](../requirements/REQ-014.md)
- 與相似名詞的差異：Carrying weight 是目前計算值。Movement weight threshold 是判斷能否移動的固定門檻。

### Ground assets

- 正式英文名稱：Ground assets
- 中文名稱：地面資產
- 定義：位於單一 Location、沒有 owner 或權限、所有同地點玩家都能 Pickup 的公共 Item 與 Resource quantity。
- 對應行為：[REQ-013 - Ground asset transfers](../requirements/REQ-013.md)
- 與相似名詞的差異：Ground assets 屬於 Location 的公共狀態。Inventory 與玩家 Resource 屬於單一玩家。

### Item

- 正式英文名稱：Item
- 中文名稱：物品
- 定義：玩家可以取得並保存在 Inventory，或放在 Location 地面的離散物品種類。每種 Item 定義正整數單位重量。
- 對應行為：[REQ-006 - Gathering and inventory](../requirements/REQ-006.md)、[REQ-008 - Action: convert](../requirements/REQ-008.md)、[REQ-009 - Action: craft](../requirements/REQ-009.md)、[REQ-010 - Building construction](../requirements/REQ-010.md)、[REQ-013 - Ground asset transfers](../requirements/REQ-013.md)、[REQ-014 - 玩家攜帶重量](../requirements/REQ-014.md)
- 與相似名詞的差異：Item 以 Inventory 或地面 quantity 保存。Resource 依 Resource type 保存，不轉換成 Item。

### Location

- 正式英文名稱：Location
- 中文名稱：位置
- 定義：後端允許玩家停留、建立 Building，並累積公共地面資產的遊戲地點。
- 對應行為：[REQ-005 - Action: move](../requirements/REQ-005.md)、[REQ-010 - Building construction](../requirements/REQ-010.md)、[REQ-013 - Ground asset transfers](../requirements/REQ-013.md)
- 與相似名詞的差異：Location 保存地點範圍與公共世界狀態。Route 描述兩個 Location 間的允許移動方向。

### Route

- 正式英文名稱：Route
- 中文名稱：路徑
- 定義：由 origin Location 指向 destination Location 的後端通行規則，包含一次移動的 AP 成本。
- 對應行為：[REQ-005 - Action: move](../requirements/REQ-005.md)
- 與相似名詞的差異：Route 是有向規則。Location 是玩家目前停留或可以到達的地點。

### Movement weight threshold

- 正式英文名稱：Movement weight threshold
- 中文名稱：移動負重門檻
- 定義：判斷玩家能否 Move 的固定攜帶重量門檻。MVP 為 1000 重量單位。
- 對應行為：[REQ-014 - 玩家攜帶重量](../requirements/REQ-014.md)
- 與相似名詞的差異：Movement weight threshold 不限制玩家持有資產。Carrying weight 超過門檻時只禁止 Move。

### Resource

- 正式英文名稱：Resource
- 中文名稱：資源
- 定義：玩家或 Location 地面持有的持久化整數 quantity。每種 Resource type 分別保存，並定義正整數單位重量。
- 類型：Food、Wood、Stone、Metal、Fiber、Hide、Medicinal、Arcane。
- 對應行為：[REQ-007 - Typed resources](../requirements/REQ-007.md)、[REQ-013 - Ground asset transfers](../requirements/REQ-013.md)、[REQ-014 - 玩家攜帶重量](../requirements/REQ-014.md)
- 與相似名詞的差異：Resource 依 type 累積 quantity。它可以在玩家與地面之間 Transfer，但不轉換成 Item。

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

### Transfer

- 正式英文名稱：Transfer
- 中文名稱：轉移
- 定義：在玩家持有狀態與目前 Location 地面之間搬移既有 quantity，且不消耗 AP 的操作。
- 對應行為：[REQ-013 - Ground asset transfers](../requirements/REQ-013.md)
- 與相似名詞的差異：Transfer 保持世界資產總量不變且不消耗 AP。Action 執行遊戲行為並消耗 AP。

### Transfer: Drop

- 正式英文名稱：Transfer: Drop
- 中文名稱：放置
- 定義：把玩家持有的 Item 或 Resource quantity 轉移至目前 Location 的公共地面。
- 對應行為：[REQ-013 - Ground asset transfers](../requirements/REQ-013.md)
- 與相似名詞的差異：Drop 的來源是玩家持有狀態。Pickup 的來源是地面資產。

### Transfer: Pickup

- 正式英文名稱：Transfer: Pickup
- 中文名稱：撿取
- 定義：把目前 Location 的公共地面 Item 或 Resource quantity 轉移給玩家。
- 對應行為：[REQ-013 - Ground asset transfers](../requirements/REQ-013.md)
- 與相似名詞的差異：Pickup 的來源是地面資產。Drop 的來源是玩家持有狀態。
