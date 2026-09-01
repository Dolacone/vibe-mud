---
title: "Game Terminology"
doc_type: glossary
last_reviewed: 2026-09-01
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
| [Action](#action) | - | 行動 | [REQ-003](../requirements/REQ-003.md)、[REQ-004](../requirements/REQ-004.md)、[REQ-005](../requirements/REQ-005.md)、[REQ-006](../requirements/REQ-006.md)、[REQ-008](../requirements/REQ-008.md)、[REQ-009](../requirements/REQ-009.md)、[REQ-010](../requirements/REQ-010.md)、[REQ-011](../requirements/REQ-011.md)、[REQ-026](../requirements/REQ-026.md) |
| [Action: rest](#action-rest) | - | 休息行動 | [REQ-004](../requirements/REQ-004.md) |
| [Action: move](#action-move) | - | 移動行動 | [REQ-005](../requirements/REQ-005.md) |
| [Action: gather](#action-gather) | - | 採集行動 | [REQ-006](../requirements/REQ-006.md) |
| [Action: convert](#action-convert) | - | 轉換行動 | [REQ-008](../requirements/REQ-008.md) |
| [Action: craft](#action-craft) | - | 製作行動 | [REQ-009](../requirements/REQ-009.md) |
| [Action: build](#action-build) | - | 開始施工 | [REQ-010](../requirements/REQ-010.md) |
| [Action: contribute construction](#action-contribute-construction) | - | 投入施工 AP | [REQ-010](../requirements/REQ-010.md) |
| [Action: repair Building](#action-repair-building) | - | 維修建築 | [REQ-011](../requirements/REQ-011.md) |
| [Action: attack](#action-attack) | - | 主動攻擊 | [REQ-026](../requirements/REQ-026.md) |
| [Action Points](#action-points-ap) | AP | 行動力 | [REQ-003](../requirements/REQ-003.md) |
| [Active Item](#active-item) | - | 有效物品 | [REQ-015](../requirements/REQ-015.md) |
| [Building](#building) | - | 建築 | [REQ-010](../requirements/REQ-010.md) |
| [Building recipe](#building-recipe) | - | 建築配方 | [REQ-010](../requirements/REQ-010.md) |
| [Building durability](#building-durability) | - | 建築耐久度 | [REQ-011](../requirements/REQ-011.md) |
| [Building extension](#building-extension) | - | 建築擴充 | [REQ-016](../requirements/REQ-016.md) |
| [Carrying weight](#carrying-weight) | - | 攜帶重量 | [REQ-014](../requirements/REQ-014.md) |
| [Construction progress](#construction-progress) | - | 施工進度 | [REQ-010](../requirements/REQ-010.md) |
| [Combat](#combat) | - | 戰鬥 | [REQ-026](../requirements/REQ-026.md) |
| [Convert method](#convert-method) | - | 轉換方式 | [REQ-008](../requirements/REQ-008.md) |
| [Essence](#essence) | - | 精華 | [REQ-008](../requirements/REQ-008.md) |
| [Extension slot](#extension-slot) | - | 擴充建築欄位 | [REQ-016](../requirements/REQ-016.md) |
| [Expired Item](#expired-item) | - | 失效物品 | [REQ-015](../requirements/REQ-015.md) |
| [Ground assets](#ground-assets) | - | 地面資產 | [REQ-013](../requirements/REQ-013.md) |
| [Hit Points](#hit-points-hp) | HP | 生命值 | [REQ-026](../requirements/REQ-026.md) |
| [Inventory](#inventory) | - | 物品欄 | [REQ-006](../requirements/REQ-006.md)、[REQ-015](../requirements/REQ-015.md) |
| [Item](#item) | - | 物品 | [REQ-006](../requirements/REQ-006.md)、[REQ-008](../requirements/REQ-008.md)、[REQ-009](../requirements/REQ-009.md)、[REQ-010](../requirements/REQ-010.md)、[REQ-013](../requirements/REQ-013.md)、[REQ-014](../requirements/REQ-014.md)、[REQ-015](../requirements/REQ-015.md) |
| [Item durability](#item-durability) | - | 物品耐久度 | [REQ-015](../requirements/REQ-015.md) |
| [Location](#location) | - | 位置 | [REQ-005](../requirements/REQ-005.md)、[REQ-010](../requirements/REQ-010.md)、[REQ-013](../requirements/REQ-013.md)、[REQ-025](../requirements/REQ-025.md) |
| [Location Monster population](#location-monster-population) | - | 地區怪物數量 | [REQ-025](../requirements/REQ-025.md) |
| [Monster](#monster) | - | 怪物 | [REQ-025](../requirements/REQ-025.md)、[REQ-026](../requirements/REQ-026.md) |
| [Monster type](#monster-type) | - | 怪物種類 | [REQ-025](../requirements/REQ-025.md)、[REQ-026](../requirements/REQ-026.md) |
| [Route](#route) | - | 路徑 | [REQ-005](../requirements/REQ-005.md) |
| [Movement weight threshold](#movement-weight-threshold) | - | 移動負重門檻 | [REQ-014](../requirements/REQ-014.md) |
| [Package Item](#package-item) | - | 擴充套件物品 | [REQ-016](../requirements/REQ-016.md) |
| [Player name](#player-name) | - | 玩家名稱 | [REQ-020](../requirements/REQ-020.md) |
| [Resource](#resource) | - | 資源 | [REQ-007](../requirements/REQ-007.md)、[REQ-013](../requirements/REQ-013.md)、[REQ-014](../requirements/REQ-014.md) |
| [Recipe](#recipe) | - | 配方 | [REQ-009](../requirements/REQ-009.md) |
| [Sawmill T1](#sawmill-t1) | - | 鋸木廠 T1 | [REQ-017](../requirements/REQ-017.md) |
| [Sawmill Package T1](#sawmill-package-t1) | - | 鋸木廠套件 T1 | [REQ-017](../requirements/REQ-017.md) |
| [Transfer](#transfer) | - | 轉移 | [REQ-013](../requirements/REQ-013.md) |
| [Transfer: Drop](#transfer-drop) | - | 放置 | [REQ-013](../requirements/REQ-013.md) |
| [Transfer: Pickup](#transfer-pickup) | - | 撿取 | [REQ-013](../requirements/REQ-013.md) |
| [Wood item](#wood-item) | - | 木材物品 | [REQ-006](../requirements/REQ-006.md)、[REQ-008](../requirements/REQ-008.md) |
| [Wood Resource](#wood-resource) | - | 木材資源 | [REQ-007](../requirements/REQ-007.md)、[REQ-008](../requirements/REQ-008.md) |
| [Wood Component](#wood-component) | - | 木質加工品 | [REQ-009](../requirements/REQ-009.md) |

## 名詞定義

### Action: attack

- 正式英文名稱：Action: attack
- 中文名稱：主動攻擊
- 定義：玩家消耗 30 AP，要求後端攻擊目前 Location 的一隻 Monster。
- 對應行為：[REQ-026 - Monster combat](../requirements/REQ-026.md)
- 與相似名詞的差異：主動 attack 消耗 AP。Move 攔截引發的 Combat 不消耗 AP。

### Combat

- 正式英文名稱：Combat
- 中文名稱：戰鬥
- 定義：後端在單一 request 內自動結算玩家與一種 Monster type 的交互攻擊。
- 對應行為：[REQ-026 - Monster combat](../requirements/REQ-026.md)
- 與相似名詞的差異：Combat 是結算流程。Action: attack 與 Move 攔截是兩種觸發方式。

### Hit Points (HP)

- 正式英文名稱：Hit Points
- 中文名稱：生命值
- 定義：玩家或 Monster 在 Combat 中可承受的傷害總量。
- 對應行為：[REQ-026 - Monster combat](../requirements/REQ-026.md)
- 與相似名詞的差異：HP 決定戰鬥存活。AP 決定玩家能否主動執行 Action。

### Location Monster population

- 正式英文名稱：Location Monster population
- 中文名稱：地區怪物數量
- 定義：Location 保存的未抽選 type Monster 總數。
- 對應行為：[REQ-025 - Location Monster population](../requirements/REQ-025.md)
- 與相似名詞的差異：Population 只保存數量。Monster type 在 Combat 開始時才抽選。

### Monster

- 正式英文名稱：Monster
- 中文名稱：怪物
- 定義：由 Location population 提供並在 Combat 中與玩家交戰的敵對單位。
- 對應行為：[REQ-025 - Location Monster population](../requirements/REQ-025.md)、[REQ-026 - Monster combat](../requirements/REQ-026.md)
- 與相似名詞的差異：Monster 是可被消耗的數量。Monster type 是戰鬥定義。

### Monster type

- 正式英文名稱：Monster type
- 中文名稱：怪物種類
- 定義：定義 Monster 的顯示名稱、HP、攻擊力與掉落規則。
- 對應行為：[REQ-025 - Location Monster population](../requirements/REQ-025.md)、[REQ-026 - Monster combat](../requirements/REQ-026.md)
- 與相似名詞的差異：Monster type 不在生成時保存。Combat 開始時依 Location encounter weight 抽選。

### Sawmill Package T1

- 正式英文名稱：Sawmill Package T1
- 中文名稱：鋸木廠套件 T1
- 定義：由配方產出的 Active Package Item，用於安裝 Sawmill T1。
- 對應行為：[REQ-017 - Sawmill T1](../requirements/REQ-017.md)
- 與相似名詞的差異：Package Item 是可安裝的物品。Sawmill T1 是安裝後提供 Convert 功能的 Building extension。

### Action

- 正式英文名稱：Action
- 中文名稱：行動
- 定義：玩家要求後端執行的世界行為。成功 Action 依各自規則消耗 AP。
- 對應行為：[REQ-003 - AP 計算](../requirements/REQ-003.md)、[REQ-004 - Action: rest](../requirements/REQ-004.md)、[REQ-005 - Action: move](../requirements/REQ-005.md)、[REQ-006 - Gathering and inventory](../requirements/REQ-006.md)、[REQ-008 - Action: convert](../requirements/REQ-008.md)、[REQ-009 - Action: craft](../requirements/REQ-009.md)、[REQ-010 - Building construction](../requirements/REQ-010.md)、[REQ-011 - Building durability and repair](../requirements/REQ-011.md)、[REQ-026 - Monster combat](../requirements/REQ-026.md)
- 與相似名詞的差異：Action 依自身規則決定 AP 成本。Transfer 搬移既有資產，不消耗 AP。

### Action: rest

- 正式英文名稱：Action: rest
- 中文名稱：休息行動
- 定義：消耗 1 AP，但不改變玩家位置的 Action。
- 對應行為：[REQ-004 - Action: rest](../requirements/REQ-004.md)
- 與相似名詞的差異：`rest` 使用固定成本。`move` 的成本由後端 Route 決定。

### Action: move

- 正式英文名稱：Action: move
- 中文名稱：移動行動
- 定義：玩家沿後端允許的 Route 前往 target Location。未被 Monster 攔截時消耗 Route AP。
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
- 定義：玩家在任何 Location 選擇後端提供的 Convert method 與 quantity，消耗一個 AP 工作單位及 Active Item，取得 Resource 並判定 Essence 的 Action。
- 對應行為：[REQ-008 - Action: convert](../requirements/REQ-008.md)
- 與相似名詞的差異：`convert` 是玩家執行的 Action。Convert method 定義成本、capacity 與產出。

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
- 定義：完成 Building 依耐久期限與後端目前時間計算的剩餘可用時間。大於 0 時為 Active。到期後為 Disabled。Disabled 超過七天後永久消失。
- 對應行為：[REQ-011 - Building durability and repair](../requirements/REQ-011.md)
- 與相似名詞的差異：Building durability 隨現實時間降低。Construction progress 只隨玩家投入 AP 增加。

### Building extension

- 正式英文名稱：Building extension
- 中文名稱：建築擴充
- 定義：安裝在 Building extension slot 的持久化功能物件。它具有 definition、tier、施工 AP 快照、progress 與施工狀態。
- 對應行為：[REQ-016 - Building extension lifecycle](../requirements/REQ-016.md)
- 與相似名詞的差異：Building extension 是已安裝狀態。Package Item 是安裝時消耗的 Inventory Item。

### Construction progress

- 正式英文名稱：Construction progress
- 中文名稱：施工進度
- 定義：同一 Location 玩家投入 Building 的累計 AP，相對於建立時保存的 required AP。
- 對應行為：[REQ-010 - Building construction](../requirements/REQ-010.md)
- 與相似名詞的差異：Construction progress 屬於 Building。玩家 AP 屬於貢獻者。

### Convert method

- 正式英文名稱：Convert method
- 中文名稱：轉換方式
- 定義：後端定義的一個 Convert 工作單位。它包含 AP 成本、input Item、capacity、Resource 產量與 Essence 規則。
- 對應行為：[REQ-008 - Action: convert](../requirements/REQ-008.md)
- 與相似名詞的差異：Convert method 是可調整的規則。Action: convert 是玩家使用規則的操作。

### Essence

- 正式英文名稱：Essence
- 中文名稱：精華
- 定義：Convert 每個 input Item 時依 method 機率產出的 Item。Wood Essence T1 是 Sawmill Package T1 的 Item input。
- 對應行為：[REQ-008 - Action: convert](../requirements/REQ-008.md)、[REQ-017 - Sawmill T1](../requirements/REQ-017.md)
- 與相似名詞的差異：Essence 是具有耐久與重量的 Item。Resource 是不會失效的累積 quantity。

### Extension slot

- 正式英文名稱：Extension slot
- 中文名稱：擴充建築欄位
- 定義：Completed Building 依建立時保存的 slot 數量提供的 extension 安裝位置。每個 slot 最多保存一個 extension。
- 對應行為：[REQ-016 - Building extension lifecycle](../requirements/REQ-016.md)
- 與相似名詞的差異：Extension slot 是容量。Building extension 是 slot 內的持久化物件。

### Inventory

- 正式英文名稱：Inventory
- 中文名稱：物品欄
- 定義：保存玩家目前持有 Item 堆疊、quantity、耐久狀態與狀態期限的持久化玩家狀態。
- 對應行為：[REQ-006 - Gathering and inventory](../requirements/REQ-006.md)、[REQ-015 - Item durability](../requirements/REQ-015.md)
- 與相似名詞的差異：Inventory 保存持有狀態。Item 定義可持有的物品種類與耐久上限。

### Carrying weight

- 正式英文名稱：Carrying weight
- 中文名稱：攜帶重量
- 定義：玩家持有的 Active Item、尚在保留期內的 Expired Item 與 Resource quantity 乘以對應單位重量後的總和。系統讀取玩家狀態時即時計算，不保存總和。
- 對應行為：[REQ-014 - 玩家攜帶重量](../requirements/REQ-014.md)、[REQ-015 - Item durability](../requirements/REQ-015.md)
- 與相似名詞的差異：Carrying weight 是目前計算值。Movement weight threshold 是判斷能否移動的固定門檻。

### Ground assets

- 正式英文名稱：Ground assets
- 中文名稱：地面資產
- 定義：位於單一 Location、沒有 owner 或權限的公共 Item 與 Resource quantity。Active Item 與 Resource 可以 Pickup，Expired Item 只能查看。
- 對應行為：[REQ-013 - Ground asset transfers](../requirements/REQ-013.md)、[REQ-015 - Item durability](../requirements/REQ-015.md)
- 與相似名詞的差異：Ground assets 屬於 Location 的公共狀態。Inventory 與玩家 Resource 屬於單一玩家。

### Item

- 正式英文名稱：Item
- 中文名稱：物品
- 定義：玩家可以取得並保存在 Inventory，或放在 Location 地面的離散物品種類。每種 Item 定義正整數單位重量與耐久時間上限。
- 對應行為：[REQ-006 - Gathering and inventory](../requirements/REQ-006.md)、[REQ-008 - Action: convert](../requirements/REQ-008.md)、[REQ-009 - Action: craft](../requirements/REQ-009.md)、[REQ-010 - Building construction](../requirements/REQ-010.md)、[REQ-013 - Ground asset transfers](../requirements/REQ-013.md)、[REQ-014 - 玩家攜帶重量](../requirements/REQ-014.md)、[REQ-015 - Item durability](../requirements/REQ-015.md)
- 與相似名詞的差異：Item 具有耐久狀態。Resource 不會失效，也不轉換成 Item。

### Active Item

- 正式英文名稱：Active Item
- 中文名稱：有效物品
- 定義：剩餘耐久時間大於 0，可以 Transfer 或作為 Action input 的 Item 堆疊。
- 對應行為：[REQ-015 - Item durability](../requirements/REQ-015.md)
- 與相似名詞的差異：Active Item 可以使用。Expired Item 只能查看或 Drop。

### Expired Item

- 正式英文名稱：Expired Item
- 中文名稱：失效物品
- 定義：耐久時間到達 0，不能恢復有效、使用或 Pickup，但仍在一天保留期內顯示並計入攜帶重量的 Item 堆疊。
- 對應行為：[REQ-013 - Ground asset transfers](../requirements/REQ-013.md)、[REQ-015 - Item durability](../requirements/REQ-015.md)
- 與相似名詞的差異：Expired Item 可以 Drop。Active Item 可以 Pickup 與作為 Action input。

### Item durability

- 正式英文名稱：Item durability
- 中文名稱：物品耐久度
- 定義：後端依 Item definition 的耐久上限與 UTC Unix seconds 計算 Item 堆疊剩餘有效時間的規則。
- 對應行為：[REQ-015 - Item durability](../requirements/REQ-015.md)
- 與相似名詞的差異：Item durability 不可修復。Building durability 可以由玩家維修。

### Location

- 正式英文名稱：Location
- 中文名稱：位置
- 定義：後端允許玩家停留、建立 Building，並累積公共地面資產與 Monster population 的遊戲地點。
- 對應行為：[REQ-005 - Action: move](../requirements/REQ-005.md)、[REQ-010 - Building construction](../requirements/REQ-010.md)、[REQ-013 - Ground asset transfers](../requirements/REQ-013.md)、[REQ-025 - Location Monster population](../requirements/REQ-025.md)
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

### Package Item

- 正式英文名稱：Package Item
- 中文名稱：擴充套件物品
- 定義：Craft 產生並保存在 Inventory 的 Active Item。Building owner 安裝 extension 時消耗它。
- 對應行為：[REQ-016 - Building extension lifecycle](../requirements/REQ-016.md)
- 與相似名詞的差異：Package Item 可以持有與 Transfer。安裝後的 Building extension 屬於 Building。

### Player name

- 正式英文名稱：Player name
- 中文名稱：玩家名稱
- 定義：玩家初次進入遊戲時選擇的持久化遊戲名稱。它會去除首尾空白，依 ASCII 字元 1 點與其他 Unicode 字元 2 點計算 1 至 16 點長度，並通過 NFKC 與 ASCII 英文字母大小寫規則判定唯一性。
- 範例：`Dolacone` 與 `旅人` 都是 Player name。後者使用 4 個中文字，占 8 點長度。
- 對應行為：[REQ-020 - Player name](../requirements/REQ-020.md)
- 與相似名詞的差異：Player name 是遊戲內公開名稱。Google 顯示名稱、email 與 application user ID 屬於登入身分資料。

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

### Sawmill T1

- 正式英文名稱：Sawmill T1
- 中文名稱：鋸木廠 T1
- 定義：第一種 Building extension。它完成施工後提供 Sawmill Wood Convert method，並在每次成功使用時消耗 parent Building 耐久。
- 對應行為：[REQ-017 - Sawmill T1](../requirements/REQ-017.md)
- 與相似名詞的差異：Sawmill T1 是已安裝的功能物件。Sawmill Package T1 是安裝時消耗的 Package Item。

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
- 定義：把玩家持有的 Active Item 或 Expired Item quantity 轉移至目前 Location 的公共地面。Resource 不提供 Drop。
- 對應行為：[REQ-013 - Ground asset transfers](../requirements/REQ-013.md)
- 與相似名詞的差異：Drop 的來源是玩家持有的 Item。Resource 只能透過 Pickup 從地面轉入玩家狀態。

### Transfer: Pickup

- 正式英文名稱：Transfer: Pickup
- 中文名稱：撿取
- 定義：把目前 Location 的公共 Active Item 或 Resource quantity 轉移給玩家。Expired Item 不能 Pickup。
- 對應行為：[REQ-013 - Ground asset transfers](../requirements/REQ-013.md)
- 與相似名詞的差異：Pickup 的來源是地面資產。Drop 的來源是玩家持有狀態。
