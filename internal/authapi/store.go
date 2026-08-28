package authapi

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

var (
	ErrInvalidArgument           = errors.New("invalid argument")
	ErrIdentityNotFound          = errors.New("identity not found")
	ErrOAuthAttemptNotFound      = errors.New("oauth attempt not found")
	ErrOAuthAttemptExpired       = errors.New("oauth attempt expired")
	ErrOAuthAttemptConsumed      = errors.New("oauth attempt already consumed")
	ErrSessionNotFound           = errors.New("session not found")
	ErrSessionExpired            = errors.New("session expired")
	ErrInsufficientAP            = errors.New("insufficient action points")
	ErrRouteNotFound             = errors.New("route not found")
	ErrGatheringNotFound         = errors.New("gathering not found")
	ErrConversionNotFound        = errors.New("conversion not found")
	ErrInsufficientItem          = errors.New("insufficient item")
	ErrCraftingNotFound          = errors.New("crafting recipe not found")
	ErrInsufficientResource      = errors.New("insufficient resource")
	ErrBuildingNotFound          = errors.New("building not found")
	ErrBuildingOccupied          = errors.New("building location already occupied")
	ErrBuildingRemote            = errors.New("building is at another location")
	ErrBuildingCompleted         = errors.New("building is already completed")
	ErrBuildingUnderConstruction = errors.New("building is under construction")
	ErrTransferAssetNotFound     = errors.New("transfer asset not found")
	ErrInsufficientTransferAsset = errors.New("insufficient transfer asset")
)

type Store struct {
	db  *sql.DB
	now func() time.Time
}

