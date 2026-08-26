---
title: "SQLite Schemas"
doc_type: schema
last_reviewed: 2026-08-26
source_paths:
  - internal/authapi/store.go
---

# SQLite Schemas

## 更新規則

[`internal/authapi/store.go`](../internal/authapi/store.go) 是可執行 schema 的來源。SQLite table、column、index、constraint、initialization 或 backfill 有變更時，必須在同一 commit 更新本文件。

本文件記錄 production database 的完整結構與生命週期。測試建立的臨時 schema 不屬於 production schema。

## Database 設定

| 設定 | 值 | 用途 |
|---|---:|---|
| `PRAGMA foreign_keys` | `ON` | 強制執行玩家資料與身分、位置、Route 的 foreign key。 |
| `PRAGMA busy_timeout` | `5000` ms | SQLite 遇到 lock 時最多等待 5 秒。 |
| Max open connections | `1` | 單一 process 內序列化 SQLite connection。 |
| Max idle connections | `1` | 保留最多一條 idle connection。 |

所有時間欄位都使用 UTC Unix nanoseconds，儲存型別為 `INTEGER`。

## Schema 總覽

| Table | 用途 | 生命週期 | 與相似 table 的差異 |
|---|---|---|---|
| `identities` | 保存 Google SSO 對應的應用程式使用者。 | 使用者首次登入時建立，後續登入更新。 | 它是永久身分，不是可撤銷的登入 session。 |
| `oauth_attempts` | 保存尚未完成的 OAuth handshake。 | 登入開始時建立，完成時消耗，逾時後失效。 | 它只驗證一次 OAuth callback，不代表登入狀態。 |
| `sessions` | 保存應用程式登入 session。 | OAuth 完成時建立，到期後失效。 | 它代表已登入瀏覽器，不保存 Google OAuth token。 |
| `player_ap` | 保存玩家恢復至滿 AP 的時間。 | 身分建立時建立，消耗 AP 時更新。 | 它只保存 `full_timestamp`，不保存 AP 現值。 |
| `locations` | 保存後端允許的位置。 | Store 初始化時建立固定 seed。 | 它定義位置，不保存玩家狀態。 |
| `routes` | 保存後端允許的有向 Route 與 AP 成本。 | Store 初始化時建立固定 seed。 | 它定義兩個位置間的通行規則，不保存玩家移動紀錄。 |
| `player_locations` | 保存每位玩家的目前位置。 | 身分建立時建立，移動成功時更新。 | 它只保存目前狀態，不保存歷史軌跡。 |

## identities

用途：將 Google provider identity 對應至穩定的應用程式 `id`。相同 `(issuer, subject)` 再次登入時，系統保留 `id` 與 `created_at`，並更新 email、顯示名稱與 `updated_at`。

```sql
CREATE TABLE IF NOT EXISTS identities (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    issuer TEXT NOT NULL,
    subject TEXT NOT NULL,
    email TEXT NOT NULL,
    display_name TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    UNIQUE (issuer, subject)
);
```

| Column | 用途 |
|---|---|
| `id` | 穩定的 application user ID，也是其他玩家資料的 foreign key。 |
| `issuer` | Google 回傳的 OpenID Connect issuer。 |
| `subject` | 該 issuer 下穩定且唯一的使用者識別值。 |
| `email` | 最近一次成功登入取得的 email。它不是唯一鍵。 |
| `display_name` | 最近一次成功登入取得的顯示名稱。 |
| `created_at` | 身分首次建立時間。 |
| `updated_at` | 身分最近一次成功登入更新時間。 |

索引與約束：primary key 為 `id`。`UNIQUE (issuer, subject)` 防止同一 provider identity 建立重複使用者。沒有 email unique constraint。

## oauth_attempts

用途：保存 OAuth `state`、PKCE verifier 與 nonce，讓 callback 只能完成一次原始登入流程。成功消耗時，系統清空 `nonce` 與 `verifier`，並寫入 `consumed_at`。

```sql
CREATE TABLE IF NOT EXISTS oauth_attempts (
    state_hash BLOB PRIMARY KEY,
    browser_token_hash BLOB,
    nonce TEXT NOT NULL,
    verifier TEXT NOT NULL,
    expires_at INTEGER NOT NULL,
    consumed_at INTEGER
);
```

| Column | 用途 |
|---|---|
| `state_hash` | OAuth state 的 SHA-256 hex bytes。原始 state 不會儲存。 |
| `browser_token_hash` | 綁定發起登入瀏覽器的 token hash。舊資料可以是 `NULL`。 |
| `nonce` | 驗證 Google identity token 對應本次登入。消耗後改為空字串。 |
| `verifier` | OAuth PKCE verifier。消耗後改為空字串。 |
| `expires_at` | 登入嘗試失效時間。 |
| `consumed_at` | 成功消耗時間。未消耗時為 `NULL`。 |

索引與約束：primary key 為 `state_hash`。此 table 不連到 `identities`，因為 OAuth callback 完成前尚未確認 application user。

## sessions

用途：保存 OAuth 成功後建立的應用程式 session。API 先用 cookie 內的原始 token 計算 hash，再查找有效 session 與對應使用者。

```sql
CREATE TABLE IF NOT EXISTS sessions (
    token_hash BLOB PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES identities(id),
    expires_at INTEGER NOT NULL,
    created_at INTEGER NOT NULL
);
```

