<!-- last_reviewed: 2026-08-25 -->

# Game Terminology

## 更新規則

本文件是遊戲名詞的正式索引。新增、重新命名或重新定義遊戲名詞時，必須在同一 commit 更新本文件。

本文件只定義名詞。可檢查的遊戲行為由對應 REQ 定義，技術資料結構由 [SQLite Schemas](schemas.md) 定義。

## 名詞索引

| 英文名稱 | 縮寫 | 中文名稱 | 對應 REQ |
|---|---|---|---|
| [Action Points](#action-points-ap) | AP | 行動力 | [REQ-003](../requirements/REQ-003.md) |

## 名詞定義

### Action Points (AP)

- 正式英文名稱：Action Points
- 縮寫：AP
- 中文名稱：行動力
- 定義：玩家角色執行行動時使用的資源，每分鐘恢復一點。
- 對應行為：[REQ-003 - AP 計算](../requirements/REQ-003.md)
- 與相似名詞的差異：AP 是目前可使用的行動力。`full_timestamp` 是後端計算 AP 的持久化資料，不是玩家持有的另一種資源。