type Identity struct {
	ID          int64
	Issuer      string
	Subject     string
	Email       string
	DisplayName string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type OAuthAttempt struct {
	Nonce     string
	Verifier  string
	ExpiresAt time.Time
}

type Session struct {
	UserID    int64
	ExpiresAt time.Time
}

type Location struct {
	ID          string
	DisplayName string
}

type Route struct {
	OriginID      string
	DestinationID string
	APCost        int
}

type Item struct {
	ID          string
	DisplayName string
}

type InventoryItem struct {
	Item     Item
	Quantity int
}

type GatheringOption struct {
	Item     Item
	Quantity int
	APCost   int
}

type ConversionOption struct {
	Item          Item
	Resource      ResourceType
	InputQuantity int
	ResourceYield int
	APCost        int
}

type ResourceType struct {
	ID          string
	DisplayName string
}

type PlayerResource struct {
	Resource ResourceType
	Quantity int
}

type GroundItem struct {
	Item     Item
	Quantity int
}

type GroundResource struct {
	Resource ResourceType
	Quantity int
}

type CraftingResourceInput struct {
	Resource ResourceType
	Quantity int
}

type CraftingItemInput struct {
	Item     Item
	Quantity int
}

type CraftingRecipe struct {
	ID             string
	DisplayName    string
	BaseAPCost     int
	ResourceInputs []CraftingResourceInput
	ItemInputs     []CraftingItemInput
	Output         Item
	OutputQuantity int
}

type BuildingRecipe struct {
	ID                   string
	DisplayName          string
	BuildingLevel        int
	RequiredAP           int
	ExtensionSlotCount   int
	MaxDurabilitySeconds int
	ResourceInputs       []CraftingResourceInput
	ItemInputs           []CraftingItemInput
}

type BuildingOwner struct {
	ID          int64
	DisplayName string
}

type Building struct {
	ID                         int64
	Owner                      BuildingOwner
	Recipe                     BuildingRecipe
	BuildingLevel              int
	RequiredAP                 int
	ContributedAP              int
	Status                     string
	ExtensionSlotCount         int
	MaxDurabilitySeconds       int
	DurabilityStatus           string
	DurabilityRemainingSeconds int
}

type ConstructionComputation struct {
	BuildingID        int64
	EffectiveAP       int
	ResultingProgress int
	RequiredAP        int
	CompletionOutcome string
}

type RepairComputation struct {
	BuildingID                int64
	PriorDurabilityStatus     string
	AddedSeconds              int
	ResultingRemainingSeconds int
	APCost                    int
	WoodCost                  int
}

type PlayerState struct {
	Location                Location
	Routes                  []Route
	AP                      int
	Inventory               []InventoryItem
	GroundItems             []GroundItem
	GroundResources         []GroundResource
	GatheringOption         *GatheringOption
	ConversionOption        *ConversionOption
	Resources               []PlayerResource
	CraftingRecipes         []CraftingRecipe
	BuildingRecipes         []BuildingRecipe
	Buildings               []Building
	ConstructionComputation *ConstructionComputation
	RepairComputation       *RepairComputation
}

const (
	maxAP                     = 3000
	apRecoveryTime            = time.Minute
	buildingDefaultDurability = 7 * 24 * time.Hour
	buildingRepairDuration    = time.Hour
	buildingRepairAPCost      = 10
	buildingRepairWoodCost    = 1
	buildingDisabledRetention = 3 * 24 * time.Hour
	unixNanosecondsThreshold  = int64(1_000_000_000_000_000)
	nanosecondsPerSecond      = int64(time.Second)
)

func NewStore(db *sql.DB) (*Store, error) {
	if db == nil {
		return nil, fmt.Errorf("%w: nil database", ErrInvalidArgument)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if _, err := db.Exec(`PRAGMA foreign_keys = ON; PRAGMA busy_timeout = 5000;`); err != nil {
		return nil, fmt.Errorf("configure auth store: %w", err)
	}
	tx, err := db.Begin()
	if err != nil {
		return nil, fmt.Errorf("begin auth store initialization: %w", err)
	}
	if _, err := tx.Exec(`
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
	CREATE TABLE IF NOT EXISTS oauth_attempts (
	state_hash BLOB PRIMARY KEY,
	browser_token_hash BLOB,
	nonce TEXT NOT NULL,
	verifier TEXT NOT NULL,
	expires_at INTEGER NOT NULL,
	consumed_at INTEGER
);
CREATE TABLE IF NOT EXISTS sessions (
	token_hash BLOB PRIMARY KEY,
	user_id INTEGER NOT NULL REFERENCES identities(id),
	expires_at INTEGER NOT NULL,
	created_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS player_ap (
	user_id INTEGER PRIMARY KEY REFERENCES identities(id),
	full_timestamp INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS locations (
	id TEXT PRIMARY KEY,
	display_name TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS routes (
	origin_id TEXT NOT NULL REFERENCES locations(id),
	destination_id TEXT NOT NULL REFERENCES locations(id),
	ap_cost INTEGER NOT NULL CHECK (ap_cost > 0),
	PRIMARY KEY (origin_id, destination_id)
);
CREATE TABLE IF NOT EXISTS player_locations (
	user_id INTEGER PRIMARY KEY REFERENCES identities(id),
	location_id TEXT NOT NULL REFERENCES locations(id)
);
CREATE TABLE IF NOT EXISTS items (
	id TEXT PRIMARY KEY,
	display_name TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS gathering_rules (
	location_id TEXT PRIMARY KEY REFERENCES locations(id),
	item_id TEXT NOT NULL REFERENCES items(id),
	quantity INTEGER NOT NULL CHECK (quantity > 0),
	ap_cost INTEGER NOT NULL CHECK (ap_cost > 0)
);
CREATE TABLE IF NOT EXISTS player_inventory (
	user_id INTEGER NOT NULL REFERENCES identities(id),
	item_id TEXT NOT NULL REFERENCES items(id),
	quantity INTEGER NOT NULL CHECK (quantity > 0),
	PRIMARY KEY (user_id, item_id)
);
CREATE TABLE IF NOT EXISTS resource_types (
	id TEXT PRIMARY KEY,
	display_name TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS conversion_rules (
	location_id TEXT PRIMARY KEY REFERENCES locations(id),
	input_item_id TEXT NOT NULL REFERENCES items(id),
	input_quantity INTEGER NOT NULL CHECK (input_quantity > 0),
	output_resource_id TEXT NOT NULL REFERENCES resource_types(id),
	resource_yield INTEGER NOT NULL CHECK (resource_yield > 0),
	ap_cost INTEGER NOT NULL CHECK (ap_cost > 0)
);
CREATE TABLE IF NOT EXISTS player_resources (
	user_id INTEGER NOT NULL REFERENCES identities(id),
	resource_id TEXT NOT NULL REFERENCES resource_types(id),
	quantity INTEGER NOT NULL CHECK (quantity >= 0),
	PRIMARY KEY (user_id, resource_id)
);
CREATE TABLE IF NOT EXISTS ground_items (
	location_id TEXT NOT NULL REFERENCES locations(id),
	item_id TEXT NOT NULL REFERENCES items(id),
	quantity INTEGER NOT NULL CHECK (quantity > 0),
	PRIMARY KEY (location_id, item_id)
);
CREATE TABLE IF NOT EXISTS ground_resources (
	location_id TEXT NOT NULL REFERENCES locations(id),
	resource_id TEXT NOT NULL REFERENCES resource_types(id),
	quantity INTEGER NOT NULL CHECK (quantity > 0),
	PRIMARY KEY (location_id, resource_id)
);
CREATE TABLE IF NOT EXISTS crafting_recipes (
	id TEXT PRIMARY KEY,
	display_name TEXT NOT NULL,
	base_ap_cost INTEGER NOT NULL CHECK (base_ap_cost > 0),
	output_item_id TEXT NOT NULL REFERENCES items(id),
	output_quantity INTEGER NOT NULL CHECK (output_quantity > 0)
);
CREATE TABLE IF NOT EXISTS crafting_recipe_resource_inputs (
	recipe_id TEXT NOT NULL REFERENCES crafting_recipes(id),
	resource_id TEXT NOT NULL REFERENCES resource_types(id),
	quantity INTEGER NOT NULL CHECK (quantity > 0),
	PRIMARY KEY (recipe_id, resource_id)
);
CREATE TABLE IF NOT EXISTS crafting_recipe_item_inputs (
	recipe_id TEXT NOT NULL REFERENCES crafting_recipes(id),
	item_id TEXT NOT NULL REFERENCES items(id),
	quantity INTEGER NOT NULL CHECK (quantity > 0),
	PRIMARY KEY (recipe_id, item_id)
);
CREATE TABLE IF NOT EXISTS building_recipes (
	id TEXT PRIMARY KEY,
	display_name TEXT NOT NULL,
	building_level INTEGER NOT NULL CHECK (building_level > 0),
	required_ap INTEGER NOT NULL CHECK (required_ap > 0),
	extension_slot_count INTEGER NOT NULL CHECK (extension_slot_count >= 0),
	max_durability_seconds INTEGER NOT NULL DEFAULT 604800 CHECK (max_durability_seconds > 0)
);
CREATE TABLE IF NOT EXISTS building_recipe_resource_inputs (
	recipe_id TEXT NOT NULL REFERENCES building_recipes(id),
	resource_id TEXT NOT NULL REFERENCES resource_types(id),
	quantity INTEGER NOT NULL CHECK (quantity > 0),
	PRIMARY KEY (recipe_id, resource_id)
);
CREATE TABLE IF NOT EXISTS building_recipe_item_inputs (
	recipe_id TEXT NOT NULL REFERENCES building_recipes(id),
	item_id TEXT NOT NULL REFERENCES items(id),
	quantity INTEGER NOT NULL CHECK (quantity > 0),
	PRIMARY KEY (recipe_id, item_id)
);
CREATE TABLE IF NOT EXISTS buildings (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	owner_id INTEGER NOT NULL REFERENCES identities(id),
	location_id TEXT NOT NULL REFERENCES locations(id),
	recipe_id TEXT NOT NULL REFERENCES building_recipes(id),
	display_name TEXT NOT NULL DEFAULT '',
	building_level INTEGER NOT NULL CHECK (building_level > 0),
	required_ap INTEGER NOT NULL CHECK (required_ap > 0),
	contributed_ap INTEGER NOT NULL CHECK (contributed_ap >= 0 AND contributed_ap <= required_ap),
	status TEXT NOT NULL CHECK (status IN ('under_construction', 'completed')),
	extension_slot_count INTEGER NOT NULL CHECK (extension_slot_count >= 0),
	max_durability_seconds INTEGER NOT NULL DEFAULT 604800 CHECK (max_durability_seconds > 0),
	durability_expires_at INTEGER,
	UNIQUE (owner_id, location_id)
);`); err != nil {
		_ = tx.Rollback()
		return nil, fmt.Errorf("initialize auth store: %w", err)
	}
	migratedTimestampValues, err := migrateTimestampsToUnixSeconds(tx)
	if err != nil {
		_ = tx.Rollback()
		return nil, fmt.Errorf("migrate timestamps to Unix seconds: %w", err)
	}
	if err := ensureBuildingSchema(tx); err != nil {
		_ = tx.Rollback()
		return nil, fmt.Errorf("upgrade building schema: %w", err)
	}
	if _, err := tx.Exec(`
INSERT OR IGNORE INTO resource_types (id, display_name) VALUES
	('food', 'Food'),
	('wood', 'Wood'),
	('stone', 'Stone'),
	('metal', 'Metal'),
	('fiber', 'Fiber'),
	('hide', 'Hide'),
	('medicinal', 'Medicinal'),
	('arcane', 'Arcane')`); err != nil {
		_ = tx.Rollback()
		return nil, fmt.Errorf("seed resource types: %w", err)
	}
	if err := ensureTypedResourceSchema(tx); err != nil {
		_ = tx.Rollback()
		return nil, fmt.Errorf("upgrade typed resource schema: %w", err)
	}
	if _, err := tx.Exec(`
INSERT OR IGNORE INTO locations (id, display_name) VALUES
	('camp', 'Camp'),
	('forest_edge', 'Forest Edge');
INSERT OR IGNORE INTO routes (origin_id, destination_id, ap_cost) VALUES
	('camp', 'forest_edge', 20),
	('forest_edge', 'camp', 20);
INSERT OR IGNORE INTO items (id, display_name) VALUES
	('wood', 'Wood'),
	('wood_component', 'Wood Component');
INSERT OR IGNORE INTO gathering_rules (location_id, item_id, quantity, ap_cost) VALUES
	('forest_edge', 'wood', 1, 10);
INSERT OR IGNORE INTO conversion_rules (location_id, input_item_id, input_quantity, output_resource_id, resource_yield, ap_cost) VALUES
	('camp', 'wood', 1, 'wood', 1, 1);
INSERT OR IGNORE INTO crafting_recipes (id, display_name, base_ap_cost, output_item_id, output_quantity) VALUES
	('wood_component', 'Wood Component', 10, 'wood_component', 1);
INSERT OR IGNORE INTO crafting_recipe_resource_inputs (recipe_id, resource_id, quantity) VALUES
	('wood_component', 'wood', 10);`); err != nil {
		_ = tx.Rollback()
		return nil, fmt.Errorf("seed movement state: %w", err)
	}
	if _, err := tx.Exec(`
INSERT OR IGNORE INTO building_recipes (id, display_name, building_level, required_ap, extension_slot_count) VALUES
	('building_lv1', 'Building Lv1', 1, 60, 1);
INSERT OR IGNORE INTO building_recipe_item_inputs (recipe_id, item_id, quantity) VALUES
	('building_lv1', 'wood_component', 1);
INSERT OR IGNORE INTO building_recipe_resource_inputs (recipe_id, resource_id, quantity) VALUES
	('building_lv1', 'wood', 10);`); err != nil {
		_ = tx.Rollback()
		return nil, fmt.Errorf("seed building state: %w", err)
	}
	if _, err := tx.Exec(`
INSERT OR IGNORE INTO player_ap (user_id, full_timestamp)
SELECT id, ? FROM identities`, time.Now().UTC().Unix()); err != nil {
		_ = tx.Rollback()
		return nil, fmt.Errorf("backfill player AP: %w", err)
	}
	if _, err := tx.Exec(`
INSERT OR IGNORE INTO player_locations (user_id, location_id)
SELECT id, 'camp' FROM identities`); err != nil {
		_ = tx.Rollback()
		return nil, fmt.Errorf("backfill player locations: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit auth store initialization: %w", err)
	}
	fmt.Fprintf(os.Stdout, "user_id=anonymous action=timestamp_migration outcome=success converted_values=%d request_id=unavailable\n", migratedTimestampValues)
	return &Store{db: db, now: time.Now}, nil
}

func migrateTimestampsToUnixSeconds(tx *sql.Tx) (int64, error) {
	timestampColumns := []struct {
		table  string
		column string
	}{
		{table: "identities", column: "created_at"},
		{table: "identities", column: "updated_at"},
		{table: "oauth_attempts", column: "expires_at"},
		{table: "oauth_attempts", column: "consumed_at"},
		{table: "sessions", column: "expires_at"},
		{table: "sessions", column: "created_at"},
		{table: "player_ap", column: "full_timestamp"},
	}
	var convertedValues int64
	for _, timestampColumn := range timestampColumns {
		query := fmt.Sprintf(
			"UPDATE %s SET %s = %s / ? WHERE %s >= ? OR %s <= -?",
			timestampColumn.table,
			timestampColumn.column,
			timestampColumn.column,
			timestampColumn.column,
			timestampColumn.column,
		)
		result, err := tx.Exec(query, nanosecondsPerSecond, unixNanosecondsThreshold, unixNanosecondsThreshold)
		if err != nil {
			return 0, fmt.Errorf("convert %s.%s: %w", timestampColumn.table, timestampColumn.column, err)
		}
		rows, err := result.RowsAffected()
		if err != nil {
			return 0, fmt.Errorf("count converted %s.%s: %w", timestampColumn.table, timestampColumn.column, err)
		}
		convertedValues += rows
	}
	return convertedValues, nil
}

func ensureTypedResourceSchema(tx *sql.Tx) error {
	playerResourceColumns, err := tableColumns(tx, "player_resources")
	if err != nil {
		return err
	}
	if playerResourceColumns["balance"] {
		if _, err := tx.Exec(`DROP TABLE player_resources`); err != nil {
			return fmt.Errorf("discard legacy player resources: %w", err)
		}
		if _, err := tx.Exec(`
CREATE TABLE player_resources (
	user_id INTEGER NOT NULL REFERENCES identities(id),
	resource_id TEXT NOT NULL REFERENCES resource_types(id),
	quantity INTEGER NOT NULL CHECK (quantity >= 0),
	PRIMARY KEY (user_id, resource_id)
)`); err != nil {
			return fmt.Errorf("create typed player resources: %w", err)
		}
	}

	conversionColumns, err := tableColumns(tx, "conversion_rules")
	if err != nil {
		return err
	}
	if !conversionColumns["output_resource_id"] {
		if _, err := tx.Exec(`ALTER TABLE conversion_rules RENAME TO conversion_rules_legacy`); err != nil {
			return fmt.Errorf("rename legacy conversion rules: %w", err)
		}
		if _, err := tx.Exec(`
CREATE TABLE conversion_rules (
	location_id TEXT PRIMARY KEY REFERENCES locations(id),
	input_item_id TEXT NOT NULL REFERENCES items(id),
	input_quantity INTEGER NOT NULL CHECK (input_quantity > 0),
	output_resource_id TEXT NOT NULL REFERENCES resource_types(id),
	resource_yield INTEGER NOT NULL CHECK (resource_yield > 0),
	ap_cost INTEGER NOT NULL CHECK (ap_cost > 0)
)`); err != nil {
			return fmt.Errorf("create typed conversion rules: %w", err)
		}
		if _, err := tx.Exec(`
INSERT INTO conversion_rules (location_id, input_item_id, input_quantity, output_resource_id, resource_yield, ap_cost)
SELECT location_id, input_item_id, input_quantity, 'wood', resource_yield, ap_cost
FROM conversion_rules_legacy`); err != nil {
			return fmt.Errorf("migrate conversion rules: %w", err)
		}
		if _, err := tx.Exec(`DROP TABLE conversion_rules_legacy`); err != nil {
			return fmt.Errorf("drop legacy conversion rules: %w", err)
		}
	}
	return nil
}

func ensureBuildingSchema(tx *sql.Tx) error {
	const defaultDurabilitySeconds = int64(buildingDefaultDurability / time.Second)
	recipeColumns, err := tableColumns(tx, "building_recipes")
	if err != nil {
		return err
	}
	if !recipeColumns["max_durability_seconds"] {
		if _, err := tx.Exec(fmt.Sprintf(`ALTER TABLE building_recipes ADD COLUMN max_durability_seconds INTEGER NOT NULL DEFAULT %d CHECK (max_durability_seconds > 0)`, defaultDurabilitySeconds)); err != nil {
			return fmt.Errorf("add building recipe durability: %w", err)
		}
	}
	columns, err := tableColumns(tx, "buildings")
	if err != nil {
		return err
	}
	if !columns["display_name"] {
		if _, err := tx.Exec(`ALTER TABLE buildings ADD COLUMN display_name TEXT NOT NULL DEFAULT ''`); err != nil {
			return fmt.Errorf("add building display name: %w", err)
		}
		if _, err := tx.Exec(`
UPDATE buildings
SET display_name = (SELECT display_name FROM building_recipes WHERE building_recipes.id = buildings.recipe_id)
WHERE display_name = ''`); err != nil {
			return fmt.Errorf("backfill building display name: %w", err)
		}
	}
	if !columns["max_durability_seconds"] {
		if _, err := tx.Exec(fmt.Sprintf(`ALTER TABLE buildings ADD COLUMN max_durability_seconds INTEGER NOT NULL DEFAULT %d CHECK (max_durability_seconds > 0)`, defaultDurabilitySeconds)); err != nil {
			return fmt.Errorf("add building durability: %w", err)
		}
	}
	if !columns["durability_expires_at"] {
		if _, err := tx.Exec(`ALTER TABLE buildings ADD COLUMN durability_expires_at INTEGER`); err != nil {
			return fmt.Errorf("add building durability expiry: %w", err)
		}
	}
	if _, err := tx.Exec(`
UPDATE buildings
SET durability_expires_at = ? + max_durability_seconds
WHERE status = 'completed' AND durability_expires_at IS NULL`, time.Now().UTC().Unix()); err != nil {
		return fmt.Errorf("backfill building durability expiry: %w", err)
	}
	return nil
}

func tableColumns(tx *sql.Tx, table string) (map[string]bool, error) {
	rows, err := tx.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		return nil, fmt.Errorf("inspect %s: %w", table, err)
	}
	defer rows.Close()
	columns := make(map[string]bool)
	for rows.Next() {
		var cid, notNull, primaryKey int
		var name, columnType string
		var defaultValue sql.NullString
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			return nil, fmt.Errorf("scan %s columns: %w", table, err)
		}
		columns[name] = true
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read %s columns: %w", table, err)
	}
	return columns, nil
}

func (s *Store) UpsertIdentity(issuer, subject, email, displayName string) (Identity, error) {
	if strings.TrimSpace(issuer) == "" || strings.TrimSpace(subject) == "" {
		return Identity{}, fmt.Errorf("%w: issuer and subject are required", ErrInvalidArgument)
	}
	now := s.now().UTC().Unix()
	tx, err := s.db.Begin()
	if err != nil {
		return Identity{}, fmt.Errorf("begin upsert identity: %w", err)
	}
	var identity Identity
	var createdAt, updatedAt int64
	err = tx.QueryRow(`
INSERT INTO identities (issuer, subject, email, display_name, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT (issuer, subject) DO UPDATE SET
	email = excluded.email,
	display_name = excluded.display_name,
	updated_at = excluded.updated_at
RETURNING id, issuer, subject, email, display_name, created_at, updated_at`,
		issuer, subject, email, displayName, now, now,
	).Scan(&identity.ID, &identity.Issuer, &identity.Subject, &identity.Email, &identity.DisplayName, &createdAt, &updatedAt)
	if err != nil {
		_ = tx.Rollback()
		return Identity{}, fmt.Errorf("upsert identity: %w", err)
	}
	if _, err := tx.Exec(`
INSERT INTO player_ap (user_id, full_timestamp) VALUES (?, ?)
ON CONFLICT (user_id) DO NOTHING`, identity.ID, now); err != nil {
		_ = tx.Rollback()
		return Identity{}, fmt.Errorf("initialize player AP: %w", err)
	}
	if _, err := tx.Exec(`
INSERT INTO player_locations (user_id, location_id) VALUES (?, 'camp')
ON CONFLICT (user_id) DO NOTHING`, identity.ID); err != nil {
		_ = tx.Rollback()
		return Identity{}, fmt.Errorf("initialize player location: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return Identity{}, fmt.Errorf("commit upsert identity: %w", err)
	}
	identity.CreatedAt = unixSeconds(createdAt)
	identity.UpdatedAt = unixSeconds(updatedAt)
	return identity, nil
}

func (s *Store) GetAP(userID int64) (int, error) {
	if userID <= 0 {
		return 0, fmt.Errorf("%w: user ID is required", ErrInvalidArgument)
	}
	var fullTimestamp int64
	err := s.db.QueryRow(`SELECT full_timestamp FROM player_ap WHERE user_id = ?`, userID).Scan(&fullTimestamp)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrIdentityNotFound
	}
	if err != nil {
		return 0, fmt.Errorf("get player AP: %w", err)
	}
	return calculateAP(unixSeconds(fullTimestamp), s.now().UTC()), nil
}