| Column | 用途 |
|---|---|
| `token_hash` | Session token 的 SHA-256 hex bytes。原始 token 不會儲存。 |
| `user_id` | 登入的 application user ID。 |
| `expires_at` | Session 失效時間。 |
| `created_at` | Session 建立時間。 |

索引與約束：primary key 為 `token_hash`。`user_id` reference `identities(id)`，未指定 `ON DELETE`，因此使用 SQLite 預設的 `NO ACTION`。

## player_ap

用途：保存玩家恢復至 3000 AP 的時間。系統依後端目前時間計算 AP，不保存 AP 現值，也不使用背景排程更新 AP。

```sql
CREATE TABLE IF NOT EXISTS player_ap (
    user_id INTEGER PRIMARY KEY REFERENCES identities(id),
    full_timestamp INTEGER NOT NULL
);
```

| Column | 用途 |
|---|---|
| `user_id` | 玩家 ID。Primary key 保證每位玩家只有一筆 AP 狀態。 |
| `full_timestamp` | 玩家恢復至 3000 AP 的 UTC Unix nanoseconds timestamp。 |

索引與約束：`user_id` 同時是 primary key 與 `identities(id)` foreign key。未指定 `ON DELETE`，因此使用 SQLite 預設的 `NO ACTION`。

計算範例：`full_timestamp` 已到時，玩家有 3000 AP。`full_timestamp` 還有 10 個完整分鐘才到時，玩家有 2990 AP。剩餘時間包含未完成分鐘時，該分鐘仍算缺少 1 AP。

## locations

用途：定義後端允許的位置。MVP 固定建立 `camp` 與 `forest_edge`。

```sql
CREATE TABLE IF NOT EXISTS locations (
    id TEXT PRIMARY KEY,
    display_name TEXT NOT NULL
);
```

| Column | 用途 |
|---|---|
| `id` | API 與 Route 使用的穩定 location identifier。 |
| `display_name` | 前端顯示的位置名稱。 |

索引與約束：primary key 為 `id`。前端不能建立位置或自行提交顯示名稱。

## routes

用途：定義起點可到達的終點，以及該次 `move` 必須消耗的 AP。Route 是有向資料，反向移動需要另一筆資料。

```sql
CREATE TABLE IF NOT EXISTS routes (
    origin_id TEXT NOT NULL REFERENCES locations(id),
    destination_id TEXT NOT NULL REFERENCES locations(id),
    ap_cost INTEGER NOT NULL CHECK (ap_cost > 0),
    PRIMARY KEY (origin_id, destination_id)
);
```

| Column | 用途 |
|---|---|
| `origin_id` | Route 起點。它必須存在於 `locations`。 |
| `destination_id` | Route 終點。它必須存在於 `locations`。 |
| `ap_cost` | 成功移動時由後端扣除的 AP。 |

索引與約束：複合 primary key 防止相同方向出現兩筆 Route。`ap_cost` 必須大於 0。MVP seed 包含 `camp` 到 `forest_edge` 與反向 Route，兩者成本都是 20 AP。

## player_locations

用途：保存每位玩家的目前位置。玩家成功移動時，系統會在同一 transaction 更新本 table 與 `player_ap`。

```sql
CREATE TABLE IF NOT EXISTS player_locations (
    user_id INTEGER PRIMARY KEY REFERENCES identities(id),
    location_id TEXT NOT NULL REFERENCES locations(id)
);
```

| Column | 用途 |
|---|---|
| `user_id` | 玩家 ID。Primary key 保證每位玩家只有一個目前位置。 |
| `location_id` | 玩家目前位置。它必須存在於 `locations`。 |

索引與約束：`user_id` 與 `location_id` 都受 foreign key 約束。新玩家與缺少位置的既有玩家使用 `camp`。

## 關聯與約束

```text
identities.id
├── sessions.user_id    多筆 session 對一位使用者
├── player_ap.user_id   一筆 AP 狀態對一位使用者
└── player_locations.user_id  一個目前位置對一位使用者

locations.id
├── routes.origin_id             Route 起點
├── routes.destination_id        Route 終點
└── player_locations.location_id 玩家目前位置

oauth_attempts          OAuth 完成前的獨立暫存狀態
```

目前沒有明確的 `CREATE INDEX`。SQLite 會為 primary key 與 `UNIQUE (issuer, subject)` 維護索引。

## 初始化與升級

`NewStore` 在同一 transaction 內建立 table，加入 location 與 Route seed，再 backfill 玩家資料。任何步驟失敗時，transaction 會 rollback。

```sql
INSERT OR IGNORE INTO player_ap (user_id, full_timestamp)
SELECT id, ? FROM identities;

INSERT OR IGNORE INTO player_locations (user_id, location_id)
SELECT id, 'camp' FROM identities;
```

新 identity 也會在 identity upsert 的同一 transaction 建立 `player_ap` 與 `player_locations`。既有玩家資料不會被 backfill 覆寫。

`move` transaction 會依玩家目前位置查找 target Route。Route 不存在或 AP 不足時，transaction 不修改資料。成功時，系統將 `full_timestamp` 向後推進 `ap_cost` 分鐘，並更新 `player_locations.location_id`。

## 已知限制

- Schema 目前直接寫在 `NewStore`，沒有編號 migration 檔案或 schema version table。
- 到期的 `oauth_attempts` 與 `sessions` 會被讀取流程拒絕，但目前沒有自動刪除工作。
- Foreign key 沒有 `ON DELETE CASCADE`。刪除仍被 reference 的 identity 會失敗。
- Production 必須維持單一 Fly.io Machine。多 process 寫入不在目前 SQLite concurrency scope。