func (s *Store) Rest(userID int64) (int, error) {
	if userID <= 0 {
		return 0, fmt.Errorf("%w: user ID is required", ErrInvalidArgument)
	}
	tx, err := s.db.Begin()
	if err != nil {
		return 0, fmt.Errorf("begin rest: %w", err)
	}
	var fullTimestamp int64
	err = tx.QueryRow(`SELECT full_timestamp FROM player_ap WHERE user_id = ?`, userID).Scan(&fullTimestamp)
	if errors.Is(err, sql.ErrNoRows) {
		_ = tx.Rollback()
		return 0, ErrIdentityNotFound
	}
	if err != nil {
		_ = tx.Rollback()
		return 0, fmt.Errorf("get player AP for rest: %w", err)
	}
	now := s.now().UTC()
	if calculateAP(unixSeconds(fullTimestamp), now) == 0 {
		_ = tx.Rollback()
		return 0, ErrInsufficientAP
	}
	fullAt := unixSeconds(fullTimestamp)
	if fullAt.Before(now) {
		fullAt = now
	}
	nextFullTimestamp := fullAt.Add(apRecoveryTime).Unix()
	result, err := tx.Exec(`
UPDATE player_ap SET full_timestamp = ?
WHERE user_id = ? AND full_timestamp = ?`, nextFullTimestamp, userID, fullTimestamp)
	if err != nil {
		_ = tx.Rollback()
		return 0, fmt.Errorf("rest player: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		_ = tx.Rollback()
		return 0, fmt.Errorf("check rest player: %w", err)
	}
	if rows != 1 {
		_ = tx.Rollback()
		return 0, ErrInsufficientAP
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit rest player: %w", err)
	}
	return calculateAP(unixSeconds(nextFullTimestamp), now), nil
}

func (s *Store) GetPlayerState(userID int64) (PlayerState, error) {
	if userID <= 0 {
		return PlayerState{}, fmt.Errorf("%w: user ID is required", ErrInvalidArgument)
	}
	tx, err := s.db.Begin()
	if err != nil {
		return PlayerState{}, fmt.Errorf("begin player state read: %w", err)
	}
	state, err := s.getPlayerStateTx(tx, userID, s.now().UTC())
	if err != nil {
		_ = tx.Rollback()
		return PlayerState{}, err
	}
	if err := tx.Commit(); err != nil {
		return PlayerState{}, fmt.Errorf("commit player state read: %w", err)
	}
	return state, nil
}

func (s *Store) getPlayerStateTx(tx *sql.Tx, userID int64, now time.Time) (PlayerState, error) {
	state := PlayerState{Routes: make([]Route, 0), Inventory: make([]InventoryItem, 0), GroundItems: make([]GroundItem, 0), GroundResources: make([]GroundResource, 0), Resources: make([]PlayerResource, 0), CraftingRecipes: make([]CraftingRecipe, 0), BuildingRecipes: make([]BuildingRecipe, 0), Buildings: make([]Building, 0)}
	if err := deleteDestroyedBuildingsTx(tx, now); err != nil {
		return PlayerState{}, err
	}
	err := tx.QueryRow(`
SELECT l.id, l.display_name
FROM player_locations pl
JOIN locations l ON l.id = pl.location_id
WHERE pl.user_id = ?`, userID).Scan(&state.Location.ID, &state.Location.DisplayName)
	if errors.Is(err, sql.ErrNoRows) {
		return PlayerState{}, ErrIdentityNotFound
	}
	if err != nil {
		return PlayerState{}, fmt.Errorf("get player location: %w", err)
	}
	var gathering GatheringOption
	err = tx.QueryRow(`
SELECT i.id, i.display_name, gr.quantity, gr.ap_cost
FROM gathering_rules gr
JOIN items i ON i.id = gr.item_id
WHERE gr.location_id = ?`, state.Location.ID).Scan(
		&gathering.Item.ID, &gathering.Item.DisplayName, &gathering.Quantity, &gathering.APCost,
	)
	if errors.Is(err, sql.ErrNoRows) {
		state.GatheringOption = nil
	} else if err != nil {
		return PlayerState{}, fmt.Errorf("get player gathering option: %w", err)
	} else {
		state.GatheringOption = &gathering
	}
	conversion, err := conversionOptionForLocation(tx, state.Location.ID)
	if errors.Is(err, sql.ErrNoRows) {
		state.ConversionOption = nil
	} else if err != nil {
		return PlayerState{}, fmt.Errorf("get player conversion option: %w", err)
	} else {
		state.ConversionOption = &conversion
	}
	inventoryRows, err := tx.Query(`
SELECT i.id, i.display_name, pi.quantity
FROM player_inventory pi
JOIN items i ON i.id = pi.item_id
WHERE pi.user_id = ?
ORDER BY pi.item_id`, userID)
	if err != nil {
		return PlayerState{}, fmt.Errorf("get player inventory: %w", err)
	}
	defer inventoryRows.Close()
	for inventoryRows.Next() {
		var inventoryItem InventoryItem
		if err := inventoryRows.Scan(&inventoryItem.Item.ID, &inventoryItem.Item.DisplayName, &inventoryItem.Quantity); err != nil {
			return PlayerState{}, fmt.Errorf("scan player inventory: %w", err)
		}
		state.Inventory = append(state.Inventory, inventoryItem)
	}
	if err := inventoryRows.Err(); err != nil {
		return PlayerState{}, fmt.Errorf("read player inventory: %w", err)
	}
	var fullTimestamp int64
	err = tx.QueryRow(`SELECT full_timestamp FROM player_ap WHERE user_id = ?`, userID).Scan(&fullTimestamp)
	if errors.Is(err, sql.ErrNoRows) {
		return PlayerState{}, ErrIdentityNotFound
	}
	if err != nil {
		return PlayerState{}, fmt.Errorf("get player AP: %w", err)
	}
	state.AP = calculateAP(unixSeconds(fullTimestamp), now)
	resourceRows, err := tx.Query(`
SELECT rt.id, rt.display_name, COALESCE(pr.quantity, 0)
FROM resource_types rt
LEFT JOIN player_resources pr
	ON pr.resource_id = rt.id AND pr.user_id = ?
ORDER BY rt.id`, userID)
	if err != nil {
		return PlayerState{}, fmt.Errorf("get player resources: %w", err)
	}
	defer resourceRows.Close()
	for resourceRows.Next() {
		var resource PlayerResource
		if err := resourceRows.Scan(&resource.Resource.ID, &resource.Resource.DisplayName, &resource.Quantity); err != nil {
			return PlayerState{}, fmt.Errorf("scan player resources: %w", err)
		}
		state.Resources = append(state.Resources, resource)
	}
	if err := resourceRows.Err(); err != nil {
		return PlayerState{}, fmt.Errorf("read player resources: %w", err)
	}
	groundItemRows, err := tx.Query(`
SELECT i.id, i.display_name, gi.quantity
FROM ground_items gi
JOIN items i ON i.id = gi.item_id
WHERE gi.location_id = ?
ORDER BY gi.item_id`, state.Location.ID)
	if err != nil {
		return PlayerState{}, fmt.Errorf("get ground items: %w", err)
	}
	defer groundItemRows.Close()
	for groundItemRows.Next() {
		var groundItem GroundItem
		if err := groundItemRows.Scan(&groundItem.Item.ID, &groundItem.Item.DisplayName, &groundItem.Quantity); err != nil {
			return PlayerState{}, fmt.Errorf("scan ground item: %w", err)
		}
		state.GroundItems = append(state.GroundItems, groundItem)
	}
	if err := groundItemRows.Err(); err != nil {
		return PlayerState{}, fmt.Errorf("read ground items: %w", err)
	}
	groundResourceRows, err := tx.Query(`
SELECT rt.id, rt.display_name, gr.quantity
FROM ground_resources gr
JOIN resource_types rt ON rt.id = gr.resource_id
WHERE gr.location_id = ?
ORDER BY gr.resource_id`, state.Location.ID)
	if err != nil {
		return PlayerState{}, fmt.Errorf("get ground resources: %w", err)
	}
	defer groundResourceRows.Close()
	for groundResourceRows.Next() {
		var groundResource GroundResource
		if err := groundResourceRows.Scan(&groundResource.Resource.ID, &groundResource.Resource.DisplayName, &groundResource.Quantity); err != nil {
			return PlayerState{}, fmt.Errorf("scan ground resource: %w", err)
		}
		state.GroundResources = append(state.GroundResources, groundResource)
	}
	if err := groundResourceRows.Err(); err != nil {
		return PlayerState{}, fmt.Errorf("read ground resources: %w", err)
	}
	recipeRows, err := tx.Query(`
SELECT cr.id, cr.display_name, cr.base_ap_cost, i.id, i.display_name, cr.output_quantity
FROM crafting_recipes cr
JOIN items i ON i.id = cr.output_item_id
WHERE EXISTS (SELECT 1 FROM crafting_recipe_resource_inputs ri WHERE ri.recipe_id = cr.id)
ORDER BY cr.id`)
	if err != nil {
		return PlayerState{}, fmt.Errorf("get crafting recipes: %w", err)
	}
	defer recipeRows.Close()
	for recipeRows.Next() {
		var recipe CraftingRecipe
		if err := recipeRows.Scan(&recipe.ID, &recipe.DisplayName, &recipe.BaseAPCost, &recipe.Output.ID, &recipe.Output.DisplayName, &recipe.OutputQuantity); err != nil {
			return PlayerState{}, fmt.Errorf("scan crafting recipe: %w", err)
		}
		if err := loadCraftingInputsTx(tx, &recipe); err != nil {
			return PlayerState{}, err
		}
		state.CraftingRecipes = append(state.CraftingRecipes, recipe)
	}
	if err := recipeRows.Err(); err != nil {
		return PlayerState{}, fmt.Errorf("read crafting recipes: %w", err)
	}
	buildingRecipeRows, err := tx.Query(`
	SELECT br.id, br.display_name, br.building_level, br.required_ap, br.extension_slot_count, br.max_durability_seconds
FROM building_recipes br
WHERE EXISTS (SELECT 1 FROM building_recipe_resource_inputs ri WHERE ri.recipe_id = br.id)
   OR EXISTS (SELECT 1 FROM building_recipe_item_inputs ii WHERE ii.recipe_id = br.id)
ORDER BY br.id`)
	if err != nil {
		return PlayerState{}, fmt.Errorf("get building recipes: %w", err)
	}
	defer buildingRecipeRows.Close()
	for buildingRecipeRows.Next() {
		var recipe BuildingRecipe
		if err := buildingRecipeRows.Scan(&recipe.ID, &recipe.DisplayName, &recipe.BuildingLevel, &recipe.RequiredAP, &recipe.ExtensionSlotCount, &recipe.MaxDurabilitySeconds); err != nil {
			return PlayerState{}, fmt.Errorf("scan building recipe: %w", err)
		}
		if err := loadBuildingInputsTx(tx, &recipe); err != nil {
			return PlayerState{}, err
		}
		state.BuildingRecipes = append(state.BuildingRecipes, recipe)
	}
	if err := buildingRecipeRows.Err(); err != nil {
		return PlayerState{}, fmt.Errorf("read building recipes: %w", err)
	}
	buildingRows, err := tx.Query(`
	SELECT b.id, i.id, i.display_name, br.id, b.display_name,
	       b.building_level, b.required_ap, b.contributed_ap, b.status, b.extension_slot_count,
	       b.max_durability_seconds, b.durability_expires_at
FROM buildings b
JOIN identities i ON i.id = b.owner_id
JOIN building_recipes br ON br.id = b.recipe_id
WHERE b.location_id = ?
ORDER BY b.id`, state.Location.ID)
	if err != nil {
		return PlayerState{}, fmt.Errorf("get buildings: %w", err)
	}
	defer buildingRows.Close()
	for buildingRows.Next() {
		var building Building
		var durabilityExpiresAt sql.NullInt64
		if err := buildingRows.Scan(&building.ID, &building.Owner.ID, &building.Owner.DisplayName, &building.Recipe.ID, &building.Recipe.DisplayName, &building.BuildingLevel, &building.RequiredAP, &building.ContributedAP, &building.Status, &building.ExtensionSlotCount, &building.MaxDurabilitySeconds, &durabilityExpiresAt); err != nil {
			return PlayerState{}, fmt.Errorf("scan building: %w", err)
		}
		building.Recipe.MaxDurabilitySeconds = building.MaxDurabilitySeconds
		setBuildingDurability(&building, durabilityExpiresAt, now)
		state.Buildings = append(state.Buildings, building)
	}
	if err := buildingRows.Err(); err != nil {
		return PlayerState{}, fmt.Errorf("read buildings: %w", err)
	}
	rows, err := tx.Query(`
SELECT origin_id, destination_id, ap_cost
FROM routes
WHERE origin_id = ?
ORDER BY destination_id`, state.Location.ID)
	if err != nil {
		return PlayerState{}, fmt.Errorf("get player routes: %w", err)
	}
	for rows.Next() {
		var route Route
		if err := rows.Scan(&route.OriginID, &route.DestinationID, &route.APCost); err != nil {
			_ = rows.Close()
			return PlayerState{}, fmt.Errorf("scan player route: %w", err)
		}
		state.Routes = append(state.Routes, route)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return PlayerState{}, fmt.Errorf("read player routes: %w", err)
	}
	if err := rows.Close(); err != nil {
		return PlayerState{}, fmt.Errorf("close player routes: %w", err)
	}
	return state, nil
}

func setBuildingDurability(building *Building, expiresAt sql.NullInt64, now time.Time) {
	if building.Status != "completed" || !expiresAt.Valid {
		return
	}
	remaining := int(expiresAt.Int64 - now.Unix())
	if remaining > 0 {
		building.DurabilityStatus = "active"
	} else {
		building.DurabilityStatus = "disabled"
		remaining = 0
	}
	building.DurabilityRemainingSeconds = remaining
}

func deleteDestroyedBuildingsTx(tx *sql.Tx, now time.Time) error {
	cutoff := now.Unix() - int64(buildingDisabledRetention/time.Second)
	if _, err := tx.Exec(`
DELETE FROM buildings
WHERE status = 'completed' AND durability_expires_at IS NOT NULL AND durability_expires_at <= ?`, cutoff); err != nil {
		return fmt.Errorf("delete destroyed buildings: %w", err)
	}
	return nil
}

func loadBuildingInputsTx(tx *sql.Tx, recipe *BuildingRecipe) error {
	resourceRows, err := tx.Query(`
SELECT rt.id, rt.display_name, ri.quantity
FROM building_recipe_resource_inputs ri
JOIN resource_types rt ON rt.id = ri.resource_id
WHERE ri.recipe_id = ? ORDER BY ri.resource_id`, recipe.ID)
	if err != nil {
		return fmt.Errorf("get building resource inputs: %w", err)
	}
	for resourceRows.Next() {
		var input CraftingResourceInput
		if err := resourceRows.Scan(&input.Resource.ID, &input.Resource.DisplayName, &input.Quantity); err != nil {
			_ = resourceRows.Close()
			return fmt.Errorf("scan building resource input: %w", err)
		}
		recipe.ResourceInputs = append(recipe.ResourceInputs, input)
	}
	if err := resourceRows.Err(); err != nil {
		_ = resourceRows.Close()
		return fmt.Errorf("read building resource inputs: %w", err)
	}
	if err := resourceRows.Close(); err != nil {
		return fmt.Errorf("close building resource inputs: %w", err)
	}
	itemRows, err := tx.Query(`
SELECT i.id, i.display_name, ii.quantity
FROM building_recipe_item_inputs ii
JOIN items i ON i.id = ii.item_id
WHERE ii.recipe_id = ? ORDER BY ii.item_id`, recipe.ID)
	if err != nil {
		return fmt.Errorf("get building item inputs: %w", err)
	}
	for itemRows.Next() {
		var input CraftingItemInput
		if err := itemRows.Scan(&input.Item.ID, &input.Item.DisplayName, &input.Quantity); err != nil {
			_ = itemRows.Close()
			return fmt.Errorf("scan building item input: %w", err)
		}
		recipe.ItemInputs = append(recipe.ItemInputs, input)
	}
	if err := itemRows.Err(); err != nil {
		_ = itemRows.Close()
		return fmt.Errorf("read building item inputs: %w", err)
	}
	if err := itemRows.Close(); err != nil {
		return fmt.Errorf("close building item inputs: %w", err)
	}
	return nil
}

func loadCraftingInputsTx(tx *sql.Tx, recipe *CraftingRecipe) error {
	resourceRows, err := tx.Query(`
SELECT rt.id, rt.display_name, ri.quantity
FROM crafting_recipe_resource_inputs ri
JOIN resource_types rt ON rt.id = ri.resource_id
WHERE ri.recipe_id = ? ORDER BY ri.resource_id`, recipe.ID)
	if err != nil {
		return fmt.Errorf("get crafting resource inputs: %w", err)
	}
	for resourceRows.Next() {
		var input CraftingResourceInput
		if err := resourceRows.Scan(&input.Resource.ID, &input.Resource.DisplayName, &input.Quantity); err != nil {
			_ = resourceRows.Close()
			return fmt.Errorf("scan crafting resource input: %w", err)
		}
		recipe.ResourceInputs = append(recipe.ResourceInputs, input)
	}
	if err := resourceRows.Err(); err != nil {
		_ = resourceRows.Close()
		return fmt.Errorf("read crafting resource inputs: %w", err)
	}
	if err := resourceRows.Close(); err != nil {
		return fmt.Errorf("close crafting resource inputs: %w", err)
	}
	itemRows, err := tx.Query(`
SELECT i.id, i.display_name, ii.quantity
FROM crafting_recipe_item_inputs ii
JOIN items i ON i.id = ii.item_id
WHERE ii.recipe_id = ? ORDER BY ii.item_id`, recipe.ID)
	if err != nil {
		return fmt.Errorf("get crafting item inputs: %w", err)
	}
	for itemRows.Next() {
		var input CraftingItemInput
		if err := itemRows.Scan(&input.Item.ID, &input.Item.DisplayName, &input.Quantity); err != nil {
			_ = itemRows.Close()
			return fmt.Errorf("scan crafting item input: %w", err)
		}
		recipe.ItemInputs = append(recipe.ItemInputs, input)
	}
	if err := itemRows.Err(); err != nil {
		_ = itemRows.Close()
		return fmt.Errorf("read crafting item inputs: %w", err)
	}
	if err := itemRows.Close(); err != nil {
		return fmt.Errorf("close crafting item inputs: %w", err)
	}
	return nil
}

func craftingRecipeForID(tx *sql.Tx, recipeID string) (CraftingRecipe, error) {
	var recipe CraftingRecipe
	err := tx.QueryRow(`
SELECT cr.id, cr.display_name, cr.base_ap_cost, i.id, i.display_name, cr.output_quantity
FROM crafting_recipes cr
JOIN items i ON i.id = cr.output_item_id
WHERE cr.id = ? AND EXISTS (SELECT 1 FROM crafting_recipe_resource_inputs ri WHERE ri.recipe_id = cr.id)`, recipeID).Scan(
		&recipe.ID, &recipe.DisplayName, &recipe.BaseAPCost, &recipe.Output.ID, &recipe.Output.DisplayName, &recipe.OutputQuantity)
	if err != nil {
		return CraftingRecipe{}, err
	}
	if err := loadCraftingInputsTx(tx, &recipe); err != nil {
		return CraftingRecipe{}, err
	}
	return recipe, nil
}

func conversionOptionForLocation(tx *sql.Tx, locationID string) (ConversionOption, error) {
	var conversion ConversionOption
	err := tx.QueryRow(`
SELECT i.id, i.display_name, rt.id, rt.display_name, cr.input_quantity, cr.resource_yield, cr.ap_cost
FROM conversion_rules cr
JOIN items i ON i.id = cr.input_item_id
JOIN resource_types rt ON rt.id = cr.output_resource_id
WHERE cr.location_id = ?`, locationID).Scan(
		&conversion.Item.ID, &conversion.Item.DisplayName, &conversion.Resource.ID, &conversion.Resource.DisplayName, &conversion.InputQuantity,
		&conversion.ResourceYield, &conversion.APCost,
	)
	return conversion, err
}

func (s *Store) Gather(userID int64) (PlayerState, error) {
	if userID <= 0 {
		return PlayerState{}, fmt.Errorf("%w: user ID is required", ErrInvalidArgument)
	}
	tx, err := s.db.Begin()
	if err != nil {
		return PlayerState{}, fmt.Errorf("begin gather: %w", err)
	}
	var locationID string
	err = tx.QueryRow(`SELECT location_id FROM player_locations WHERE user_id = ?`, userID).Scan(&locationID)
	if errors.Is(err, sql.ErrNoRows) {
		_ = tx.Rollback()
		return PlayerState{}, ErrIdentityNotFound
	}
	if err != nil {
		_ = tx.Rollback()
		return PlayerState{}, fmt.Errorf("get player location for gather: %w", err)
	}
	var option GatheringOption
	err = tx.QueryRow(`
SELECT i.id, i.display_name, gr.quantity, gr.ap_cost
FROM gathering_rules gr
JOIN items i ON i.id = gr.item_id
WHERE gr.location_id = ?`, locationID).Scan(
		&option.Item.ID, &option.Item.DisplayName, &option.Quantity, &option.APCost,
	)
	if errors.Is(err, sql.ErrNoRows) {
		_ = tx.Rollback()
		return PlayerState{}, ErrGatheringNotFound
	}
	if err != nil {
		_ = tx.Rollback()
		return PlayerState{}, fmt.Errorf("get gathering rule: %w", err)
	}
	var fullTimestamp int64
	err = tx.QueryRow(`SELECT full_timestamp FROM player_ap WHERE user_id = ?`, userID).Scan(&fullTimestamp)
	if errors.Is(err, sql.ErrNoRows) {
		_ = tx.Rollback()
		return PlayerState{}, ErrIdentityNotFound
	}
	if err != nil {
		_ = tx.Rollback()
		return PlayerState{}, fmt.Errorf("get player AP for gather: %w", err)
	}
	now := s.now().UTC()
	if calculateAP(unixSeconds(fullTimestamp), now) < option.APCost {
		_ = tx.Rollback()
		return PlayerState{}, ErrInsufficientAP
	}
	fullAt := unixSeconds(fullTimestamp)
	if fullAt.Before(now) {
		fullAt = now
	}
	nextFullTimestamp := fullAt.Add(time.Duration(option.APCost) * apRecoveryTime).Unix()
	result, err := tx.Exec(`
UPDATE player_ap SET full_timestamp = ?
WHERE user_id = ? AND full_timestamp = ?`, nextFullTimestamp, userID, fullTimestamp)
	if err != nil {
		_ = tx.Rollback()
		return PlayerState{}, fmt.Errorf("consume AP for gather: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		_ = tx.Rollback()
		return PlayerState{}, fmt.Errorf("check gather AP: %w", err)
	}
	if rows != 1 {
		_ = tx.Rollback()
		return PlayerState{}, ErrInsufficientAP
	}
	_, err = tx.Exec(`
INSERT INTO player_inventory (user_id, item_id, quantity)
VALUES (?, ?, ?)
ON CONFLICT (user_id, item_id) DO UPDATE SET quantity = player_inventory.quantity + excluded.quantity`, userID, option.Item.ID, option.Quantity)
	if err != nil {
		_ = tx.Rollback()
		return PlayerState{}, fmt.Errorf("add gathered item: %w", err)
	}
	state, err := s.getPlayerStateTx(tx, userID, now)
	if err != nil {
		_ = tx.Rollback()
		return PlayerState{}, err
	}
	if err := tx.Commit(); err != nil {
		return PlayerState{}, fmt.Errorf("commit gather: %w", err)
	}
	return state, nil
}

func (s *Store) Drop(userID int64, assetType, assetID string, quantity int) (PlayerState, error) {
	if err := validateTransfer(userID, assetType, assetID, quantity); err != nil {
		return PlayerState{}, err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return PlayerState{}, fmt.Errorf("begin drop: %w", err)
	}
	locationID, err := playerLocationTx(tx, userID)
	if err != nil {
		_ = tx.Rollback()
		return PlayerState{}, err
	}
	switch assetType {
	case "item":
		if err := requireItemTx(tx, assetID); err != nil {
			_ = tx.Rollback()
			return PlayerState{}, err
		}
		if err := consumeTransferQuantityTx(tx, "player_inventory", "user_id", userID, "item_id", assetID, quantity, "dropped item"); err != nil {
			_ = tx.Rollback()
			return PlayerState{}, err
		}
		if _, err := tx.Exec(`
INSERT INTO ground_items (location_id, item_id, quantity)
VALUES (?, ?, ?)
ON CONFLICT (location_id, item_id) DO UPDATE SET quantity = ground_items.quantity + excluded.quantity`, locationID, assetID, quantity); err != nil {
			_ = tx.Rollback()
			return PlayerState{}, fmt.Errorf("add ground item: %w", err)
		}
	case "resource":
		if err := requireResourceTx(tx, assetID); err != nil {
			_ = tx.Rollback()
			return PlayerState{}, err
		}
		if err := consumeTransferQuantityTx(tx, "player_resources", "user_id", userID, "resource_id", assetID, quantity, "dropped resource"); err != nil {
			_ = tx.Rollback()
			return PlayerState{}, err
		}
		if _, err := tx.Exec(`
INSERT INTO ground_resources (location_id, resource_id, quantity)
VALUES (?, ?, ?)
ON CONFLICT (location_id, resource_id) DO UPDATE SET quantity = ground_resources.quantity + excluded.quantity`, locationID, assetID, quantity); err != nil {
			_ = tx.Rollback()
			return PlayerState{}, fmt.Errorf("add ground resource: %w", err)
		}
	}
	state, err := s.getPlayerStateTx(tx, userID, s.now().UTC())
	if err != nil {
		_ = tx.Rollback()
		return PlayerState{}, err
	}
	if err := tx.Commit(); err != nil {
		return PlayerState{}, fmt.Errorf("commit drop: %w", err)
	}
	return state, nil
}

func (s *Store) Pickup(userID int64, assetType, assetID string, quantity int) (PlayerState, error) {
	if err := validateTransfer(userID, assetType, assetID, quantity); err != nil {
		return PlayerState{}, err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return PlayerState{}, fmt.Errorf("begin pickup: %w", err)
	}
	locationID, err := playerLocationTx(tx, userID)
	if err != nil {
		_ = tx.Rollback()
		return PlayerState{}, err
	}
	switch assetType {
	case "item":
		if err := requireItemTx(tx, assetID); err != nil {
			_ = tx.Rollback()
			return PlayerState{}, err
		}
		if err := consumeTransferQuantityTx(tx, "ground_items", "location_id", locationID, "item_id", assetID, quantity, "ground item"); err != nil {
			_ = tx.Rollback()
			return PlayerState{}, err
		}
		if _, err := tx.Exec(`
INSERT INTO player_inventory (user_id, item_id, quantity)
VALUES (?, ?, ?)
ON CONFLICT (user_id, item_id) DO UPDATE SET quantity = player_inventory.quantity + excluded.quantity`, userID, assetID, quantity); err != nil {
			_ = tx.Rollback()
			return PlayerState{}, fmt.Errorf("add picked item: %w", err)
		}
	case "resource":
		if err := requireResourceTx(tx, assetID); err != nil {
			_ = tx.Rollback()
			return PlayerState{}, err
		}
		if err := consumeTransferQuantityTx(tx, "ground_resources", "location_id", locationID, "resource_id", assetID, quantity, "ground resource"); err != nil {
			_ = tx.Rollback()
			return PlayerState{}, err
		}
		if _, err := tx.Exec(`
INSERT INTO player_resources (user_id, resource_id, quantity)
VALUES (?, ?, ?)
ON CONFLICT (user_id, resource_id) DO UPDATE SET quantity = player_resources.quantity + excluded.quantity`, userID, assetID, quantity); err != nil {
			_ = tx.Rollback()
			return PlayerState{}, fmt.Errorf("add picked resource: %w", err)
		}
	}
	state, err := s.getPlayerStateTx(tx, userID, s.now().UTC())
	if err != nil {
		_ = tx.Rollback()
		return PlayerState{}, err
	}
	if err := tx.Commit(); err != nil {
		return PlayerState{}, fmt.Errorf("commit pickup: %w", err)
	}
	return state, nil
}

func consumeTransferQuantityTx(tx *sql.Tx, table, scopeColumn string, scopeValue any, assetColumn, assetID string, quantity int, label string) error {
	deleteQuery := fmt.Sprintf("DELETE FROM %s WHERE %s = ? AND %s = ? AND quantity = ?", table, scopeColumn, assetColumn)
	result, err := tx.Exec(deleteQuery, scopeValue, assetID, quantity)
	if err != nil {
		return fmt.Errorf("consume %s: %w", label, err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check %s: %w", label, err)
	}
	if rows == 0 {
		updateQuery := fmt.Sprintf("UPDATE %s SET quantity = quantity - ? WHERE %s = ? AND %s = ? AND quantity > ?", table, scopeColumn, assetColumn)
		result, err = tx.Exec(updateQuery, quantity, scopeValue, assetID, quantity)
		if err != nil {
			return fmt.Errorf("consume %s: %w", label, err)
		}
		rows, err = result.RowsAffected()
		if err != nil {
			return fmt.Errorf("check %s: %w", label, err)
		}
	}
	if rows != 1 {
		return ErrInsufficientTransferAsset
	}
	return nil
}

func validateTransfer(userID int64, assetType, assetID string, quantity int) error {
	if userID <= 0 || (assetType != "item" && assetType != "resource") || strings.TrimSpace(assetID) == "" || quantity <= 0 {
		return fmt.Errorf("%w: valid transfer asset and positive quantity are required", ErrInvalidArgument)
	}
	return nil
}

func playerLocationTx(tx *sql.Tx, userID int64) (string, error) {
	var locationID string
	err := tx.QueryRow(`SELECT location_id FROM player_locations WHERE user_id = ?`, userID).Scan(&locationID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrIdentityNotFound
	}
	if err != nil {
		return "", fmt.Errorf("get player location for transfer: %w", err)
	}
	return locationID, nil
}

func requireItemTx(tx *sql.Tx, itemID string) error {
	var exists int
	err := tx.QueryRow(`SELECT 1 FROM items WHERE id = ?`, itemID).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrTransferAssetNotFound
	}
	if err != nil {
		return fmt.Errorf("find transfer item: %w", err)
	}
	return nil
}

func requireResourceTx(tx *sql.Tx, resourceID string) error {
	var exists int
	err := tx.QueryRow(`SELECT 1 FROM resource_types WHERE id = ?`, resourceID).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrTransferAssetNotFound
	}
	if err != nil {
		return fmt.Errorf("find transfer resource: %w", err)
	}
	return nil
}

func (s *Store) Move(userID int64, targetID string) (PlayerState, error) {
	if userID <= 0 {
		return PlayerState{}, fmt.Errorf("%w: user ID is required", ErrInvalidArgument)
	}
	if strings.TrimSpace(targetID) == "" {
		return PlayerState{}, fmt.Errorf("%w: target is required", ErrInvalidArgument)
	}
	tx, err := s.db.Begin()
	if err != nil {
		return PlayerState{}, fmt.Errorf("begin move: %w", err)
	}
	var originID string
	err = tx.QueryRow(`SELECT location_id FROM player_locations WHERE user_id = ?`, userID).Scan(&originID)
	if errors.Is(err, sql.ErrNoRows) {
		_ = tx.Rollback()
		return PlayerState{}, ErrIdentityNotFound
	}
	if err != nil {
		_ = tx.Rollback()
		return PlayerState{}, fmt.Errorf("get player location for move: %w", err)
	}
	var route Route
	err = tx.QueryRow(`
SELECT origin_id, destination_id, ap_cost
FROM routes
WHERE origin_id = ? AND destination_id = ?`, originID, targetID).Scan(&route.OriginID, &route.DestinationID, &route.APCost)
	if errors.Is(err, sql.ErrNoRows) {
		_ = tx.Rollback()
		return PlayerState{}, ErrRouteNotFound
	}
	if err != nil {
		_ = tx.Rollback()
		return PlayerState{}, fmt.Errorf("get route for move: %w", err)
	}
	var fullTimestamp int64
	err = tx.QueryRow(`SELECT full_timestamp FROM player_ap WHERE user_id = ?`, userID).Scan(&fullTimestamp)
	if errors.Is(err, sql.ErrNoRows) {
		_ = tx.Rollback()
		return PlayerState{}, ErrIdentityNotFound
	}
	if err != nil {
		_ = tx.Rollback()
		return PlayerState{}, fmt.Errorf("get player AP for move: %w", err)
	}
	now := s.now().UTC()
	if calculateAP(unixSeconds(fullTimestamp), now) < route.APCost {
		_ = tx.Rollback()
		return PlayerState{}, ErrInsufficientAP
	}
	fullAt := unixSeconds(fullTimestamp)
	if fullAt.Before(now) {
		fullAt = now
	}
	nextFullTimestamp := fullAt.Add(time.Duration(route.APCost) * apRecoveryTime).Unix()
	result, err := tx.Exec(`
UPDATE player_ap SET full_timestamp = ?
WHERE user_id = ? AND full_timestamp = ?`, nextFullTimestamp, userID, fullTimestamp)
	if err != nil {
		_ = tx.Rollback()
		return PlayerState{}, fmt.Errorf("consume AP for move: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		_ = tx.Rollback()
		return PlayerState{}, fmt.Errorf("check move AP: %w", err)
	}
	if rows != 1 {
		_ = tx.Rollback()
		return PlayerState{}, ErrInsufficientAP
	}
	result, err = tx.Exec(`
UPDATE player_locations SET location_id = ?
WHERE user_id = ? AND location_id = ?`, route.DestinationID, userID, originID)
	if err != nil {
		_ = tx.Rollback()
		return PlayerState{}, fmt.Errorf("update player location: %w", err)
	}
	rows, err = result.RowsAffected()
	if err != nil {
		_ = tx.Rollback()
		return PlayerState{}, fmt.Errorf("check player location update: %w", err)
	}
	if rows != 1 {
		_ = tx.Rollback()
		return PlayerState{}, ErrRouteNotFound
	}
	state, err := s.getPlayerStateTx(tx, userID, now)
	if err != nil {
		_ = tx.Rollback()
		return PlayerState{}, err
	}
	if err := tx.Commit(); err != nil {
		return PlayerState{}, fmt.Errorf("commit move: %w", err)
	}
	return state, nil
}

func (s *Store) Convert(userID int64) (PlayerState, error) {
	if userID <= 0 {
		return PlayerState{}, fmt.Errorf("%w: user ID is required", ErrInvalidArgument)
	}
	tx, err := s.db.Begin()
	if err != nil {
		return PlayerState{}, fmt.Errorf("begin convert: %w", err)
	}
	var locationID string
	err = tx.QueryRow(`SELECT location_id FROM player_locations WHERE user_id = ?`, userID).Scan(&locationID)
	if errors.Is(err, sql.ErrNoRows) {
		_ = tx.Rollback()
		return PlayerState{}, ErrIdentityNotFound
	}
	if err != nil {
		_ = tx.Rollback()
		return PlayerState{}, fmt.Errorf("get player location for convert: %w", err)
	}
	option, err := conversionOptionForLocation(tx, locationID)
	if errors.Is(err, sql.ErrNoRows) {
		_ = tx.Rollback()
		return PlayerState{}, ErrConversionNotFound
	}
	if err != nil {
		_ = tx.Rollback()
		return PlayerState{}, fmt.Errorf("get conversion rule: %w", err)
	}
	var fullTimestamp int64
	err = tx.QueryRow(`SELECT full_timestamp FROM player_ap WHERE user_id = ?`, userID).Scan(&fullTimestamp)
	if errors.Is(err, sql.ErrNoRows) {
		_ = tx.Rollback()
		return PlayerState{}, ErrIdentityNotFound
	}
	if err != nil {
		_ = tx.Rollback()
		return PlayerState{}, fmt.Errorf("get player AP for convert: %w", err)
	}
	var itemQuantity int
	err = tx.QueryRow(`SELECT quantity FROM player_inventory WHERE user_id = ? AND item_id = ?`, userID, option.Item.ID).Scan(&itemQuantity)
	if errors.Is(err, sql.ErrNoRows) || itemQuantity < option.InputQuantity {
		_ = tx.Rollback()
		return PlayerState{}, ErrInsufficientItem
	}
	if err != nil {
		_ = tx.Rollback()
		return PlayerState{}, fmt.Errorf("get conversion item: %w", err)
	}
	now := s.now().UTC()
	if calculateAP(unixSeconds(fullTimestamp), now) < option.APCost {
		_ = tx.Rollback()
		return PlayerState{}, ErrInsufficientAP
	}
	fullAt := unixSeconds(fullTimestamp)
	if fullAt.Before(now) {
		fullAt = now
	}
	nextFullTimestamp := fullAt.Add(time.Duration(option.APCost) * apRecoveryTime).Unix()
	result, err := tx.Exec(`
UPDATE player_ap SET full_timestamp = ?
WHERE user_id = ? AND full_timestamp = ?`, nextFullTimestamp, userID, fullTimestamp)
	if err != nil {
		_ = tx.Rollback()
		return PlayerState{}, fmt.Errorf("consume AP for convert: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		_ = tx.Rollback()
		return PlayerState{}, fmt.Errorf("check convert AP: %w", err)
	}
	if rows != 1 {
		_ = tx.Rollback()
		return PlayerState{}, ErrInsufficientAP
	}
	if itemQuantity == option.InputQuantity {
		_, err = tx.Exec(`DELETE FROM player_inventory WHERE user_id = ? AND item_id = ?`, userID, option.Item.ID)
	} else {
		_, err = tx.Exec(`UPDATE player_inventory SET quantity = quantity - ? WHERE user_id = ? AND item_id = ?`, option.InputQuantity, userID, option.Item.ID)
	}
	if err != nil {
		_ = tx.Rollback()
		return PlayerState{}, fmt.Errorf("consume conversion item: %w", err)
	}
	_, err = tx.Exec(`
INSERT INTO player_resources (user_id, resource_id, quantity)
VALUES (?, ?, ?)
ON CONFLICT (user_id, resource_id) DO UPDATE SET quantity = player_resources.quantity + excluded.quantity`, userID, option.Resource.ID, option.ResourceYield)
	if err != nil {
		_ = tx.Rollback()
		return PlayerState{}, fmt.Errorf("add conversion resources: %w", err)
	}
	state, err := s.getPlayerStateTx(tx, userID, now)
	if err != nil {
		_ = tx.Rollback()
		return PlayerState{}, err
	}
	if err := tx.Commit(); err != nil {
		return PlayerState{}, fmt.Errorf("commit convert: %w", err)
	}
	return state, nil
}

func (s *Store) Craft(userID int64, recipeID string) (PlayerState, error) {
	if userID <= 0 || strings.TrimSpace(recipeID) == "" {
		return PlayerState{}, fmt.Errorf("%w: user ID and recipe ID are required", ErrInvalidArgument)
	}
	tx, err := s.db.Begin()
	if err != nil {
		return PlayerState{}, fmt.Errorf("begin craft: %w", err)
	}
	recipe, err := craftingRecipeForID(tx, recipeID)
	if errors.Is(err, sql.ErrNoRows) {
		_ = tx.Rollback()
		return PlayerState{}, ErrCraftingNotFound
	}
	if err != nil {
		_ = tx.Rollback()
		return PlayerState{}, fmt.Errorf("get crafting recipe: %w", err)
	}
	var fullTimestamp int64
	err = tx.QueryRow(`SELECT full_timestamp FROM player_ap WHERE user_id = ?`, userID).Scan(&fullTimestamp)
	if errors.Is(err, sql.ErrNoRows) {
		_ = tx.Rollback()
		return PlayerState{}, ErrIdentityNotFound
	}
	if err != nil {
		_ = tx.Rollback()
		return PlayerState{}, fmt.Errorf("get player AP for craft: %w", err)
	}
	now := s.now().UTC()
	if calculateAP(unixSeconds(fullTimestamp), now) < recipe.BaseAPCost {
		_ = tx.Rollback()
		return PlayerState{}, ErrInsufficientAP
	}
	for _, input := range recipe.ResourceInputs {
		var quantity int
		err := tx.QueryRow(`SELECT quantity FROM player_resources WHERE user_id = ? AND resource_id = ?`, userID, input.Resource.ID).Scan(&quantity)
		if errors.Is(err, sql.ErrNoRows) || quantity < input.Quantity {
			_ = tx.Rollback()
			return PlayerState{}, ErrInsufficientResource
		}
		if err != nil {
			_ = tx.Rollback()
			return PlayerState{}, fmt.Errorf("get crafting resource: %w", err)
		}
	}
	itemQuantities := make(map[string]int, len(recipe.ItemInputs))
	for _, input := range recipe.ItemInputs {
		var quantity int
		err := tx.QueryRow(`SELECT quantity FROM player_inventory WHERE user_id = ? AND item_id = ?`, userID, input.Item.ID).Scan(&quantity)
		if errors.Is(err, sql.ErrNoRows) || quantity < input.Quantity {
			_ = tx.Rollback()
			return PlayerState{}, ErrInsufficientItem
		}
		if err != nil {
			_ = tx.Rollback()
			return PlayerState{}, fmt.Errorf("get crafting item: %w", err)
		}
		itemQuantities[input.Item.ID] = quantity
	}
	fullAt := unixSeconds(fullTimestamp)
	if fullAt.Before(now) {
		fullAt = now
	}
	result, err := tx.Exec(`UPDATE player_ap SET full_timestamp = ? WHERE user_id = ? AND full_timestamp = ?`, fullAt.Add(time.Duration(recipe.BaseAPCost)*apRecoveryTime).Unix(), userID, fullTimestamp)
	if err != nil {
		_ = tx.Rollback()
		return PlayerState{}, fmt.Errorf("consume AP for craft: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil || rows != 1 {
		_ = tx.Rollback()
		if err != nil {
			return PlayerState{}, fmt.Errorf("check craft AP: %w", err)
		}
		return PlayerState{}, ErrInsufficientAP
	}
	for _, input := range recipe.ResourceInputs {
		result, err := tx.Exec(`UPDATE player_resources SET quantity = quantity - ? WHERE user_id = ? AND resource_id = ?`, input.Quantity, userID, input.Resource.ID)
		if err != nil {
			_ = tx.Rollback()
			return PlayerState{}, fmt.Errorf("consume crafting resource: %w", err)
		}
		if rows, err := result.RowsAffected(); err != nil || rows != 1 {
			_ = tx.Rollback()
			if err != nil {
				return PlayerState{}, fmt.Errorf("check crafting resource: %w", err)
			}
			return PlayerState{}, ErrInsufficientResource
		}
	}
	for _, input := range recipe.ItemInputs {
		var result sql.Result
		if itemQuantities[input.Item.ID] == input.Quantity {
			result, err = tx.Exec(`DELETE FROM player_inventory WHERE user_id = ? AND item_id = ?`, userID, input.Item.ID)
		} else {
			result, err = tx.Exec(`UPDATE player_inventory SET quantity = quantity - ? WHERE user_id = ? AND item_id = ?`, input.Quantity, userID, input.Item.ID)
		}
		if err != nil {
			_ = tx.Rollback()
			return PlayerState{}, fmt.Errorf("consume crafting item: %w", err)
		}
		if rows, err := result.RowsAffected(); err != nil || rows != 1 {
			_ = tx.Rollback()
			if err != nil {
				return PlayerState{}, fmt.Errorf("check crafting item: %w", err)
			}
			return PlayerState{}, ErrInsufficientItem
		}
	}
	if _, err := tx.Exec(`DELETE FROM player_resources WHERE user_id = ? AND quantity = 0`, userID); err != nil {
		_ = tx.Rollback()
		return PlayerState{}, fmt.Errorf("delete empty crafting resources: %w", err)
	}
	if _, err := tx.Exec(`DELETE FROM player_inventory WHERE user_id = ? AND quantity = 0`, userID); err != nil {
		_ = tx.Rollback()
		return PlayerState{}, fmt.Errorf("delete empty crafting items: %w", err)
	}
	if _, err := tx.Exec(`INSERT INTO player_inventory (user_id, item_id, quantity) VALUES (?, ?, ?) ON CONFLICT (user_id, item_id) DO UPDATE SET quantity = player_inventory.quantity + excluded.quantity`, userID, recipe.Output.ID, recipe.OutputQuantity); err != nil {
		_ = tx.Rollback()
		return PlayerState{}, fmt.Errorf("add crafted item: %w", err)
	}
	state, err := s.getPlayerStateTx(tx, userID, now)
	if err != nil {
		_ = tx.Rollback()
		return PlayerState{}, err
	}
	if err := tx.Commit(); err != nil {
		return PlayerState{}, fmt.Errorf("commit craft: %w", err)
	}
	return state, nil
}

func (s *Store) Build(userID int64, recipeID string) (PlayerState, error) {
	if userID <= 0 || strings.TrimSpace(recipeID) == "" {
		return PlayerState{}, fmt.Errorf("%w: user ID and recipe ID are required", ErrInvalidArgument)
	}
	tx, err := s.db.Begin()
	if err != nil {
		return PlayerState{}, fmt.Errorf("begin building: %w", err)
	}
	now := s.now().UTC()
	if err := deleteDestroyedBuildingsTx(tx, now); err != nil {
		_ = tx.Rollback()
		return PlayerState{}, err
	}
	recipe, err := buildingRecipeForID(tx, recipeID)
	if errors.Is(err, sql.ErrNoRows) {
		_ = tx.Rollback()
		return PlayerState{}, ErrBuildingNotFound
	}
	if err != nil {
		_ = tx.Rollback()
		return PlayerState{}, fmt.Errorf("get building recipe: %w", err)
	}
	var locationID string
	err = tx.QueryRow(`SELECT location_id FROM player_locations WHERE user_id = ?`, userID).Scan(&locationID)
	if errors.Is(err, sql.ErrNoRows) {
		_ = tx.Rollback()
		return PlayerState{}, ErrIdentityNotFound
	}
	if err != nil {
		_ = tx.Rollback()
		return PlayerState{}, fmt.Errorf("get building location: %w", err)
	}
	var occupied int
	err = tx.QueryRow(`SELECT 1 FROM buildings WHERE owner_id = ? AND location_id = ?`, userID, locationID).Scan(&occupied)
	if err == nil {
		_ = tx.Rollback()
		return PlayerState{}, ErrBuildingOccupied
	}
	if !errors.Is(err, sql.ErrNoRows) {
		_ = tx.Rollback()
		return PlayerState{}, fmt.Errorf("check building location: %w", err)
	}
	for _, input := range recipe.ResourceInputs {
		var quantity int
		err := tx.QueryRow(`SELECT quantity FROM player_resources WHERE user_id = ? AND resource_id = ?`, userID, input.Resource.ID).Scan(&quantity)
		if errors.Is(err, sql.ErrNoRows) || quantity < input.Quantity {
			_ = tx.Rollback()
			return PlayerState{}, ErrInsufficientResource
		}
		if err != nil {
			_ = tx.Rollback()
			return PlayerState{}, fmt.Errorf("get building resource: %w", err)
		}
	}
	itemQuantities := make(map[string]int, len(recipe.ItemInputs))
	for _, input := range recipe.ItemInputs {
		var quantity int
		err := tx.QueryRow(`SELECT quantity FROM player_inventory WHERE user_id = ? AND item_id = ?`, userID, input.Item.ID).Scan(&quantity)
		if errors.Is(err, sql.ErrNoRows) || quantity < input.Quantity {
			_ = tx.Rollback()
			return PlayerState{}, ErrInsufficientItem
		}
		if err != nil {
			_ = tx.Rollback()
			return PlayerState{}, fmt.Errorf("get building item: %w", err)
		}
		itemQuantities[input.Item.ID] = quantity
	}
	for _, input := range recipe.ResourceInputs {
		result, err := tx.Exec(`UPDATE player_resources SET quantity = quantity - ? WHERE user_id = ? AND resource_id = ?`, input.Quantity, userID, input.Resource.ID)
		if err != nil {
			_ = tx.Rollback()
			return PlayerState{}, fmt.Errorf("consume building resource: %w", err)
		}
		if rows, err := result.RowsAffected(); err != nil || rows != 1 {
			_ = tx.Rollback()
			if err != nil {
				return PlayerState{}, fmt.Errorf("check building resource: %w", err)
			}
			return PlayerState{}, ErrInsufficientResource
		}
	}
	for _, input := range recipe.ItemInputs {
		var result sql.Result
		if itemQuantities[input.Item.ID] == input.Quantity {
			result, err = tx.Exec(`DELETE FROM player_inventory WHERE user_id = ? AND item_id = ?`, userID, input.Item.ID)
		} else {
			result, err = tx.Exec(`UPDATE player_inventory SET quantity = quantity - ? WHERE user_id = ? AND item_id = ?`, input.Quantity, userID, input.Item.ID)
		}
		if err != nil {
			_ = tx.Rollback()
			return PlayerState{}, fmt.Errorf("consume building item: %w", err)
		}
		if rows, err := result.RowsAffected(); err != nil || rows != 1 {
			_ = tx.Rollback()
			if err != nil {
				return PlayerState{}, fmt.Errorf("check building item: %w", err)
			}
			return PlayerState{}, ErrInsufficientItem
		}
	}
	if _, err := tx.Exec(`DELETE FROM player_resources WHERE user_id = ? AND quantity = 0`, userID); err != nil {
		_ = tx.Rollback()
		return PlayerState{}, fmt.Errorf("delete empty building resources: %w", err)
	}
	if _, err := tx.Exec(`
	INSERT INTO buildings (owner_id, location_id, recipe_id, display_name, building_level, required_ap, contributed_ap, status, extension_slot_count, max_durability_seconds)
	VALUES (?, ?, ?, ?, ?, ?, 0, 'under_construction', ?, ?)`, userID, locationID, recipe.ID, recipe.DisplayName, recipe.BuildingLevel, recipe.RequiredAP, recipe.ExtensionSlotCount, recipe.MaxDurabilitySeconds); err != nil {
		_ = tx.Rollback()
		if strings.Contains(err.Error(), "UNIQUE constraint failed: buildings.owner_id, buildings.location_id") {
			return PlayerState{}, ErrBuildingOccupied
		}
		return PlayerState{}, fmt.Errorf("create building: %w", err)
	}
	state, err := s.getPlayerStateTx(tx, userID, now)
	if err != nil {
		_ = tx.Rollback()
		return PlayerState{}, err
	}
	if err := tx.Commit(); err != nil {
		return PlayerState{}, fmt.Errorf("commit building: %w", err)
	}
	return state, nil
}

func (s *Store) ContributeConstruction(userID, buildingID int64, requestedAP int) (PlayerState, error) {
	if userID <= 0 || buildingID <= 0 || requestedAP <= 0 {
		return PlayerState{}, fmt.Errorf("%w: user ID, building ID, and AP are required", ErrInvalidArgument)
	}
	tx, err := s.db.Begin()
	if err != nil {
		return PlayerState{}, fmt.Errorf("begin construction contribution: %w", err)
	}
	now := s.now().UTC()
	if err := deleteDestroyedBuildingsTx(tx, now); err != nil {
		_ = tx.Rollback()
		return PlayerState{}, err
	}
	var contributorLocation, buildingLocation, status string
	var contributedAP, requiredAP, maxDurabilitySeconds int
	err = tx.QueryRow(`SELECT location_id FROM player_locations WHERE user_id = ?`, userID).Scan(&contributorLocation)
	if errors.Is(err, sql.ErrNoRows) {
		_ = tx.Rollback()
		return PlayerState{}, ErrIdentityNotFound
	}
	if err != nil {
		_ = tx.Rollback()
		return PlayerState{}, fmt.Errorf("get contributor location: %w", err)
	}
	err = tx.QueryRow(`
SELECT location_id, contributed_ap, required_ap, status, max_durability_seconds
FROM buildings WHERE id = ?`, buildingID).Scan(&buildingLocation, &contributedAP, &requiredAP, &status, &maxDurabilitySeconds)
	if errors.Is(err, sql.ErrNoRows) {
		_ = tx.Rollback()
		return PlayerState{}, ErrBuildingNotFound
	}
	if err != nil {
		_ = tx.Rollback()
		return PlayerState{}, fmt.Errorf("get construction target: %w", err)
	}
	if contributorLocation != buildingLocation {
		_ = tx.Rollback()
		return PlayerState{}, ErrBuildingRemote
	}
	if status == "completed" || contributedAP >= requiredAP {
		_ = tx.Rollback()
		return PlayerState{}, ErrBuildingCompleted
	}

	var fullTimestamp int64
	err = tx.QueryRow(`SELECT full_timestamp FROM player_ap WHERE user_id = ?`, userID).Scan(&fullTimestamp)
	if errors.Is(err, sql.ErrNoRows) {
		_ = tx.Rollback()
		return PlayerState{}, ErrIdentityNotFound
	}
	if err != nil {
		_ = tx.Rollback()
		return PlayerState{}, fmt.Errorf("get contributor AP: %w", err)
	}
	availableAP := calculateAP(unixSeconds(fullTimestamp), now)
	remainingAP := requiredAP - contributedAP
	actualAP := requestedAP
	if actualAP > remainingAP {
		actualAP = remainingAP
	}
	if availableAP < actualAP {
		_ = tx.Rollback()
		return PlayerState{}, ErrInsufficientAP
	}
	fullAt := unixSeconds(fullTimestamp)
	if fullAt.Before(now) {
		fullAt = now
	}
	result, err := tx.Exec(`
UPDATE player_ap SET full_timestamp = ?
WHERE user_id = ? AND full_timestamp = ?`, fullAt.Add(time.Duration(actualAP)*apRecoveryTime).Unix(), userID, fullTimestamp)
	if err != nil {
		_ = tx.Rollback()
		return PlayerState{}, fmt.Errorf("consume construction AP: %w", err)
	}
	if rows, err := result.RowsAffected(); err != nil || rows != 1 {
		_ = tx.Rollback()
		if err != nil {
			return PlayerState{}, fmt.Errorf("check construction AP: %w", err)
		}
		return PlayerState{}, ErrInsufficientAP
	}
	newProgress := contributedAP + actualAP
	newStatus := "under_construction"
	if newProgress == requiredAP {
		newStatus = "completed"
	}
	if newStatus == "completed" {
		result, err = tx.Exec(`
			UPDATE buildings SET contributed_ap = ?, status = ?, durability_expires_at = ?
WHERE id = ? AND status = 'under_construction' AND contributed_ap = ?`, newProgress, newStatus, now.Unix()+int64(maxDurabilitySeconds), buildingID, contributedAP)
	} else {
		result, err = tx.Exec(`
UPDATE buildings SET contributed_ap = ?, status = ?
WHERE id = ? AND status = 'under_construction' AND contributed_ap = ?`, newProgress, newStatus, buildingID, contributedAP)
	}
	if err != nil {
		_ = tx.Rollback()
		return PlayerState{}, fmt.Errorf("update construction progress: %w", err)
	}
	if rows, err := result.RowsAffected(); err != nil || rows != 1 {
		_ = tx.Rollback()
		if err != nil {
			return PlayerState{}, fmt.Errorf("check construction progress: %w", err)
		}
		return PlayerState{}, ErrBuildingCompleted
	}
	state, err := s.getPlayerStateTx(tx, userID, now)
	if err != nil {
		_ = tx.Rollback()
		return PlayerState{}, err
	}
	if err := tx.Commit(); err != nil {
		return PlayerState{}, fmt.Errorf("commit construction contribution: %w", err)
	}
	state.ConstructionComputation = &ConstructionComputation{
		BuildingID:        buildingID,
		EffectiveAP:       actualAP,
		ResultingProgress: newProgress,
		RequiredAP:        requiredAP,
		CompletionOutcome: newStatus,
	}
	return state, nil
}

func (s *Store) RepairBuilding(userID, buildingID int64) (PlayerState, error) {
	if userID <= 0 || buildingID <= 0 {
		return PlayerState{}, fmt.Errorf("%w: user ID and building ID are required", ErrInvalidArgument)
	}
	tx, err := s.db.Begin()
	if err != nil {
		return PlayerState{}, fmt.Errorf("begin building repair: %w", err)
	}
	now := s.now().UTC()
	if err := deleteDestroyedBuildingsTx(tx, now); err != nil {
		_ = tx.Rollback()
		return PlayerState{}, err
	}
	var playerLocation string
	if err := tx.QueryRow(`SELECT location_id FROM player_locations WHERE user_id = ?`, userID).Scan(&playerLocation); errors.Is(err, sql.ErrNoRows) {
		_ = tx.Rollback()
		return PlayerState{}, ErrIdentityNotFound
	} else if err != nil {
		_ = tx.Rollback()
		return PlayerState{}, fmt.Errorf("get repair player location: %w", err)
	}
	var buildingLocation, status string
	var maxDurabilitySeconds int64
	var durabilityExpiresAt sql.NullInt64
	err = tx.QueryRow(`
SELECT location_id, status, max_durability_seconds, durability_expires_at
FROM buildings WHERE id = ?`, buildingID).Scan(&buildingLocation, &status, &maxDurabilitySeconds, &durabilityExpiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		_ = tx.Rollback()
		return PlayerState{}, ErrBuildingNotFound
	}
	if err != nil {
		_ = tx.Rollback()
		return PlayerState{}, fmt.Errorf("get repair target: %w", err)
	}
	if playerLocation != buildingLocation {
		_ = tx.Rollback()
		return PlayerState{}, ErrBuildingRemote
	}
	if status != "completed" || maxDurabilitySeconds <= 0 || !durabilityExpiresAt.Valid {
		_ = tx.Rollback()
		return PlayerState{}, ErrBuildingUnderConstruction
	}
	nowSeconds := now.Unix()
	priorStatus := "disabled"
	if durabilityExpiresAt.Int64 > nowSeconds {
		priorStatus = "active"
	}
	var fullTimestamp int64
	if err := tx.QueryRow(`SELECT full_timestamp FROM player_ap WHERE user_id = ?`, userID).Scan(&fullTimestamp); errors.Is(err, sql.ErrNoRows) {
		_ = tx.Rollback()
		return PlayerState{}, ErrIdentityNotFound
	} else if err != nil {
		_ = tx.Rollback()
		return PlayerState{}, fmt.Errorf("get repair player AP: %w", err)
	}
	if calculateAP(unixSeconds(fullTimestamp), now) < buildingRepairAPCost {
		_ = tx.Rollback()
		return PlayerState{}, ErrInsufficientAP
	}
	var woodQuantity int
	err = tx.QueryRow(`SELECT quantity FROM player_resources WHERE user_id = ? AND resource_id = 'wood'`, userID).Scan(&woodQuantity)
	if errors.Is(err, sql.ErrNoRows) || woodQuantity < buildingRepairWoodCost {
		_ = tx.Rollback()
		return PlayerState{}, ErrInsufficientResource
	}
	if err != nil {
		_ = tx.Rollback()
		return PlayerState{}, fmt.Errorf("get repair wood: %w", err)
	}
	fullAt := unixSeconds(fullTimestamp)
	if fullAt.Before(now) {
		fullAt = now
	}
	result, err := tx.Exec(`
UPDATE player_ap SET full_timestamp = ?
WHERE user_id = ? AND full_timestamp = ?`, fullAt.Add(time.Duration(buildingRepairAPCost)*apRecoveryTime).Unix(), userID, fullTimestamp)
	if err != nil {
		_ = tx.Rollback()
		return PlayerState{}, fmt.Errorf("consume repair AP: %w", err)
	}
	if rows, rowsErr := result.RowsAffected(); rowsErr != nil || rows != 1 {
		_ = tx.Rollback()
		if rowsErr != nil {
			return PlayerState{}, fmt.Errorf("check repair AP: %w", rowsErr)
		}
		return PlayerState{}, ErrInsufficientAP
	}
	result, err = tx.Exec(`
UPDATE player_resources SET quantity = quantity - ?
WHERE user_id = ? AND resource_id = 'wood' AND quantity >= ?`, buildingRepairWoodCost, userID, buildingRepairWoodCost)
	if err != nil {
		_ = tx.Rollback()
		return PlayerState{}, fmt.Errorf("consume repair wood: %w", err)
	}
	if rows, rowsErr := result.RowsAffected(); rowsErr != nil || rows != 1 {
		_ = tx.Rollback()
		if rowsErr != nil {
			return PlayerState{}, fmt.Errorf("check repair wood: %w", rowsErr)
		}
		return PlayerState{}, ErrInsufficientResource
	}
	newExpiry := durabilityExpiresAt.Int64
	if newExpiry <= nowSeconds {
		newExpiry = nowSeconds
	}
	newExpiry += int64(buildingRepairDuration / time.Second)
	maximumExpiry := nowSeconds + maxDurabilitySeconds
	if newExpiry > maximumExpiry {
		newExpiry = maximumExpiry
	}
	result, err = tx.Exec(`
UPDATE buildings SET durability_expires_at = ?
WHERE id = ? AND status = 'completed' AND durability_expires_at = ?`, newExpiry, buildingID, durabilityExpiresAt.Int64)
	if err != nil {
		_ = tx.Rollback()
		return PlayerState{}, fmt.Errorf("update building durability: %w", err)
	}
	if rows, rowsErr := result.RowsAffected(); rowsErr != nil || rows != 1 {
		_ = tx.Rollback()
		if rowsErr != nil {
			return PlayerState{}, fmt.Errorf("check building durability: %w", rowsErr)
		}
		return PlayerState{}, ErrBuildingNotFound
	}
	if _, err := tx.Exec(`DELETE FROM player_resources WHERE user_id = ? AND resource_id = 'wood' AND quantity = 0`, userID); err != nil {
		_ = tx.Rollback()
		return PlayerState{}, fmt.Errorf("delete empty repair wood: %w", err)
	}
	state, err := s.getPlayerStateTx(tx, userID, now)
	if err != nil {
		_ = tx.Rollback()
		return PlayerState{}, err
	}
	if err := tx.Commit(); err != nil {
		return PlayerState{}, fmt.Errorf("commit building repair: %w", err)
	}
	state.RepairComputation = &RepairComputation{
		BuildingID:                buildingID,
		PriorDurabilityStatus:     priorStatus,
		AddedSeconds:              int(newExpiry - maxInt64(durabilityExpiresAt.Int64, nowSeconds)),
		ResultingRemainingSeconds: int(newExpiry - nowSeconds),
		APCost:                    buildingRepairAPCost,
		WoodCost:                  buildingRepairWoodCost,
	}
	return state, nil
}

func maxInt64(left, right int64) int64 {
	if left > right {
		return left
	}
	return right
}

func buildingRecipeForID(tx *sql.Tx, recipeID string) (BuildingRecipe, error) {
	var recipe BuildingRecipe
	err := tx.QueryRow(`
SELECT id, display_name, building_level, required_ap, extension_slot_count, max_durability_seconds
FROM building_recipes WHERE id = ?`, recipeID).Scan(&recipe.ID, &recipe.DisplayName, &recipe.BuildingLevel, &recipe.RequiredAP, &recipe.ExtensionSlotCount, &recipe.MaxDurabilitySeconds)
	if err != nil {
		return BuildingRecipe{}, err
	}
	if err := loadBuildingInputsTx(tx, &recipe); err != nil {
		return BuildingRecipe{}, err
	}
	if len(recipe.ResourceInputs) == 0 && len(recipe.ItemInputs) == 0 {
		return BuildingRecipe{}, sql.ErrNoRows
	}
	return recipe, nil
}

func calculateAP(fullTimestamp, now time.Time) int {
	remaining := fullTimestamp.Sub(now)
	if remaining <= 0 {
		return maxAP
	}
	missing := remaining / apRecoveryTime
	if remaining%apRecoveryTime != 0 {
		missing++
	}
	if missing >= maxAP {
		return 0
	}
	return maxAP - int(missing)
}

func (s *Store) GetIdentity(id int64) (Identity, error) {
	var identity Identity
	var createdAt, updatedAt int64
	err := s.db.QueryRow(`
SELECT id, issuer, subject, email, display_name, created_at, updated_at
FROM identities WHERE id = ?`, id).Scan(
		&identity.ID, &identity.Issuer, &identity.Subject, &identity.Email,
		&identity.DisplayName, &createdAt, &updatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Identity{}, ErrIdentityNotFound
	}
	if err != nil {
		return Identity{}, fmt.Errorf("get identity: %w", err)
	}
	identity.CreatedAt = unixSeconds(createdAt)
	identity.UpdatedAt = unixSeconds(updatedAt)
	return identity, nil
}

func (s *Store) CreateOAuthAttempt(state, nonce, verifier string, expiresAt time.Time, browserToken ...string) error {
	if strings.TrimSpace(state) == "" || strings.TrimSpace(nonce) == "" || strings.TrimSpace(verifier) == "" {
		return fmt.Errorf("%w: oauth attempt values are required", ErrInvalidArgument)
	}
	var browserHash any
	if len(browserToken) > 0 && strings.TrimSpace(browserToken[0]) != "" {
		browserHash = hashSecret(browserToken[0])
	}
	_, err := s.db.Exec(`
INSERT INTO oauth_attempts (state_hash, browser_token_hash, nonce, verifier, expires_at)
VALUES (?, ?, ?, ?, ?)`, hashSecret(state), browserHash, nonce, verifier, expiresAt.UTC().Unix())
	if err != nil {
		return fmt.Errorf("create oauth attempt: %w", err)
	}
	return nil
}

func (s *Store) ConsumeOAuthAttempt(state string, browserToken ...string) (OAuthAttempt, error) {
	if strings.TrimSpace(state) == "" {
		return OAuthAttempt{}, fmt.Errorf("%w: state is required", ErrInvalidArgument)
	}
	now := s.now().UTC()
	nowSeconds := now.Unix()
	var browserHash any
	bound := len(browserToken) > 0
	boundFlag := 0
	if bound {
		browserHash = hashSecret(browserToken[0])
		boundFlag = 1
	}
	tx, err := s.db.Begin()
	if err != nil {
		return OAuthAttempt{}, fmt.Errorf("begin oauth attempt consumption: %w", err)
	}
	var attempt OAuthAttempt
	var expiresAt int64
	err = tx.QueryRow(`
	SELECT nonce, verifier, expires_at FROM oauth_attempts
	WHERE state_hash = ? AND consumed_at IS NULL AND expires_at > ?
		AND (? = 0 OR browser_token_hash = ?)`, hashSecret(state), nowSeconds, boundFlag, browserHash).Scan(
		&attempt.Nonce, &attempt.Verifier, &expiresAt,
	)
	if err == nil {
		result, updateErr := tx.Exec(`
UPDATE oauth_attempts
SET nonce = '', verifier = '', consumed_at = ?
WHERE state_hash = ? AND consumed_at IS NULL AND expires_at > ?
			AND (? = 0 OR browser_token_hash = ?)`, nowSeconds, hashSecret(state), nowSeconds, boundFlag, browserHash)
		if updateErr != nil {
			_ = tx.Rollback()
			return OAuthAttempt{}, fmt.Errorf("consume oauth attempt: %w", updateErr)
		}
		rows, rowsErr := result.RowsAffected()
		if rowsErr != nil {
			_ = tx.Rollback()
			return OAuthAttempt{}, fmt.Errorf("check oauth attempt consumption: %w", rowsErr)
		}
		if rows != 1 {
			_ = tx.Rollback()
			return OAuthAttempt{}, ErrOAuthAttemptConsumed
		}
		if err = tx.Commit(); err != nil {
			return OAuthAttempt{}, fmt.Errorf("commit oauth attempt consumption: %w", err)
		}
		attempt.ExpiresAt = unixSeconds(expiresAt)
		return attempt, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		_ = tx.Rollback()
		return OAuthAttempt{}, fmt.Errorf("consume oauth attempt: %w", err)
	}
	var consumedAt sql.NullInt64
	err = tx.QueryRow(`
SELECT consumed_at, expires_at FROM oauth_attempts WHERE state_hash = ?`, hashSecret(state)).Scan(&consumedAt, &expiresAt)
	_ = tx.Rollback()
	if errors.Is(err, sql.ErrNoRows) {
		return OAuthAttempt{}, ErrOAuthAttemptNotFound
	}
	if err != nil {
		return OAuthAttempt{}, fmt.Errorf("inspect oauth attempt: %w", err)
	}
	if consumedAt.Valid {
		return OAuthAttempt{}, ErrOAuthAttemptConsumed
	}
	if expiresAt <= nowSeconds {
		return OAuthAttempt{}, ErrOAuthAttemptExpired
	}
	return OAuthAttempt{}, ErrOAuthAttemptNotFound
}

func (s *Store) CreateSession(userID int64, token string, expiresAt time.Time) error {
	if userID <= 0 || strings.TrimSpace(token) == "" {
		return fmt.Errorf("%w: user ID and token are required", ErrInvalidArgument)
	}
	_, err := s.db.Exec(`
INSERT INTO sessions (token_hash, user_id, expires_at, created_at)
VALUES (?, ?, ?, ?)`, hashSecret(token), userID, expiresAt.UTC().Unix(), s.now().UTC().Unix())
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}
	return nil
}

func (s *Store) GetSession(token string) (Session, error) {
	if strings.TrimSpace(token) == "" {
		return Session{}, fmt.Errorf("%w: token is required", ErrInvalidArgument)
	}
	var session Session
	var expiresAt int64
	err := s.db.QueryRow(`
SELECT user_id, expires_at FROM sessions WHERE token_hash = ?`, hashSecret(token)).Scan(&session.UserID, &expiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Session{}, ErrSessionNotFound
	}
	if err != nil {
		return Session{}, fmt.Errorf("get session: %w", err)
	}
	session.ExpiresAt = unixSeconds(expiresAt)
	if expiresAt <= s.now().UTC().Unix() {
		return Session{}, ErrSessionExpired
	}
	return session, nil
}

func (s *Store) GetIdentityForSession(token string) (Identity, error) {
	session, err := s.GetSession(token)
	if err != nil {
		return Identity{}, err
	}
	return s.GetIdentity(session.UserID)
}

func hashSecret(value string) []byte {
	digest := sha256.Sum256([]byte(value))
	encoded := make([]byte, hex.EncodedLen(len(digest)))
	hex.Encode(encoded, digest[:])
	return encoded
}

func unixSeconds(value int64) time.Time {
	return time.Unix(value, 0).UTC()
}
