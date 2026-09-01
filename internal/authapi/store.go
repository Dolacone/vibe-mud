package authapi

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"math/bits"
	"math/rand"
	"os"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
	_ "modernc.org/sqlite"
)

var (
	ErrInvalidArgument             = errors.New("invalid argument")
	ErrIdentityNotFound            = errors.New("identity not found")
	ErrOAuthAttemptNotFound        = errors.New("oauth attempt not found")
	ErrOAuthAttemptExpired         = errors.New("oauth attempt expired")
	ErrOAuthAttemptConsumed        = errors.New("oauth attempt already consumed")
	ErrSessionNotFound             = errors.New("session not found")
	ErrSessionExpired              = errors.New("session expired")
	ErrInsufficientAP              = errors.New("insufficient action points")
	ErrNoMonster                   = errors.New("no monsters available")
	ErrNoMonsters                  = ErrNoMonster
	ErrOverweight                  = errors.New("player is overweight")
	ErrRouteNotFound               = errors.New("route not found")
	ErrGatheringNotFound           = errors.New("gathering not found")
	ErrConversionNotFound          = errors.New("conversion not found")
	ErrInsufficientItem            = errors.New("insufficient item")
	ErrCraftingNotFound            = errors.New("crafting recipe not found")
	ErrInsufficientResource        = errors.New("insufficient resource")
	ErrBuildingNotFound            = errors.New("building not found")
	ErrBuildingOccupied            = errors.New("building location already occupied")
	ErrBuildingRemote              = errors.New("building is at another location")
	ErrBuildingCompleted           = errors.New("building is already completed")
	ErrBuildingUnderConstruction   = errors.New("building is under construction")
	ErrBuildingDisabled            = errors.New("building is disabled")
	ErrBuildingNotOwner            = errors.New("building owner required")
	ErrExtensionNotFound           = errors.New("extension not found")
	ErrExtensionDefinitionNotFound = errors.New("extension definition not found")
	ErrExtensionOccupied           = errors.New("extension slot already occupied")
	ErrExtensionCompleted          = errors.New("extension is already completed")
	ErrTransferAssetNotFound       = errors.New("transfer asset not found")
	ErrInsufficientTransferAsset   = errors.New("insufficient transfer asset")
	ErrResourceDropNotAllowed      = errors.New("resource drop is not allowed")
	ErrInvalidPlayerName           = errors.New("invalid player name")
	ErrPlayerNameUnavailable       = errors.New("player name unavailable")
)

type Store struct {
	db          *sql.DB
	now         func() time.Time
	essenceRoll func() int
	monsterRoll func() int
	combatRoll  func() int
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

type PlayerProfile struct {
	UserID         int64
	PlayerName     *string
	NormalizedName *string
	UpdatedAt      time.Time
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

type PlayerCombatDefinition struct {
	ID                        int
	MaxHP                     int
	HPRecoveryIntervalSeconds int
	BaseAttackPower           int
}

type PlayerHP struct {
	UserID        int64
	HP            int
	FullTimestamp time.Time
}

type CombatEvent struct {
	Attacker          string
	Damage            int
	TargetRemainingHP int
}

type CombatMonster struct {
	TypeID      int64
	DisplayName string
}

type CombatDrop struct {
	Item     Item
	Quantity int
}

type CombatDropCalculation struct {
	ItemID    string
	ChanceBPS int
	Quantity  int
	Outcome   string
}

type CombatResult struct {
	Monster          CombatMonster
	Events           []CombatEvent
	Result           string
	Drops            []CombatDrop
	DropCalculations []CombatDropCalculation
}

type MonsterInterceptionComputation struct {
	LocationID          string
	MonsterCount        int
	PerMonsterChanceBPS int
	CombinedChanceBPS   int
	Outcome             string
}

type MonsterSettlementComputation struct {
	LocationID         string
	Intervals          uint64
	SpawnChanceBPS     int
	MonsterCountBefore int
	MonsterCountAfter  int
	Outcome            string
}

type Location struct {
	ID          string
	DisplayName string
}

type LocationMonsterState struct {
	LocationID   string
	MonsterCount int
	SettledAt    time.Time
	Computation  *MonsterSettlementComputation
}

type Route struct {
	OriginID      string
	DestinationID string
	APCost        int
}

type Item struct {
	ID                   string
	DisplayName          string
	WeightUnits          int
	MaxDurabilitySeconds int
}

type InventoryItem struct {
	Item                       Item
	Quantity                   int
	DurabilityStatus           string
	DurabilityRemainingSeconds *int
	RetentionRemainingSeconds  *int
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

type ConversionMethod struct {
	ID                       string
	DisplayName              string
	APCost                   int
	Input                    Item
	MaxInputQuantity         int
	OutputResource           ResourceType
	ResourceQuantityPerInput int
	EssenceItem              *Item
	EssenceChanceBPS         int
	EssenceQuantity          int
	IsGlobal                 bool
	ProviderDefinitionIDs    []string
}

type BuildingExtensionDefinition struct {
	ID          string
	DisplayName string
	Tier        int
	PackageItem Item
	RequiredAP  int
}

type ExtensionConversionCapability struct {
	ExtensionDefinitionID         string
	ConversionMethodID            string
	BuildingDurabilityCostSeconds int
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
	Item                       Item
	Quantity                   int
	DurabilityStatus           string
	DurabilityRemainingSeconds *int
	RetentionRemainingSeconds  *int
}

type GroundResource struct {
	Resource ResourceType
	Quantity int
}

type ItemDurabilityComputation struct {
	Holding                    string
	ItemID                     string
	Quantity                   int
	DurabilityStatus           string
	DurabilityRemainingSeconds *int
	RetentionRemainingSeconds  *int
}

type ItemDurabilityCleanup struct {
	Holding            string
	ItemID             string
	Quantity           int
	Action             string
	ExpiredAt          int64
	RetentionExpiresAt int64
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
	Extensions                 []BuildingExtension
}

type BuildingExtension struct {
	ID            int64
	SlotIndex     int
	DefinitionID  string
	DisplayName   string
	Tier          int
	RequiredAP    int
	ContributedAP int
	Status        string
}

type ConstructionComputation struct {
	BuildingID        int64
	EffectiveAP       int
	ResultingProgress int
	RequiredAP        int
	CompletionOutcome string
}

type ExtensionConstructionComputation struct {
	BuildingID        int64
	ExtensionID       int64
	RequestedAP       int
	EffectiveAP       int
	ResultingProgress int
	RequiredAP        int
	ResultingStatus   string
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
	Location                         Location
	MonsterCount                     int
	HP                               int
	AttackAPCost                     int `json:"-"`
	Routes                           []Route
	AP                               int
	Inventory                        []InventoryItem
	GroundItems                      []GroundItem
	GroundResources                  []GroundResource
	GatheringOption                  *GatheringOption
	ConversionOption                 *ConversionOption
	ConversionMethods                []ConversionMethod
	Resources                        []PlayerResource
	CraftingRecipes                  []CraftingRecipe
	BuildingRecipes                  []BuildingRecipe
	BuildingExtensionDefinitions     []BuildingExtensionDefinition
	Buildings                        []Building
	ConstructionComputation          *ConstructionComputation
	ExtensionConstructionComputation *ExtensionConstructionComputation
	RepairComputation                *RepairComputation
	CarriedWeight                    int
	MovementWeightThreshold          int
	ItemDurabilityComputations       []ItemDurabilityComputation     `json:"-"`
	ItemDurabilityCleanups           []ItemDurabilityCleanup         `json:"-"`
	MonsterSettlement                *MonsterSettlementComputation   `json:"-"`
	MonsterInterception              *MonsterInterceptionComputation `json:"-"`
}

const (
	maxAP                            = 3000
	apRecoveryTime                   = time.Minute
	defaultActiveAttackAPCost        = 30
	buildingDefaultDurability        = 7 * 24 * time.Hour
	buildingDefaultDurabilitySeconds = int64(buildingDefaultDurability / time.Second)
	buildingRepairDuration           = time.Hour
	buildingRepairAPCost             = 10
	buildingRepairWoodCost           = 1
	buildingDisabledRetention        = 7 * 24 * time.Hour
	buildingDisabledRetentionSeconds = int64(buildingDisabledRetention / time.Second)
	itemDefaultDurability            = time.Hour
	itemDefaultDurabilitySeconds     = int64(itemDefaultDurability / time.Second)
	itemExpiredRetention             = 24 * time.Hour
	itemExpiredRetentionSeconds      = int64(itemExpiredRetention / time.Second)
	movementWeightThreshold          = 1000
	unixNanosecondsThreshold         = int64(1_000_000_000_000_000)
	nanosecondsPerSecond             = int64(time.Second)
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
CREATE TABLE IF NOT EXISTS player_profiles (
	user_id INTEGER PRIMARY KEY REFERENCES identities(id),
	player_name TEXT,
	normalized_name TEXT UNIQUE,
	updated_at INTEGER NOT NULL,
	CHECK (
		(player_name IS NULL AND normalized_name IS NULL) OR
		(player_name IS NOT NULL AND normalized_name IS NOT NULL)
	)
);
CREATE TABLE IF NOT EXISTS player_ap (
	user_id INTEGER PRIMARY KEY REFERENCES identities(id),
	full_timestamp INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS player_combat_definitions (
	id INTEGER PRIMARY KEY CHECK (id > 0),
	max_hp INTEGER NOT NULL CHECK (max_hp > 0),
	hp_recovery_interval_seconds INTEGER NOT NULL CHECK (hp_recovery_interval_seconds > 0),
	base_attack_power INTEGER NOT NULL CHECK (base_attack_power > 0)
);
CREATE TABLE IF NOT EXISTS player_hp (
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
CREATE TABLE IF NOT EXISTS monster_types (
	id INTEGER PRIMARY KEY CHECK (id > 0),
	display_name TEXT NOT NULL,
	max_hp INTEGER NOT NULL CHECK (max_hp > 0),
	attack_power INTEGER NOT NULL CHECK (attack_power > 0)
);
CREATE TABLE IF NOT EXISTS monster_drop_rules (
	monster_type_id INTEGER NOT NULL REFERENCES monster_types(id),
	item_id TEXT NOT NULL REFERENCES items(id),
	chance_bps INTEGER NOT NULL CHECK (chance_bps BETWEEN 0 AND 10000),
	quantity INTEGER NOT NULL CHECK (quantity > 0),
	PRIMARY KEY (monster_type_id, item_id)
);
CREATE TABLE IF NOT EXISTS location_monster_rules (
	location_id TEXT PRIMARY KEY REFERENCES locations(id),
	spawn_interval_seconds INTEGER NOT NULL CHECK (spawn_interval_seconds > 0),
	spawn_chance_bps INTEGER NOT NULL CHECK (spawn_chance_bps BETWEEN 0 AND 10000),
	max_monsters INTEGER NOT NULL CHECK (max_monsters >= 0),
	intercept_chance_bps INTEGER NOT NULL CHECK (intercept_chance_bps BETWEEN 0 AND 10000)
);
CREATE TABLE IF NOT EXISTS location_monster_encounters (
	location_id TEXT NOT NULL REFERENCES locations(id),
	monster_type_id INTEGER NOT NULL REFERENCES monster_types(id),
	encounter_weight INTEGER NOT NULL CHECK (encounter_weight > 0),
	PRIMARY KEY (location_id, monster_type_id)
);
CREATE TABLE IF NOT EXISTS location_monster_populations (
	location_id TEXT PRIMARY KEY REFERENCES locations(id),
	monster_count INTEGER NOT NULL CHECK (monster_count >= 0),
	settled_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS items (
	id TEXT PRIMARY KEY,
	display_name TEXT NOT NULL,
	weight_units INTEGER NOT NULL CHECK (weight_units > 0),
	max_durability_seconds INTEGER NOT NULL DEFAULT 3600 CHECK (max_durability_seconds > 0)
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
	durability_status TEXT NOT NULL DEFAULT 'active' CHECK (durability_status IN ('active', 'expired')),
	status_expires_at INTEGER NOT NULL DEFAULT 0,
	quantity INTEGER NOT NULL CHECK (quantity > 0),
	PRIMARY KEY (user_id, item_id, durability_status)
);
CREATE TABLE IF NOT EXISTS resource_types (
	id TEXT PRIMARY KEY,
	display_name TEXT NOT NULL,
	weight_units INTEGER NOT NULL CHECK (weight_units > 0)
);
CREATE TABLE IF NOT EXISTS conversion_rules (
	location_id TEXT PRIMARY KEY REFERENCES locations(id),
	input_item_id TEXT NOT NULL REFERENCES items(id),
	input_quantity INTEGER NOT NULL CHECK (input_quantity > 0),
	output_resource_id TEXT NOT NULL REFERENCES resource_types(id),
	resource_yield INTEGER NOT NULL CHECK (resource_yield > 0),
	ap_cost INTEGER NOT NULL CHECK (ap_cost > 0)
);
CREATE TABLE IF NOT EXISTS conversion_methods (
	id TEXT PRIMARY KEY,
	display_name TEXT NOT NULL,
	ap_cost INTEGER NOT NULL CHECK (ap_cost > 0),
	input_item_id TEXT NOT NULL REFERENCES items(id),
	max_input_quantity INTEGER NOT NULL CHECK (max_input_quantity > 0),
	output_resource_id TEXT NOT NULL REFERENCES resource_types(id),
	resource_quantity_per_input INTEGER NOT NULL CHECK (resource_quantity_per_input > 0),
	essence_item_id TEXT REFERENCES items(id),
	essence_chance_bps INTEGER NOT NULL CHECK (essence_chance_bps BETWEEN 0 AND 10000),
	essence_quantity INTEGER NOT NULL CHECK (essence_quantity >= 0),
	CHECK ((essence_item_id IS NULL AND essence_chance_bps = 0 AND essence_quantity = 0) OR (essence_item_id IS NOT NULL AND essence_chance_bps > 0 AND essence_quantity > 0))
);
CREATE TABLE IF NOT EXISTS global_conversion_methods (
	conversion_method_id TEXT PRIMARY KEY REFERENCES conversion_methods(id)
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
	durability_status TEXT NOT NULL DEFAULT 'active' CHECK (durability_status IN ('active', 'expired')),
	status_expires_at INTEGER NOT NULL DEFAULT 0,
	quantity INTEGER NOT NULL CHECK (quantity > 0),
	PRIMARY KEY (location_id, item_id, durability_status)
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
CREATE TABLE IF NOT EXISTS building_extension_definitions (
	id TEXT PRIMARY KEY,
	display_name TEXT NOT NULL,
	tier INTEGER NOT NULL CHECK (tier > 0),
	package_item_id TEXT NOT NULL REFERENCES items(id),
	required_ap INTEGER NOT NULL CHECK (required_ap > 0)
);
CREATE TABLE IF NOT EXISTS extension_conversion_capabilities (
	extension_definition_id TEXT NOT NULL REFERENCES building_extension_definitions(id),
	conversion_method_id TEXT NOT NULL REFERENCES conversion_methods(id),
	building_durability_cost_seconds INTEGER NOT NULL CHECK (building_durability_cost_seconds > 0),
	PRIMARY KEY (extension_definition_id, conversion_method_id)
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
	if _, err := tx.Exec(`
CREATE TABLE IF NOT EXISTS building_extensions (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	building_id INTEGER NOT NULL REFERENCES buildings(id) ON DELETE CASCADE,
	slot_index INTEGER NOT NULL CHECK (slot_index >= 0),
	definition_id TEXT NOT NULL REFERENCES building_extension_definitions(id),
	display_name TEXT NOT NULL,
	tier INTEGER NOT NULL CHECK (tier > 0),
	required_ap INTEGER NOT NULL CHECK (required_ap > 0),
	contributed_ap INTEGER NOT NULL CHECK (contributed_ap >= 0 AND contributed_ap <= required_ap),
	status TEXT NOT NULL CHECK (status IN ('under_construction', 'completed')),
	UNIQUE (building_id, slot_index)
);`); err != nil {
		_ = tx.Rollback()
		return nil, fmt.Errorf("initialize building extensions: %w", err)
	}
	migrationNow := time.Now().UTC()
	migratedTimestampValues, err := migrateTimestampsToUnixSeconds(tx)
	if err != nil {
		_ = tx.Rollback()
		return nil, fmt.Errorf("migrate timestamps to Unix seconds: %w", err)
	}
	if err := ensureBuildingSchema(tx); err != nil {
		_ = tx.Rollback()
		return nil, fmt.Errorf("upgrade building schema: %w", err)
	}
	if err := ensureWeightSchema(tx); err != nil {
		_ = tx.Rollback()
		return nil, fmt.Errorf("upgrade weight schema: %w", err)
	}
	if err := ensureItemDurabilitySchema(tx, migrationNow); err != nil {
		_ = tx.Rollback()
		return nil, fmt.Errorf("upgrade item durability schema: %w", err)
	}
	if _, err := tx.Exec(`
INSERT OR IGNORE INTO resource_types (id, display_name, weight_units) VALUES
	('food', 'Food', 1),
	('wood', 'Wood', 1),
	('stone', 'Stone', 1),
	('metal', 'Metal', 1),
	('fiber', 'Fiber', 1),
	('hide', 'Hide', 1),
	('medicinal', 'Medicinal', 1),
	('arcane', 'Arcane', 1);`); err != nil {
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
INSERT OR IGNORE INTO items (id, display_name, weight_units) VALUES
	('wood', 'Wood', 100),
	('wood_component', 'Wood Component', 10);
INSERT OR IGNORE INTO items (id, display_name, weight_units, max_durability_seconds) VALUES
	('wood_essence_t1', 'Wood Essence T1', 1, 3600),
	('sawmill_package_t1', 'Sawmill Package T1', 10, 3600),
	('rat_tail', 'Rat Tail', 1, 3600);
INSERT OR IGNORE INTO gathering_rules (location_id, item_id, quantity, ap_cost) VALUES
	('forest_edge', 'wood', 1, 10);
INSERT OR IGNORE INTO conversion_rules (location_id, input_item_id, input_quantity, output_resource_id, resource_yield, ap_cost) VALUES
	('camp', 'wood', 1, 'wood', 1, 1);
INSERT OR IGNORE INTO conversion_methods (id, display_name, ap_cost, input_item_id, max_input_quantity, output_resource_id, resource_quantity_per_input, essence_item_id, essence_chance_bps, essence_quantity) VALUES
	('hand_wood_t1', 'Hand Wood Convert', 30, 'wood', 3, 'wood', 1, 'wood_essence_t1', 1000, 1),
	('sawmill_wood_t1', 'Sawmill Wood Convert', 30, 'wood', 6, 'wood', 1, 'wood_essence_t1', 1000, 1);
INSERT OR IGNORE INTO global_conversion_methods (conversion_method_id) VALUES
	('hand_wood_t1');
INSERT OR IGNORE INTO crafting_recipes (id, display_name, base_ap_cost, output_item_id, output_quantity) VALUES
	('wood_component', 'Wood Component', 10, 'wood_component', 1);
INSERT OR IGNORE INTO crafting_recipe_resource_inputs (recipe_id, resource_id, quantity) VALUES
	('wood_component', 'wood', 10);`); err != nil {
		_ = tx.Rollback()
		return nil, fmt.Errorf("seed movement state: %w", err)
	}
	if _, err := tx.Exec(`
INSERT OR IGNORE INTO player_combat_definitions (id, max_hp, hp_recovery_interval_seconds, base_attack_power)
VALUES (1, 100, 60, 3);
INSERT OR IGNORE INTO monster_types (id, display_name, max_hp, attack_power)
VALUES (1, 'Forest Rat', 10, 2);
INSERT OR IGNORE INTO monster_drop_rules (monster_type_id, item_id, chance_bps, quantity)
VALUES (1, 'rat_tail', 5000, 1);
INSERT OR IGNORE INTO location_monster_rules (location_id, spawn_interval_seconds, spawn_chance_bps, max_monsters, intercept_chance_bps) VALUES
	('camp', 1800, 0, 0, 0),
	('forest_edge', 1800, 5000, 10, 1000);
INSERT OR IGNORE INTO location_monster_encounters (location_id, monster_type_id, encounter_weight)
VALUES ('forest_edge', 1, 1);
INSERT OR IGNORE INTO location_monster_populations (location_id, monster_count, settled_at)
SELECT id, 0, ? FROM locations;`, migrationNow.Unix()); err != nil {
		_ = tx.Rollback()
		return nil, fmt.Errorf("seed monster state: %w", err)
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
INSERT OR IGNORE INTO crafting_recipes (id, display_name, base_ap_cost, output_item_id, output_quantity)
VALUES ('sawmill_package_t1', 'Sawmill Package T1', 30, 'sawmill_package_t1', 1);
INSERT OR IGNORE INTO crafting_recipe_resource_inputs (recipe_id, resource_id, quantity)
VALUES ('sawmill_package_t1', 'wood', 10);
INSERT OR IGNORE INTO crafting_recipe_item_inputs (recipe_id, item_id, quantity)
VALUES ('sawmill_package_t1', 'wood_essence_t1', 1);
INSERT OR IGNORE INTO building_extension_definitions (id, display_name, tier, package_item_id, required_ap)
VALUES ('sawmill_t1', 'Sawmill T1', 1, 'sawmill_package_t1', 30);
INSERT OR IGNORE INTO extension_conversion_capabilities (extension_definition_id, conversion_method_id, building_durability_cost_seconds)
VALUES ('sawmill_t1', 'sawmill_wood_t1', 60);`); err != nil {
		_ = tx.Rollback()
		return nil, fmt.Errorf("seed sawmill definitions: %w", err)
	}
	if _, err := tx.Exec(`
INSERT OR IGNORE INTO player_ap (user_id, full_timestamp)
SELECT id, ? FROM identities`, time.Now().UTC().Unix()); err != nil {
		_ = tx.Rollback()
		return nil, fmt.Errorf("backfill player AP: %w", err)
	}
	if _, err := tx.Exec(`
INSERT OR IGNORE INTO player_hp (user_id, full_timestamp)
SELECT id, ? FROM identities`, migrationNow.Unix()); err != nil {
		_ = tx.Rollback()
		return nil, fmt.Errorf("backfill player HP: %w", err)
	}
	if _, err := tx.Exec(`
INSERT OR IGNORE INTO player_locations (user_id, location_id)
SELECT id, 'camp' FROM identities`); err != nil {
		_ = tx.Rollback()
		return nil, fmt.Errorf("backfill player locations: %w", err)
	}
	if _, err := tx.Exec(`
INSERT OR IGNORE INTO player_profiles (user_id, updated_at)
SELECT id, ? FROM identities`, migrationNow.Unix()); err != nil {
		_ = tx.Rollback()
		return nil, fmt.Errorf("backfill player profiles: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit auth store initialization: %w", err)
	}
	fmt.Fprintf(os.Stdout, "user_id=anonymous action=timestamp_migration outcome=success converted_values=%d request_id=unavailable\n", migratedTimestampValues)
	return &Store{db: db, now: time.Now, essenceRoll: func() int { return int(time.Now().UnixNano() % 10000) }, monsterRoll: func() int { return rand.Intn(10000) }, combatRoll: func() int { return rand.Intn(10000) }}, nil
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
		{table: "player_profiles", column: "updated_at"},
		{table: "player_ap", column: "full_timestamp"},
		{table: "player_hp", column: "full_timestamp"},
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

func ensureWeightSchema(tx *sql.Tx) error {
	itemColumns, err := tableColumns(tx, "items")
	if err != nil {
		return err
	}
	itemWeightAdded := !itemColumns["weight_units"]
	if itemWeightAdded {
		if _, err := tx.Exec(`ALTER TABLE items ADD COLUMN weight_units INTEGER NOT NULL DEFAULT 1 CHECK (weight_units > 0)`); err != nil {
			return fmt.Errorf("add item weight: %w", err)
		}
	}
	resourceColumns, err := tableColumns(tx, "resource_types")
	if err != nil {
		return err
	}
	resourceWeightAdded := !resourceColumns["weight_units"]
	if resourceWeightAdded {
		if _, err := tx.Exec(`ALTER TABLE resource_types ADD COLUMN weight_units INTEGER NOT NULL DEFAULT 1 CHECK (weight_units > 0)`); err != nil {
			return fmt.Errorf("add resource weight: %w", err)
		}
	}
	if itemWeightAdded {
		if _, err := tx.Exec(`UPDATE items SET weight_units = CASE id WHEN 'wood' THEN 100 WHEN 'wood_component' THEN 10 ELSE weight_units END`); err != nil {
			return fmt.Errorf("backfill item weights: %w", err)
		}
	}
	if resourceWeightAdded {
		if _, err := tx.Exec(`UPDATE resource_types SET weight_units = 1`); err != nil {
			return fmt.Errorf("backfill resource weights: %w", err)
		}
	}
	return nil
}

func ensureItemDurabilitySchema(tx *sql.Tx, migrationNow time.Time) error {
	itemColumns, err := tableColumns(tx, "items")
	if err != nil {
		return err
	}
	if !itemColumns["max_durability_seconds"] {
		if _, err := tx.Exec(fmt.Sprintf(`ALTER TABLE items ADD COLUMN max_durability_seconds INTEGER NOT NULL DEFAULT %d CHECK (max_durability_seconds > 0)`, itemDefaultDurabilitySeconds)); err != nil {
			return fmt.Errorf("add item durability: %w", err)
		}
	}
	if err := migrateItemHoldingTable(tx, "player_inventory", "user_id", migrationNow); err != nil {
		return err
	}
	if err := migrateItemHoldingTable(tx, "ground_items", "location_id", migrationNow); err != nil {
		return err
	}
	return nil
}

func migrateItemHoldingTable(tx *sql.Tx, table, scopeColumn string, migrationNow time.Time) error {
	columns, err := tableColumns(tx, table)
	if err != nil {
		return err
	}
	if columns["durability_status"] && columns["status_expires_at"] {
		return nil
	}
	legacyTable := table + "_legacy"
	if _, err := tx.Exec(`ALTER TABLE ` + table + ` RENAME TO ` + legacyTable); err != nil {
		return fmt.Errorf("rename legacy %s: %w", table, err)
	}
	if table == "player_inventory" {
		if _, err := tx.Exec(`
CREATE TABLE player_inventory (
	user_id INTEGER NOT NULL REFERENCES identities(id),
	item_id TEXT NOT NULL REFERENCES items(id),
	durability_status TEXT NOT NULL DEFAULT 'active' CHECK (durability_status IN ('active', 'expired')),
	status_expires_at INTEGER NOT NULL DEFAULT 0,
	quantity INTEGER NOT NULL CHECK (quantity > 0),
	PRIMARY KEY (user_id, item_id, durability_status)
)`); err != nil {
			return fmt.Errorf("create migrated player inventory: %w", err)
		}
	} else {
		if _, err := tx.Exec(`
CREATE TABLE ground_items (
	location_id TEXT NOT NULL REFERENCES locations(id),
	item_id TEXT NOT NULL REFERENCES items(id),
	durability_status TEXT NOT NULL DEFAULT 'active' CHECK (durability_status IN ('active', 'expired')),
	status_expires_at INTEGER NOT NULL DEFAULT 0,
	quantity INTEGER NOT NULL CHECK (quantity > 0),
	PRIMARY KEY (location_id, item_id, durability_status)
)`); err != nil {
			return fmt.Errorf("create migrated ground items: %w", err)
		}
	}
	if _, err := tx.Exec(fmt.Sprintf(`
INSERT INTO %s (%s, item_id, durability_status, status_expires_at, quantity)
SELECT legacy.%s, legacy.item_id, 'active', ? + i.max_durability_seconds, legacy.quantity
FROM %s legacy
JOIN items i ON i.id = legacy.item_id`, table, scopeColumn, scopeColumn, legacyTable), migrationNow.Unix()); err != nil {
		return fmt.Errorf("migrate %s holdings: %w", table, err)
	}
	if _, err := tx.Exec(`DROP TABLE ` + legacyTable); err != nil {
		return fmt.Errorf("drop legacy %s: %w", table, err)
	}
	return nil
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
	recipeColumns, err := tableColumns(tx, "building_recipes")
	if err != nil {
		return err
	}
	if !recipeColumns["max_durability_seconds"] {
		if _, err := tx.Exec(fmt.Sprintf(`ALTER TABLE building_recipes ADD COLUMN max_durability_seconds INTEGER NOT NULL DEFAULT %d CHECK (max_durability_seconds > 0)`, buildingDefaultDurabilitySeconds)); err != nil {
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
		if _, err := tx.Exec(fmt.Sprintf(`ALTER TABLE buildings ADD COLUMN max_durability_seconds INTEGER NOT NULL DEFAULT %d CHECK (max_durability_seconds > 0)`, buildingDefaultDurabilitySeconds)); err != nil {
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
INSERT INTO player_hp (user_id, full_timestamp) VALUES (?, ?)
ON CONFLICT (user_id) DO NOTHING`, identity.ID, now); err != nil {
		_ = tx.Rollback()
		return Identity{}, fmt.Errorf("initialize player HP: %w", err)
	}
	if _, err := tx.Exec(`
INSERT INTO player_locations (user_id, location_id) VALUES (?, 'camp')
ON CONFLICT (user_id) DO NOTHING`, identity.ID); err != nil {
		_ = tx.Rollback()
		return Identity{}, fmt.Errorf("initialize player location: %w", err)
	}
	if _, err := tx.Exec(`
INSERT INTO player_profiles (user_id, updated_at) VALUES (?, ?)
ON CONFLICT (user_id) DO NOTHING`, identity.ID, now); err != nil {
		_ = tx.Rollback()
		return Identity{}, fmt.Errorf("initialize player profile: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return Identity{}, fmt.Errorf("commit upsert identity: %w", err)
	}
	identity.CreatedAt = unixSeconds(createdAt)
	identity.UpdatedAt = unixSeconds(updatedAt)
	return identity, nil
}

func normalizePlayerName(playerName string) (string, string, error) {
	for _, character := range playerName {
		if unicode.IsControl(character) {
			return "", "", ErrInvalidPlayerName
		}
	}
	trimmed := strings.TrimSpace(playerName)
	if trimmed == "" || !utf8.ValidString(trimmed) {
		return "", "", ErrInvalidPlayerName
	}
	length := 0
	for _, character := range trimmed {
		if character < utf8.RuneSelf {
			length++
		} else {
			length += 2
		}
		if length > 16 {
			return "", "", ErrInvalidPlayerName
		}
	}
	if length == 0 {
		return "", "", ErrInvalidPlayerName
	}
	normalized := norm.NFKC.String(trimmed)
	normalized = strings.Map(func(character rune) rune {
		if character >= 'A' && character <= 'Z' {
			return character + ('a' - 'A')
		}
		return character
	}, normalized)
	return trimmed, normalized, nil
}

func (s *Store) GetPlayerProfile(userID int64) (PlayerProfile, error) {
	if userID <= 0 {
		return PlayerProfile{}, fmt.Errorf("%w: user ID is required", ErrInvalidArgument)
	}
	var profile PlayerProfile
	var playerName, normalizedName sql.NullString
	var updatedAt int64
	err := s.db.QueryRow(`
SELECT user_id, player_name, normalized_name, updated_at
FROM player_profiles WHERE user_id = ?`, userID).Scan(&profile.UserID, &playerName, &normalizedName, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return PlayerProfile{}, ErrIdentityNotFound
	}
	if err != nil {
		return PlayerProfile{}, fmt.Errorf("get player profile: %w", err)
	}
	if playerName.Valid {
		value := playerName.String
		profile.PlayerName = &value
	}
	if normalizedName.Valid {
		value := normalizedName.String
		profile.NormalizedName = &value
	}
	profile.UpdatedAt = unixSeconds(updatedAt)
	return profile, nil
}

func (s *Store) GetPlayerName(userID int64) (*string, error) {
	profile, err := s.GetPlayerProfile(userID)
	if err != nil {
		return nil, err
	}
	return profile.PlayerName, nil
}

func (s *Store) SetPlayerName(userID int64, playerName string) (PlayerProfile, error) {
	if userID <= 0 {
		return PlayerProfile{}, fmt.Errorf("%w: user ID is required", ErrInvalidArgument)
	}
	displayName, normalizedName, err := normalizePlayerName(playerName)
	if err != nil {
		return PlayerProfile{}, err
	}
	now := s.now().UTC().Unix()
	tx, err := s.db.Begin()
	if err != nil {
		return PlayerProfile{}, fmt.Errorf("begin player name update: %w", err)
	}
	var existingUserID int64
	err = tx.QueryRow(`SELECT user_id FROM player_profiles WHERE normalized_name = ? AND user_id <> ?`, normalizedName, userID).Scan(&existingUserID)
	if err == nil {
		_ = tx.Rollback()
		return PlayerProfile{}, ErrPlayerNameUnavailable
	}
	if !errors.Is(err, sql.ErrNoRows) {
		_ = tx.Rollback()
		return PlayerProfile{}, fmt.Errorf("check player name availability: %w", err)
	}
	result, err := tx.Exec(`
UPDATE player_profiles
SET player_name = ?, normalized_name = ?, updated_at = ?
WHERE user_id = ?`, displayName, normalizedName, now, userID)
	if err != nil {
		_ = tx.Rollback()
		if strings.Contains(err.Error(), "UNIQUE constraint failed: player_profiles.normalized_name") {
			return PlayerProfile{}, ErrPlayerNameUnavailable
		}
		return PlayerProfile{}, fmt.Errorf("update player name: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		_ = tx.Rollback()
		return PlayerProfile{}, fmt.Errorf("count player name update: %w", err)
	}
	if rowsAffected != 1 {
		_ = tx.Rollback()
		return PlayerProfile{}, ErrIdentityNotFound
	}
	if err := tx.Commit(); err != nil {
		return PlayerProfile{}, fmt.Errorf("commit player name update: %w", err)
	}
	value := displayName
	normalizedValue := normalizedName
	return PlayerProfile{UserID: userID, PlayerName: &value, NormalizedName: &normalizedValue, UpdatedAt: unixSeconds(now)}, nil
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

func (s *Store) GetPlayerCombatDefinition() (PlayerCombatDefinition, error) {
	var definition PlayerCombatDefinition
	err := s.db.QueryRow(`
SELECT id, max_hp, hp_recovery_interval_seconds, base_attack_power
FROM player_combat_definitions WHERE id = 1`).Scan(
		&definition.ID,
		&definition.MaxHP,
		&definition.HPRecoveryIntervalSeconds,
		&definition.BaseAttackPower,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return PlayerCombatDefinition{}, sql.ErrNoRows
	}
	if err != nil {
		return PlayerCombatDefinition{}, fmt.Errorf("get player combat definition: %w", err)
	}
	return definition, nil
}

func (s *Store) GetPlayerHPState(userID int64) (PlayerHP, error) {
	if userID <= 0 {
		return PlayerHP{}, fmt.Errorf("%w: user ID is required", ErrInvalidArgument)
	}
	var hp PlayerHP
	var fullTimestamp int64
	err := s.db.QueryRow(`
SELECT user_id, full_timestamp FROM player_hp WHERE user_id = ?`, userID).Scan(&hp.UserID, &fullTimestamp)
	if errors.Is(err, sql.ErrNoRows) {
		return PlayerHP{}, ErrIdentityNotFound
	}
	if err != nil {
		return PlayerHP{}, fmt.Errorf("get player HP: %w", err)
	}
	hp.FullTimestamp = unixSeconds(fullTimestamp)
	return hp, nil
}

func (s *Store) GetHP(userID int64) (int, error) {
	hp, err := s.GetPlayerHPState(userID)
	if err != nil {
		return 0, err
	}
	definition, err := s.GetPlayerCombatDefinition()
	if err != nil {
		return 0, err
	}
	return calculateHP(hp.FullTimestamp, s.now().UTC(), definition.MaxHP, definition.HPRecoveryIntervalSeconds), nil
}

func (s *Store) GetLocationMonsterState(locationID string) (LocationMonsterState, error) {
	if strings.TrimSpace(locationID) == "" {
		return LocationMonsterState{}, fmt.Errorf("%w: location ID is required", ErrInvalidArgument)
	}
	tx, err := s.db.Begin()
	if err != nil {
		return LocationMonsterState{}, fmt.Errorf("begin location monster state read: %w", err)
	}
	state, err := settleLocationMonstersTx(tx, locationID, s.now().UTC(), s.monsterRoll)
	if err != nil {
		_ = tx.Rollback()
		return LocationMonsterState{}, err
	}
	if err := tx.Commit(); err != nil {
		return LocationMonsterState{}, fmt.Errorf("commit location monster state read: %w", err)
	}
	return state, nil
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
	return s.getPlayerStateTxWithOptions(tx, userID, now, true, true)
}

func (s *Store) getPlayerStateTxWithOptions(tx *sql.Tx, userID int64, now time.Time, normalizeItems, settleMonsters bool) (PlayerState, error) {
	state := PlayerState{Routes: make([]Route, 0), Inventory: make([]InventoryItem, 0), GroundItems: make([]GroundItem, 0), GroundResources: make([]GroundResource, 0), Resources: make([]PlayerResource, 0), ConversionMethods: make([]ConversionMethod, 0), CraftingRecipes: make([]CraftingRecipe, 0), BuildingRecipes: make([]BuildingRecipe, 0), BuildingExtensionDefinitions: make([]BuildingExtensionDefinition, 0), Buildings: make([]Building, 0), ItemDurabilityComputations: make([]ItemDurabilityComputation, 0), ItemDurabilityCleanups: make([]ItemDurabilityCleanup, 0), MovementWeightThreshold: movementWeightThreshold}
	if normalizeItems {
		cleanups, err := normalizeItemHoldingsWithMetadataTx(tx, now, userID)
		if err != nil {
			return PlayerState{}, err
		}
		state.ItemDurabilityCleanups = cleanups
		if err := deleteDestroyedBuildingsTx(tx, now); err != nil {
			return PlayerState{}, err
		}
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
	if settleMonsters {
		monsterState, err := settleLocationMonstersTx(tx, state.Location.ID, now, s.monsterRoll)
		if err != nil {
			return PlayerState{}, err
		}
		state.MonsterCount = monsterState.MonsterCount
		state.MonsterSettlement = monsterState.Computation
	} else {
		if err := tx.QueryRow(`SELECT monster_count FROM location_monster_populations WHERE location_id = ?`, state.Location.ID).Scan(&state.MonsterCount); err != nil {
			return PlayerState{}, fmt.Errorf("get settled location monster population: %w", err)
		}
	}
	var gathering GatheringOption
	err = tx.QueryRow(`
SELECT i.id, i.display_name, i.weight_units, i.max_durability_seconds, gr.quantity, gr.ap_cost
FROM gathering_rules gr
JOIN items i ON i.id = gr.item_id
WHERE gr.location_id = ?`, state.Location.ID).Scan(
		&gathering.Item.ID, &gathering.Item.DisplayName, &gathering.Item.WeightUnits, &gathering.Item.MaxDurabilitySeconds, &gathering.Quantity, &gathering.APCost,
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
	if err := loadConversionMethodsTx(tx, &state); err != nil {
		return PlayerState{}, err
	}
	if err := loadBuildingExtensionDefinitionsTx(tx, &state); err != nil {
		return PlayerState{}, err
	}
	inventoryRows, err := tx.Query(`
SELECT i.id, i.display_name, i.weight_units, i.max_durability_seconds, pi.quantity, pi.durability_status, pi.status_expires_at
FROM player_inventory pi
JOIN items i ON i.id = pi.item_id
WHERE pi.user_id = ?
ORDER BY pi.item_id, pi.durability_status`, userID)
	if err != nil {
		return PlayerState{}, fmt.Errorf("get player inventory: %w", err)
	}
	defer inventoryRows.Close()
	for inventoryRows.Next() {
		var inventoryItem InventoryItem
		var expiresAt int64
		if err := inventoryRows.Scan(&inventoryItem.Item.ID, &inventoryItem.Item.DisplayName, &inventoryItem.Item.WeightUnits, &inventoryItem.Item.MaxDurabilitySeconds, &inventoryItem.Quantity, &inventoryItem.DurabilityStatus, &expiresAt); err != nil {
			return PlayerState{}, fmt.Errorf("scan player inventory: %w", err)
		}
		setItemDurability(&inventoryItem.DurabilityStatus, &inventoryItem.DurabilityRemainingSeconds, &inventoryItem.RetentionRemainingSeconds, expiresAt, now)
		state.Inventory = append(state.Inventory, inventoryItem)
		state.ItemDurabilityComputations = append(state.ItemDurabilityComputations, ItemDurabilityComputation{Holding: "inventory", ItemID: inventoryItem.Item.ID, Quantity: inventoryItem.Quantity, DurabilityStatus: inventoryItem.DurabilityStatus, DurabilityRemainingSeconds: inventoryItem.DurabilityRemainingSeconds, RetentionRemainingSeconds: inventoryItem.RetentionRemainingSeconds})
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
	definition, err := playerCombatDefinitionTx(tx)
	if err != nil {
		return PlayerState{}, err
	}
	var hpTimestamp int64
	if err := tx.QueryRow(`SELECT full_timestamp FROM player_hp WHERE user_id = ?`, userID).Scan(&hpTimestamp); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return PlayerState{}, ErrIdentityNotFound
		}
		return PlayerState{}, fmt.Errorf("get player HP: %w", err)
	}
	state.HP = calculateHP(unixSeconds(hpTimestamp), now, definition.MaxHP, definition.HPRecoveryIntervalSeconds)
	state.AttackAPCost = defaultActiveAttackAPCost
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
SELECT i.id, i.display_name, i.weight_units, i.max_durability_seconds, gi.quantity, gi.durability_status, gi.status_expires_at
FROM ground_items gi
JOIN items i ON i.id = gi.item_id
WHERE gi.location_id = ?
ORDER BY gi.item_id, gi.durability_status`, state.Location.ID)
	if err != nil {
		return PlayerState{}, fmt.Errorf("get ground items: %w", err)
	}
	defer groundItemRows.Close()
	for groundItemRows.Next() {
		var groundItem GroundItem
		var expiresAt int64
		if err := groundItemRows.Scan(&groundItem.Item.ID, &groundItem.Item.DisplayName, &groundItem.Item.WeightUnits, &groundItem.Item.MaxDurabilitySeconds, &groundItem.Quantity, &groundItem.DurabilityStatus, &expiresAt); err != nil {
			return PlayerState{}, fmt.Errorf("scan ground item: %w", err)
		}
		setItemDurability(&groundItem.DurabilityStatus, &groundItem.DurabilityRemainingSeconds, &groundItem.RetentionRemainingSeconds, expiresAt, now)
		state.GroundItems = append(state.GroundItems, groundItem)
		state.ItemDurabilityComputations = append(state.ItemDurabilityComputations, ItemDurabilityComputation{Holding: "ground", ItemID: groundItem.Item.ID, Quantity: groundItem.Quantity, DurabilityStatus: groundItem.DurabilityStatus, DurabilityRemainingSeconds: groundItem.DurabilityRemainingSeconds, RetentionRemainingSeconds: groundItem.RetentionRemainingSeconds})
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
SELECT cr.id, cr.display_name, cr.base_ap_cost, i.id, i.display_name, i.weight_units, i.max_durability_seconds, cr.output_quantity
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
		if err := recipeRows.Scan(&recipe.ID, &recipe.DisplayName, &recipe.BaseAPCost, &recipe.Output.ID, &recipe.Output.DisplayName, &recipe.Output.WeightUnits, &recipe.Output.MaxDurabilitySeconds, &recipe.OutputQuantity); err != nil {
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
		building := Building{Extensions: make([]BuildingExtension, 0)}
		var durabilityExpiresAt sql.NullInt64
		if err := buildingRows.Scan(&building.ID, &building.Owner.ID, &building.Owner.DisplayName, &building.Recipe.ID, &building.Recipe.DisplayName, &building.BuildingLevel, &building.RequiredAP, &building.ContributedAP, &building.Status, &building.ExtensionSlotCount, &building.MaxDurabilitySeconds, &durabilityExpiresAt); err != nil {
			return PlayerState{}, fmt.Errorf("scan building: %w", err)
		}
		building.Recipe.MaxDurabilitySeconds = building.MaxDurabilitySeconds
		setBuildingDurability(&building, durabilityExpiresAt, now)
		if err := loadBuildingExtensionsTx(tx, &building); err != nil {
			return PlayerState{}, err
		}
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
	state.CarriedWeight, err = carryingWeightTx(tx, userID)
	if err != nil {
		return PlayerState{}, err
	}
	return state, nil
}

func settleLocationMonstersTx(tx *sql.Tx, locationID string, now time.Time, roll func() int) (LocationMonsterState, error) {
	var state LocationMonsterState
	var settledAt int64
	computation := &MonsterSettlementComputation{LocationID: locationID, Outcome: "unchanged"}
	err := tx.QueryRow(`
SELECT monster_count, settled_at
FROM location_monster_populations
WHERE location_id = ?`, locationID).Scan(&state.MonsterCount, &settledAt)
	if errors.Is(err, sql.ErrNoRows) {
		var exists int
		if err := tx.QueryRow(`SELECT 1 FROM locations WHERE id = ?`, locationID).Scan(&exists); errors.Is(err, sql.ErrNoRows) {
			return LocationMonsterState{}, sql.ErrNoRows
		} else if err != nil {
			return LocationMonsterState{}, fmt.Errorf("find monster location: %w", err)
		}
		settledAt = now.Unix()
		if _, err := tx.Exec(`
INSERT INTO location_monster_populations (location_id, monster_count, settled_at)
VALUES (?, 0, ?)`, locationID, settledAt); err != nil {
			return LocationMonsterState{}, fmt.Errorf("initialize location monster population: %w", err)
		}
		state.MonsterCount = 0
		computation.Outcome = "initialized"
	} else if err != nil {
		return LocationMonsterState{}, fmt.Errorf("get location monster population: %w", err)
	}
	state.LocationID = locationID
	state.SettledAt = unixSeconds(settledAt)
	computation.MonsterCountBefore = state.MonsterCount

	var intervalSeconds, spawnChanceBPS, maxMonsters int64
	err = tx.QueryRow(`
SELECT spawn_interval_seconds, spawn_chance_bps, max_monsters
FROM location_monster_rules
WHERE location_id = ?`, locationID).Scan(&intervalSeconds, &spawnChanceBPS, &maxMonsters)
	if errors.Is(err, sql.ErrNoRows) {
		computation.MonsterCountAfter = state.MonsterCount
		state.Computation = computation
		return state, nil
	}
	if err != nil {
		return LocationMonsterState{}, fmt.Errorf("get location monster rule: %w", err)
	}
	computation.SpawnChanceBPS = int(spawnChanceBPS)
	if state.MonsterCount > int(maxMonsters) {
		state.MonsterCount = int(maxMonsters)
	}
	computation.MonsterCountAfter = state.MonsterCount

	nowUnix := now.Unix()
	if nowUnix <= settledAt {
		state.Computation = computation
		return state, nil
	}
	intervals := (nowUnix - settledAt) / intervalSeconds
	if intervals == 0 {
		state.Computation = computation
		return state, nil
	}
	computation.Intervals = uint64(intervals)
	if roll == nil {
		roll = func() int { return rand.Intn(10000) }
	}
	for interval := int64(0); interval < intervals && state.MonsterCount < int(maxMonsters); interval++ {
		if roll() < int(spawnChanceBPS) {
			state.MonsterCount++
		}
	}
	nextSettledAt := settledAt + intervals*intervalSeconds
	result, err := tx.Exec(`
UPDATE location_monster_populations
SET monster_count = ?, settled_at = ?
WHERE location_id = ? AND settled_at = ?`, state.MonsterCount, nextSettledAt, locationID, settledAt)
	if err != nil {
		return LocationMonsterState{}, fmt.Errorf("settle location monster population: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return LocationMonsterState{}, fmt.Errorf("check location monster settlement: %w", err)
	}
	if rows != 1 {
		return LocationMonsterState{}, fmt.Errorf("location monster settlement lost update")
	}
	state.SettledAt = unixSeconds(nextSettledAt)
	computation.MonsterCountAfter = state.MonsterCount
	if state.MonsterCount != computation.MonsterCountBefore {
		computation.Outcome = "spawned"
	} else if state.MonsterCount >= int(maxMonsters) && maxMonsters > 0 {
		computation.Outcome = "capped"
	}
	state.Computation = computation
	return state, nil
}

func canAttack(monsterCount, ap, attackAPCost int) bool {
	return monsterCount > 0 && ap >= attackAPCost
}

func normalizeCombatRoll(roll func() int) int {
	if roll == nil {
		return 0
	}
	value := roll() % 10000
	if value < 0 {
		value += 10000
	}
	return value
}

func damageRoll(attackPower int, roll func() int) int {
	minimum := (attackPower + 1) / 2
	if minimum < 1 {
		minimum = 1
	}
	maximum := (attackPower * 3) / 2
	if maximum < minimum {
		maximum = minimum
	}
	return minimum + normalizeCombatRoll(roll)*(maximum-minimum+1)/10000
}

func combinedInterceptionChanceBPS(perMonsterChanceBPS, monsterCount int) int {
	if perMonsterChanceBPS <= 0 || monsterCount <= 0 {
		return 0
	}
	if perMonsterChanceBPS >= 10000 {
		return 10000
	}
	remaining := 10000.0 * math.Pow(float64(10000-perMonsterChanceBPS)/10000.0, float64(monsterCount))
	return 10000 - int(math.Floor(remaining))
}

type encounterMonster struct {
	ID          int64
	DisplayName string
	MaxHP       int
	AttackPower int
	Weight      int
}

func selectEncounterMonsterTx(tx *sql.Tx, locationID string, roll func() int) (encounterMonster, error) {
	rows, err := tx.Query(`
SELECT mt.id, mt.display_name, mt.max_hp, mt.attack_power, e.encounter_weight
FROM location_monster_encounters e
JOIN monster_types mt ON mt.id = e.monster_type_id
WHERE e.location_id = ?
ORDER BY mt.id`, locationID)
	if err != nil {
		return encounterMonster{}, fmt.Errorf("get monster encounters: %w", err)
	}
	defer rows.Close()
	encounters := make([]encounterMonster, 0)
	totalWeight := 0
	for rows.Next() {
		var monster encounterMonster
		if err := rows.Scan(&monster.ID, &monster.DisplayName, &monster.MaxHP, &monster.AttackPower, &monster.Weight); err != nil {
			return encounterMonster{}, fmt.Errorf("scan monster encounter: %w", err)
		}
		totalWeight += monster.Weight
		encounters = append(encounters, monster)
	}
	if err := rows.Err(); err != nil {
		return encounterMonster{}, fmt.Errorf("read monster encounters: %w", err)
	}
	if totalWeight == 0 {
		return encounterMonster{}, fmt.Errorf("location has no monster encounters")
	}
	selected := normalizeCombatRoll(roll) % totalWeight
	for _, monster := range encounters {
		if selected < monster.Weight {
			return monster, nil
		}
		selected -= monster.Weight
	}
	return encounterMonster{}, fmt.Errorf("select monster encounter")
}

func (s *Store) combatRandom() func() int {
	if s.combatRoll != nil {
		return s.combatRoll
	}
	return s.monsterRoll
}

func playerCombatDefinitionTx(tx *sql.Tx) (PlayerCombatDefinition, error) {
	var definition PlayerCombatDefinition
	err := tx.QueryRow(`
SELECT id, max_hp, hp_recovery_interval_seconds, base_attack_power
FROM player_combat_definitions WHERE id = 1`).Scan(
		&definition.ID, &definition.MaxHP, &definition.HPRecoveryIntervalSeconds, &definition.BaseAttackPower,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return PlayerCombatDefinition{}, sql.ErrNoRows
	}
	if err != nil {
		return PlayerCombatDefinition{}, fmt.Errorf("get player combat definition: %w", err)
	}
	return definition, nil
}

func persistHPForValueTx(tx *sql.Tx, userID int64, hp int, definition PlayerCombatDefinition, now time.Time) error {
	if hp < 1 {
		hp = 1
	}
	if hp > definition.MaxHP {
		hp = definition.MaxHP
	}
	fullTimestamp := now.Add(time.Duration(definition.MaxHP-hp) * time.Duration(definition.HPRecoveryIntervalSeconds) * time.Second).Unix()
	if _, err := tx.Exec(`UPDATE player_hp SET full_timestamp = ? WHERE user_id = ?`, fullTimestamp, userID); err != nil {
		return fmt.Errorf("persist player HP: %w", err)
	}
	return nil
}

func consumeAttackAPTx(tx *sql.Tx, userID int64, fullTimestamp int64, cost int, now time.Time) error {
	fullAt := unixSeconds(fullTimestamp)
	if fullAt.Before(now) {
		fullAt = now
	}
	nextFullTimestamp := fullAt.Add(time.Duration(cost) * apRecoveryTime).Unix()
	result, err := tx.Exec(`
UPDATE player_ap SET full_timestamp = ?
WHERE user_id = ? AND full_timestamp = ?`, nextFullTimestamp, userID, fullTimestamp)
	if err != nil {
		return fmt.Errorf("consume attack AP: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check attack AP: %w", err)
	}
	if rows != 1 {
		return ErrInsufficientAP
	}
	return nil
}

func (s *Store) resolveCombatTx(tx *sql.Tx, userID int64, locationID string, now time.Time) (CombatResult, error) {
	definition, err := playerCombatDefinitionTx(tx)
	if err != nil {
		return CombatResult{}, err
	}
	monster, err := selectEncounterMonsterTx(tx, locationID, s.combatRandom())
	if err != nil {
		return CombatResult{}, err
	}
	combat := CombatResult{Monster: CombatMonster{TypeID: monster.ID, DisplayName: monster.DisplayName}, Events: make([]CombatEvent, 0), Drops: make([]CombatDrop, 0), DropCalculations: make([]CombatDropCalculation, 0)}
	var fullTimestamp int64
	if err := tx.QueryRow(`SELECT full_timestamp FROM player_hp WHERE user_id = ?`, userID).Scan(&fullTimestamp); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return CombatResult{}, ErrIdentityNotFound
		}
		return CombatResult{}, fmt.Errorf("get player HP for combat: %w", err)
	}
	playerHP := calculateHP(unixSeconds(fullTimestamp), now, definition.MaxHP, definition.HPRecoveryIntervalSeconds)
	monsterHP := monster.MaxHP
	for playerHP > 0 && monsterHP > 0 {
		damage := damageRoll(definition.BaseAttackPower, s.combatRandom())
		monsterHP -= damage
		if monsterHP < 0 {
			monsterHP = 0
		}
		combat.Events = append(combat.Events, CombatEvent{Attacker: "player", Damage: damage, TargetRemainingHP: monsterHP})
		if monsterHP == 0 {
			combat.Result = "victory"
			break
		}
		damage = damageRoll(monster.AttackPower, s.combatRandom())
		playerHP -= damage
		if playerHP < 0 {
			playerHP = 0
		}
		combat.Events = append(combat.Events, CombatEvent{Attacker: "monster", Damage: damage, TargetRemainingHP: playerHP})
		if playerHP == 0 {
			combat.Result = "defeat"
		}
	}
	if combat.Result == "defeat" {
		if err := persistHPForValueTx(tx, userID, 1, definition, now); err != nil {
			return CombatResult{}, err
		}
		return combat, nil
	}
	if len(combat.Events) > 1 {
		if err := persistHPForValueTx(tx, userID, playerHP, definition, now); err != nil {
			return CombatResult{}, err
		}
	}
	result, err := tx.Exec(`
UPDATE location_monster_populations
SET monster_count = monster_count - 1
WHERE location_id = ? AND monster_count > 0`, locationID)
	if err != nil {
		return CombatResult{}, fmt.Errorf("decrement location monster population: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return CombatResult{}, fmt.Errorf("check location monster decrement: %w", err)
	}
	if rows != 1 {
		return CombatResult{}, ErrNoMonster
	}
	dropRows, err := tx.Query(`
SELECT i.id, i.display_name, i.weight_units, i.max_durability_seconds, d.chance_bps, d.quantity
FROM monster_drop_rules d
JOIN items i ON i.id = d.item_id
WHERE d.monster_type_id = ?
ORDER BY i.id`, monster.ID)
	if err != nil {
		return CombatResult{}, fmt.Errorf("get monster drops: %w", err)
	}
	defer dropRows.Close()
	for dropRows.Next() {
		var drop CombatDrop
		var chance int
		if err := dropRows.Scan(&drop.Item.ID, &drop.Item.DisplayName, &drop.Item.WeightUnits, &drop.Item.MaxDurabilitySeconds, &chance, &drop.Quantity); err != nil {
			return CombatResult{}, fmt.Errorf("scan monster drop: %w", err)
		}
		outcome := "not_dropped"
		if normalizeCombatRoll(s.combatRandom()) < chance {
			if err := addActiveItemHoldingTx(tx, "player_inventory", "user_id", userID, drop.Item.ID, drop.Quantity, now.Unix()+int64(drop.Item.MaxDurabilitySeconds)); err != nil {
				return CombatResult{}, fmt.Errorf("add monster drop: %w", err)
			}
			combat.Drops = append(combat.Drops, drop)
			outcome = "dropped"
		}
		combat.DropCalculations = append(combat.DropCalculations, CombatDropCalculation{ItemID: drop.Item.ID, ChanceBPS: chance, Quantity: drop.Quantity, Outcome: outcome})
	}
	if err := dropRows.Err(); err != nil {
		return CombatResult{}, fmt.Errorf("read monster drops: %w", err)
	}
	return combat, nil
}

func (s *Store) Attack(userID int64) (PlayerState, CombatResult, error) {
	if userID <= 0 {
		return PlayerState{}, CombatResult{}, fmt.Errorf("%w: user ID is required", ErrInvalidArgument)
	}
	tx, err := s.db.Begin()
	if err != nil {
		return PlayerState{}, CombatResult{}, fmt.Errorf("begin attack: %w", err)
	}
	now := s.now().UTC()
	locationID, err := playerLocationTx(tx, userID)
	if err != nil {
		_ = tx.Rollback()
		return PlayerState{}, CombatResult{}, err
	}
	var fullTimestamp int64
	if err := tx.QueryRow(`SELECT full_timestamp FROM player_ap WHERE user_id = ?`, userID).Scan(&fullTimestamp); err != nil {
		_ = tx.Rollback()
		if errors.Is(err, sql.ErrNoRows) {
			return PlayerState{}, CombatResult{}, ErrIdentityNotFound
		}
		return PlayerState{}, CombatResult{}, fmt.Errorf("get player AP for attack: %w", err)
	}
	if calculateAP(unixSeconds(fullTimestamp), now) < defaultActiveAttackAPCost {
		_ = tx.Rollback()
		return PlayerState{}, CombatResult{}, ErrInsufficientAP
	}
	monsterState, err := settleLocationMonstersTx(tx, locationID, now, s.monsterRoll)
	if err != nil {
		_ = tx.Rollback()
		return PlayerState{}, CombatResult{}, err
	}
	if monsterState.MonsterCount == 0 {
		state, stateErr := s.getPlayerStateTxWithOptions(tx, userID, now, false, false)
		if stateErr != nil {
			_ = tx.Rollback()
			return PlayerState{}, CombatResult{}, stateErr
		}
		if err := tx.Commit(); err != nil {
			return PlayerState{}, CombatResult{}, fmt.Errorf("commit empty attack settlement: %w", err)
		}
		return state, CombatResult{}, ErrNoMonster
	}
	cleanupEvents, err := normalizeItemHoldingsWithMetadataTx(tx, now, userID)
	if err != nil {
		_ = tx.Rollback()
		return PlayerState{}, CombatResult{}, err
	}
	if err := consumeAttackAPTx(tx, userID, fullTimestamp, defaultActiveAttackAPCost, now); err != nil {
		_ = tx.Rollback()
		return PlayerState{}, CombatResult{}, err
	}
	combat, err := s.resolveCombatTx(tx, userID, locationID, now)
	if err != nil {
		_ = tx.Rollback()
		return PlayerState{}, CombatResult{}, err
	}
	state, err := s.getPlayerStateTx(tx, userID, now)
	if err != nil {
		_ = tx.Rollback()
		return PlayerState{}, CombatResult{}, err
	}
	prependItemDurabilityCleanups(&state, cleanupEvents)
	if err := tx.Commit(); err != nil {
		return PlayerState{}, CombatResult{}, fmt.Errorf("commit attack: %w", err)
	}
	return state, combat, nil
}

func carryingWeightTx(tx *sql.Tx, userID int64) (int, error) {
	var weight int
	err := tx.QueryRow(`
SELECT COALESCE((SELECT SUM(pi.quantity * i.weight_units)
                 FROM player_inventory pi
                 JOIN items i ON i.id = pi.item_id
                 WHERE pi.user_id = ?), 0)
     + COALESCE((SELECT SUM(pr.quantity * rt.weight_units)
                 FROM player_resources pr
                 JOIN resource_types rt ON rt.id = pr.resource_id
                 WHERE pr.user_id = ?), 0)`, userID, userID).Scan(&weight)
	if err != nil {
		return 0, fmt.Errorf("calculate carrying weight: %w", err)
	}
	return weight, nil
}

func normalizeItemHoldingsWithMetadataTx(tx *sql.Tx, now time.Time, userID int64) ([]ItemDurabilityCleanup, error) {
	var locationID string
	if err := tx.QueryRow(`SELECT location_id FROM player_locations WHERE user_id = ?`, userID).Scan(&locationID); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("get player location for item normalization: %w", err)
	}
	if _, err := tx.Exec(`
UPDATE player_inventory
SET status_expires_at = (SELECT ? + i.max_durability_seconds FROM items i WHERE i.id = player_inventory.item_id)
WHERE user_id = ? AND durability_status = 'active' AND status_expires_at = 0`, now.Unix(), userID); err != nil {
		return nil, fmt.Errorf("initialize inventory item durability: %w", err)
	}
	if locationID != "" {
		if _, err := tx.Exec(`
UPDATE ground_items
SET status_expires_at = (SELECT ? + i.max_durability_seconds FROM items i WHERE i.id = ground_items.item_id)
		WHERE location_id = ? AND durability_status = 'active' AND status_expires_at = 0`, now.Unix(), locationID); err != nil {
			return nil, fmt.Errorf("initialize ground item durability: %w", err)
		}
	}
	cleanups := make([]ItemDurabilityCleanup, 0)
	holdings := []struct {
		table, scope, name string
		scopeValue         interface{}
	}{
		{table: "player_inventory", scope: "user_id", name: "inventory", scopeValue: userID},
	}
	for _, holding := range holdings {
		events, err := expireItemHoldingTableWithMetadataTx(tx, holding.table, holding.scope, holding.name, holding.scopeValue, now)
		if err != nil {
			return nil, err
		}
		cleanups = append(cleanups, events...)
	}
	if locationID != "" {
		events, err := expireItemHoldingTableWithMetadataTx(tx, "ground_items", "location_id", "ground", locationID, now)
		if err != nil {
			return nil, err
		}
		cleanups = append(cleanups, events...)
	}
	return cleanups, nil
}

func prependItemDurabilityCleanups(state *PlayerState, cleanups []ItemDurabilityCleanup) {
	state.ItemDurabilityCleanups = append(cleanups, state.ItemDurabilityCleanups...)
}

func expireItemHoldingTableWithMetadataTx(tx *sql.Tx, table, scopeColumn, holdingName string, scopeValue interface{}, now time.Time) ([]ItemDurabilityCleanup, error) {
	query := fmt.Sprintf(`SELECT %s, item_id, quantity, status_expires_at FROM %s WHERE %s = ? AND durability_status = 'active' AND status_expires_at <= ?`, scopeColumn, table, scopeColumn)
	rows, err := tx.Query(query, scopeValue, now.Unix())
	if err != nil {
		return nil, fmt.Errorf("find expired %s: %w", table, err)
	}
	type expiredHolding struct {
		scope, itemID string
		quantity      int
		expiresAt     int64
	}
	holdings := make([]expiredHolding, 0)
	for rows.Next() {
		var holding expiredHolding
		if err := rows.Scan(&holding.scope, &holding.itemID, &holding.quantity, &holding.expiresAt); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("scan expired %s: %w", table, err)
		}
		holdings = append(holdings, holding)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, fmt.Errorf("read expired %s: %w", table, err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("close expired %s: %w", table, err)
	}
	cleanups := make([]ItemDurabilityCleanup, 0, len(holdings))
	for _, holding := range holdings {
		insertQuery := fmt.Sprintf(`
INSERT INTO %s (%s, item_id, durability_status, status_expires_at, quantity)
VALUES (?, ?, 'expired', ?, ?)
ON CONFLICT (%s, item_id, durability_status) DO UPDATE SET
quantity = %s.quantity + excluded.quantity,
status_expires_at = MAX(%s.status_expires_at, excluded.status_expires_at)`, table, scopeColumn, scopeColumn, table, table)
		if _, err := tx.Exec(insertQuery, holding.scope, holding.itemID, holding.expiresAt+itemExpiredRetentionSeconds, holding.quantity); err != nil {
			return nil, fmt.Errorf("merge expired %s: %w", table, err)
		}
		deleteQuery := fmt.Sprintf(`DELETE FROM %s WHERE %s = ? AND item_id = ? AND durability_status = 'active'`, table, scopeColumn)
		if _, err := tx.Exec(deleteQuery, holding.scope, holding.itemID); err != nil {
			return nil, fmt.Errorf("remove active %s: %w", table, err)
		}
		if holdingName != "" {
			cleanups = append(cleanups, ItemDurabilityCleanup{Holding: holdingName, ItemID: holding.itemID, Quantity: holding.quantity, Action: "expired", ExpiredAt: holding.expiresAt, RetentionExpiresAt: holding.expiresAt + itemExpiredRetentionSeconds})
		}
	}
	cleanupRows, err := tx.Query(fmt.Sprintf(`SELECT item_id, quantity, status_expires_at FROM %s WHERE %s = ? AND durability_status = 'expired' AND status_expires_at <= ?`, table, scopeColumn), scopeValue, now.Unix())
	if err != nil {
		return nil, fmt.Errorf("find retained %s: %w", table, err)
	}
	type retainedHolding struct {
		itemID    string
		quantity  int
		expiresAt int64
	}
	retained := make([]retainedHolding, 0)
	for cleanupRows.Next() {
		var holding retainedHolding
		if err := cleanupRows.Scan(&holding.itemID, &holding.quantity, &holding.expiresAt); err != nil {
			_ = cleanupRows.Close()
			return nil, fmt.Errorf("scan retained %s: %w", table, err)
		}
		retained = append(retained, holding)
	}
	if err := cleanupRows.Err(); err != nil {
		_ = cleanupRows.Close()
		return nil, fmt.Errorf("read retained %s: %w", table, err)
	}
	if err := cleanupRows.Close(); err != nil {
		return nil, fmt.Errorf("close retained %s: %w", table, err)
	}
	cleanupQuery := fmt.Sprintf(`DELETE FROM %s WHERE %s = ? AND durability_status = 'expired' AND status_expires_at <= ?`, table, scopeColumn)
	if _, err := tx.Exec(cleanupQuery, scopeValue, now.Unix()); err != nil {
		return nil, fmt.Errorf("delete retained %s: %w", table, err)
	}
	if holdingName != "" {
		for _, holding := range retained {
			cleanups = append(cleanups, ItemDurabilityCleanup{Holding: holdingName, ItemID: holding.itemID, Quantity: holding.quantity, Action: "deleted", RetentionExpiresAt: holding.expiresAt})
		}
	}
	return cleanups, nil
}

func setItemDurability(status *string, durabilityRemaining, retentionRemaining **int, expiresAt int64, now time.Time) {
	remaining := int(expiresAt - now.Unix())
	if *status == "active" {
		if remaining < 0 {
			remaining = 0
		}
		*durabilityRemaining = &remaining
		return
	}
	*durabilityRemaining = nil
	if remaining < 0 {
		remaining = 0
	}
	*retentionRemaining = &remaining
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
	cutoff := now.Unix() - buildingDisabledRetentionSeconds
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
SELECT i.id, i.display_name, i.weight_units, i.max_durability_seconds, ii.quantity
FROM building_recipe_item_inputs ii
JOIN items i ON i.id = ii.item_id
WHERE ii.recipe_id = ? ORDER BY ii.item_id`, recipe.ID)
	if err != nil {
		return fmt.Errorf("get building item inputs: %w", err)
	}
	for itemRows.Next() {
		var input CraftingItemInput
		if err := itemRows.Scan(&input.Item.ID, &input.Item.DisplayName, &input.Item.WeightUnits, &input.Item.MaxDurabilitySeconds, &input.Quantity); err != nil {
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
SELECT i.id, i.display_name, i.weight_units, i.max_durability_seconds, ii.quantity
FROM crafting_recipe_item_inputs ii
JOIN items i ON i.id = ii.item_id
WHERE ii.recipe_id = ? ORDER BY ii.item_id`, recipe.ID)
	if err != nil {
		return fmt.Errorf("get crafting item inputs: %w", err)
	}
	for itemRows.Next() {
		var input CraftingItemInput
		if err := itemRows.Scan(&input.Item.ID, &input.Item.DisplayName, &input.Item.WeightUnits, &input.Item.MaxDurabilitySeconds, &input.Quantity); err != nil {
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
	SELECT cr.id, cr.display_name, cr.base_ap_cost, i.id, i.display_name, i.weight_units, i.max_durability_seconds, cr.output_quantity
FROM crafting_recipes cr
JOIN items i ON i.id = cr.output_item_id
WHERE cr.id = ? AND EXISTS (SELECT 1 FROM crafting_recipe_resource_inputs ri WHERE ri.recipe_id = cr.id)`, recipeID).Scan(
		&recipe.ID, &recipe.DisplayName, &recipe.BaseAPCost, &recipe.Output.ID, &recipe.Output.DisplayName, &recipe.Output.WeightUnits, &recipe.Output.MaxDurabilitySeconds, &recipe.OutputQuantity)
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
SELECT i.id, i.display_name, i.weight_units, i.max_durability_seconds, rt.id, rt.display_name, cr.input_quantity, cr.resource_yield, cr.ap_cost
FROM conversion_rules cr
JOIN items i ON i.id = cr.input_item_id
JOIN resource_types rt ON rt.id = cr.output_resource_id
WHERE cr.location_id = ?`, locationID).Scan(
		&conversion.Item.ID, &conversion.Item.DisplayName, &conversion.Item.WeightUnits, &conversion.Item.MaxDurabilitySeconds, &conversion.Resource.ID, &conversion.Resource.DisplayName, &conversion.InputQuantity,
		&conversion.ResourceYield, &conversion.APCost,
	)
	return conversion, err
}

func loadConversionMethodsTx(tx *sql.Tx, state *PlayerState) error {
	rows, err := tx.Query(`
SELECT cm.id, cm.display_name, cm.ap_cost, ii.id, ii.display_name, ii.weight_units, ii.max_durability_seconds,
       cm.max_input_quantity, output.id, output.display_name, cm.resource_quantity_per_input,
       essence.id, essence.display_name, essence.weight_units, essence.max_durability_seconds,
       cm.essence_chance_bps, cm.essence_quantity,
       EXISTS (SELECT 1 FROM global_conversion_methods gm WHERE gm.conversion_method_id = cm.id)
FROM conversion_methods cm
JOIN items ii ON ii.id = cm.input_item_id
JOIN resource_types output ON output.id = cm.output_resource_id
LEFT JOIN items essence ON essence.id = cm.essence_item_id
ORDER BY cm.id`)
	if err != nil {
		return fmt.Errorf("get conversion methods: %w", err)
	}
	for rows.Next() {
		method := ConversionMethod{ProviderDefinitionIDs: make([]string, 0)}
		var essenceID, essenceDisplay sql.NullString
		var essenceWeight, essenceDurability sql.NullInt64
		if err := rows.Scan(
			&method.ID, &method.DisplayName, &method.APCost,
			&method.Input.ID, &method.Input.DisplayName, &method.Input.WeightUnits, &method.Input.MaxDurabilitySeconds,
			&method.MaxInputQuantity, &method.OutputResource.ID, &method.OutputResource.DisplayName,
			&method.ResourceQuantityPerInput, &essenceID, &essenceDisplay, &essenceWeight, &essenceDurability,
			&method.EssenceChanceBPS, &method.EssenceQuantity, &method.IsGlobal,
		); err != nil {
			return fmt.Errorf("scan conversion method: %w", err)
		}
		if essenceID.Valid {
			method.EssenceItem = &Item{ID: essenceID.String, DisplayName: essenceDisplay.String, WeightUnits: int(essenceWeight.Int64), MaxDurabilitySeconds: int(essenceDurability.Int64)}
		}
		state.ConversionMethods = append(state.ConversionMethods, method)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return fmt.Errorf("read conversion methods: %w", err)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close conversion methods: %w", err)
	}
	providerRows, err := tx.Query(`
SELECT conversion_method_id, extension_definition_id
FROM extension_conversion_capabilities
ORDER BY conversion_method_id, extension_definition_id`)
	if err != nil {
		return fmt.Errorf("get conversion method providers: %w", err)
	}
	defer providerRows.Close()
	for providerRows.Next() {
		var methodID, definitionID string
		if err := providerRows.Scan(&methodID, &definitionID); err != nil {
			return fmt.Errorf("scan conversion method provider: %w", err)
		}
		for index := range state.ConversionMethods {
			if state.ConversionMethods[index].ID == methodID {
				state.ConversionMethods[index].ProviderDefinitionIDs = append(state.ConversionMethods[index].ProviderDefinitionIDs, definitionID)
				break
			}
		}
	}
	if err := providerRows.Err(); err != nil {
		return fmt.Errorf("read conversion method providers: %w", err)
	}
	return nil
}

func loadBuildingExtensionDefinitionsTx(tx *sql.Tx, state *PlayerState) error {
	rows, err := tx.Query(`
SELECT ed.id, ed.display_name, ed.tier, i.id, i.display_name, i.weight_units, i.max_durability_seconds, ed.required_ap
FROM building_extension_definitions ed
JOIN items i ON i.id = ed.package_item_id
ORDER BY ed.id`)
	if err != nil {
		return fmt.Errorf("get building extension definitions: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var definition BuildingExtensionDefinition
		if err := rows.Scan(
			&definition.ID, &definition.DisplayName, &definition.Tier,
			&definition.PackageItem.ID, &definition.PackageItem.DisplayName, &definition.PackageItem.WeightUnits,
			&definition.PackageItem.MaxDurabilitySeconds, &definition.RequiredAP,
		); err != nil {
			return fmt.Errorf("scan building extension definition: %w", err)
		}
		state.BuildingExtensionDefinitions = append(state.BuildingExtensionDefinitions, definition)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("read building extension definitions: %w", err)
	}
	return nil
}

func loadBuildingExtensionsTx(tx *sql.Tx, building *Building) error {
	rows, err := tx.Query(`
SELECT id, slot_index, definition_id, display_name, tier, required_ap, contributed_ap, status
FROM building_extensions
WHERE building_id = ?
ORDER BY slot_index`, building.ID)
	if err != nil {
		return fmt.Errorf("get building extensions: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var extension BuildingExtension
		if err := rows.Scan(&extension.ID, &extension.SlotIndex, &extension.DefinitionID, &extension.DisplayName, &extension.Tier, &extension.RequiredAP, &extension.ContributedAP, &extension.Status); err != nil {
			return fmt.Errorf("scan building extension: %w", err)
		}
		building.Extensions = append(building.Extensions, extension)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("read building extensions: %w", err)
	}
	return nil
}

func (s *Store) Gather(userID int64) (PlayerState, error) {
	if userID <= 0 {
		return PlayerState{}, fmt.Errorf("%w: user ID is required", ErrInvalidArgument)
	}
	tx, err := s.db.Begin()
	if err != nil {
		return PlayerState{}, fmt.Errorf("begin gather: %w", err)
	}
	now := s.now().UTC()
	cleanupEvents, err := normalizeItemHoldingsWithMetadataTx(tx, now, userID)
	if err != nil {
		_ = tx.Rollback()
		return PlayerState{}, err
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
	SELECT i.id, i.display_name, i.weight_units, i.max_durability_seconds, gr.quantity, gr.ap_cost
FROM gathering_rules gr
JOIN items i ON i.id = gr.item_id
WHERE gr.location_id = ?`, locationID).Scan(
		&option.Item.ID, &option.Item.DisplayName, &option.Item.WeightUnits, &option.Item.MaxDurabilitySeconds, &option.Quantity, &option.APCost,
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
	if err := addActiveItemHoldingTx(tx, "player_inventory", "user_id", userID, option.Item.ID, option.Quantity, now.Unix()+int64(option.Item.MaxDurabilitySeconds)); err != nil {
		_ = tx.Rollback()
		return PlayerState{}, fmt.Errorf("add gathered item: %w", err)
	}
	state, err := s.getPlayerStateTx(tx, userID, now)
	if err != nil {
		_ = tx.Rollback()
		return PlayerState{}, err
	}
	prependItemDurabilityCleanups(&state, cleanupEvents)
	if err := tx.Commit(); err != nil {
		return PlayerState{}, fmt.Errorf("commit gather: %w", err)
	}
	return state, nil
}

func (s *Store) Drop(userID int64, assetType, assetID string, quantity int, itemStatus string) (PlayerState, error) {
	if err := validateTransfer(userID, assetType, assetID, quantity); err != nil {
		return PlayerState{}, err
	}
	status, err := transferItemStatus(assetType, itemStatus)
	if err != nil {
		return PlayerState{}, err
	}
	if assetType == "resource" {
		return PlayerState{}, ErrResourceDropNotAllowed
	}
	tx, err := s.db.Begin()
	if err != nil {
		return PlayerState{}, fmt.Errorf("begin drop: %w", err)
	}
	now := s.now().UTC()
	cleanupEvents, err := normalizeItemHoldingsWithMetadataTx(tx, now, userID)
	if err != nil {
		_ = tx.Rollback()
		return PlayerState{}, err
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
		var expiresAt int64
		if err := tx.QueryRow(`SELECT status_expires_at FROM player_inventory WHERE user_id = ? AND item_id = ? AND durability_status = ?`, userID, assetID, status).Scan(&expiresAt); err != nil {
			_ = tx.Rollback()
			if errors.Is(err, sql.ErrNoRows) {
				return PlayerState{}, ErrInsufficientTransferAsset
			}
			return PlayerState{}, fmt.Errorf("get dropped item durability: %w", err)
		}
		if err := consumeItemHoldingTx(tx, "player_inventory", "user_id", userID, assetID, status, quantity, "dropped item"); err != nil {
			_ = tx.Rollback()
			return PlayerState{}, err
		}
		if err := addItemHoldingTx(tx, "ground_items", "location_id", locationID, assetID, status, quantity, expiresAt); err != nil {
			_ = tx.Rollback()
			return PlayerState{}, fmt.Errorf("add ground item: %w", err)
		}
	}
	state, err := s.getPlayerStateTx(tx, userID, now)
	if err != nil {
		_ = tx.Rollback()
		return PlayerState{}, err
	}
	prependItemDurabilityCleanups(&state, cleanupEvents)
	if err := tx.Commit(); err != nil {
		return PlayerState{}, fmt.Errorf("commit drop: %w", err)
	}
	return state, nil
}

func (s *Store) Pickup(userID int64, assetType, assetID string, quantity int, itemStatus string) (PlayerState, error) {
	if err := validateTransfer(userID, assetType, assetID, quantity); err != nil {
		return PlayerState{}, err
	}
	status, err := transferItemStatus(assetType, itemStatus)
	if err != nil {
		return PlayerState{}, err
	}
	if assetType == "item" && status == "expired" {
		return PlayerState{}, ErrInsufficientTransferAsset
	}
	tx, err := s.db.Begin()
	if err != nil {
		return PlayerState{}, fmt.Errorf("begin pickup: %w", err)
	}
	now := s.now().UTC()
	cleanupEvents, err := normalizeItemHoldingsWithMetadataTx(tx, now, userID)
	if err != nil {
		_ = tx.Rollback()
		return PlayerState{}, err
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
		var expiresAt int64
		if err := tx.QueryRow(`SELECT status_expires_at FROM ground_items WHERE location_id = ? AND item_id = ? AND durability_status = 'active'`, locationID, assetID).Scan(&expiresAt); err != nil {
			_ = tx.Rollback()
			if errors.Is(err, sql.ErrNoRows) {
				return PlayerState{}, ErrInsufficientTransferAsset
			}
			return PlayerState{}, fmt.Errorf("get picked item durability: %w", err)
		}
		if err := consumeItemHoldingTx(tx, "ground_items", "location_id", locationID, assetID, status, quantity, "ground item"); err != nil {
			_ = tx.Rollback()
			return PlayerState{}, err
		}
		if err := addItemHoldingTx(tx, "player_inventory", "user_id", userID, assetID, status, quantity, expiresAt); err != nil {
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
	state, err := s.getPlayerStateTx(tx, userID, now)
	if err != nil {
		_ = tx.Rollback()
		return PlayerState{}, err
	}
	prependItemDurabilityCleanups(&state, cleanupEvents)
	if err := tx.Commit(); err != nil {
		return PlayerState{}, fmt.Errorf("commit pickup: %w", err)
	}
	return state, nil
}

func transferItemStatus(assetType, itemStatus string) (string, error) {
	if assetType == "resource" {
		if itemStatus != "" {
			return "", fmt.Errorf("%w: resources do not accept item status", ErrInvalidArgument)
		}
		return "", nil
	}
	status := itemStatus
	if status != "active" && status != "expired" {
		return "", fmt.Errorf("%w: item status must be active or expired", ErrInvalidArgument)
	}
	return status, nil
}

func consumeItemHoldingTx(tx *sql.Tx, table, scopeColumn string, scopeValue any, itemID, status string, quantity int, label string) error {
	deleteQuery := fmt.Sprintf("DELETE FROM %s WHERE %s = ? AND item_id = ? AND durability_status = ? AND quantity = ?", table, scopeColumn)
	result, err := tx.Exec(deleteQuery, scopeValue, itemID, status, quantity)
	if err != nil {
		return fmt.Errorf("consume %s: %w", label, err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check %s: %w", label, err)
	}
	if rows == 0 {
		updateQuery := fmt.Sprintf("UPDATE %s SET quantity = quantity - ? WHERE %s = ? AND item_id = ? AND durability_status = ? AND quantity > ?", table, scopeColumn)
		result, err = tx.Exec(updateQuery, quantity, scopeValue, itemID, status, quantity)
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

func addActiveItemHoldingTx(tx *sql.Tx, table, scopeColumn string, scopeValue any, itemID string, quantity int, expiresAt int64) error {
	return addItemHoldingTx(tx, table, scopeColumn, scopeValue, itemID, "active", quantity, expiresAt)
}

func addItemHoldingTx(tx *sql.Tx, table, scopeColumn string, scopeValue any, itemID, status string, quantity int, expiresAt int64) error {
	var existingQuantity, existingExpiry int64
	query := fmt.Sprintf("SELECT quantity, status_expires_at FROM %s WHERE %s = ? AND item_id = ? AND durability_status = ?", table, scopeColumn)
	err := tx.QueryRow(query, scopeValue, itemID, status).Scan(&existingQuantity, &existingExpiry)
	if errors.Is(err, sql.ErrNoRows) {
		insertQuery := fmt.Sprintf("INSERT INTO %s (%s, item_id, durability_status, status_expires_at, quantity) VALUES (?, ?, ?, ?, ?)", table, scopeColumn)
		_, err = tx.Exec(insertQuery, scopeValue, itemID, status, expiresAt, quantity)
		return err
	}
	if err != nil {
		return err
	}
	newExpiry := expiresAt
	if status == "active" {
		incomingQuantity := int64(quantity)
		if existingQuantity <= 0 || incomingQuantity <= 0 || existingQuantity > math.MaxInt64-incomingQuantity {
			return fmt.Errorf("merge active item quantity overflow")
		}
		newExpiry = weightedActiveExpiry(existingQuantity, incomingQuantity, existingExpiry, expiresAt)
	} else if expiresAt < existingExpiry {
		newExpiry = existingExpiry
	}
	updateQuery := fmt.Sprintf("UPDATE %s SET quantity = quantity + ?, status_expires_at = ? WHERE %s = ? AND item_id = ? AND durability_status = ?", table, scopeColumn)
	_, err = tx.Exec(updateQuery, quantity, newExpiry, scopeValue, itemID, status)
	return err
}

func weightedActiveExpiry(existingQuantity, incomingQuantity, existingExpiry, incomingExpiry int64) int64 {
	totalQuantity := existingQuantity + incomingQuantity
	lowExpiry, highExpiry := existingExpiry, incomingExpiry
	lowQuantity, highQuantity := existingQuantity, incomingQuantity
	if lowExpiry > highExpiry {
		lowExpiry, highExpiry = highExpiry, lowExpiry
		lowQuantity, highQuantity = highQuantity, lowQuantity
	}
	if lowExpiry >= 0 {
		delta := uint64(highExpiry - lowExpiry)
		weightedDelta, _ := mulDivFloor(uint64(highQuantity), delta, uint64(totalQuantity))
		return lowExpiry + int64(weightedDelta)
	}
	if highExpiry <= 0 {
		delta := uint64(highExpiry - lowExpiry)
		weightedDelta, remainder := mulDivFloor(uint64(lowQuantity), delta, uint64(totalQuantity))
		if remainder != 0 {
			weightedDelta++
		}
		return highExpiry - int64(weightedDelta)
	}
	positivePart, positiveRemainder := mulDivFloor(uint64(highQuantity), uint64(highExpiry), uint64(totalQuantity))
	negativePart, negativeRemainder := mulDivFloor(uint64(lowQuantity), int64Magnitude(lowExpiry), uint64(totalQuantity))
	weightedExpiry := int64(positivePart) - int64(negativePart)
	if positiveRemainder < negativeRemainder {
		weightedExpiry--
	}
	return weightedExpiry
}

func mulDivFloor(multiplicand, multiplier, divisor uint64) (uint64, uint64) {
	high, low := bits.Mul64(multiplicand, multiplier)
	return bits.Div64(high, low, divisor)
}

func int64Magnitude(value int64) uint64 {
	if value >= 0 {
		return uint64(value)
	}
	return uint64(-(value + 1)) + 1
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
	state, _, err := s.MoveWithCombat(userID, targetID)
	return state, err
}

func (s *Store) MoveWithCombat(userID int64, targetID string) (PlayerState, CombatResult, error) {
	if userID <= 0 {
		return PlayerState{}, CombatResult{}, fmt.Errorf("%w: user ID is required", ErrInvalidArgument)
	}
	if strings.TrimSpace(targetID) == "" {
		return PlayerState{}, CombatResult{}, fmt.Errorf("%w: target is required", ErrInvalidArgument)
	}
	tx, err := s.db.Begin()
	if err != nil {
		return PlayerState{}, CombatResult{}, fmt.Errorf("begin move: %w", err)
	}
	now := s.now().UTC()
	cleanupEvents, err := normalizeItemHoldingsWithMetadataTx(tx, now, userID)
	if err != nil {
		_ = tx.Rollback()
		return PlayerState{}, CombatResult{}, err
	}
	var originID string
	err = tx.QueryRow(`SELECT location_id FROM player_locations WHERE user_id = ?`, userID).Scan(&originID)
	if errors.Is(err, sql.ErrNoRows) {
		_ = tx.Rollback()
		return PlayerState{}, CombatResult{}, ErrIdentityNotFound
	}
	if err != nil {
		_ = tx.Rollback()
		return PlayerState{}, CombatResult{}, fmt.Errorf("get player location for move: %w", err)
	}
	var route Route
	err = tx.QueryRow(`
SELECT origin_id, destination_id, ap_cost
FROM routes
WHERE origin_id = ? AND destination_id = ?`, originID, targetID).Scan(&route.OriginID, &route.DestinationID, &route.APCost)
	if errors.Is(err, sql.ErrNoRows) {
		_ = tx.Rollback()
		return PlayerState{}, CombatResult{}, ErrRouteNotFound
	}
	if err != nil {
		_ = tx.Rollback()
		return PlayerState{}, CombatResult{}, fmt.Errorf("get route for move: %w", err)
	}
	var fullTimestamp int64
	err = tx.QueryRow(`SELECT full_timestamp FROM player_ap WHERE user_id = ?`, userID).Scan(&fullTimestamp)
	if errors.Is(err, sql.ErrNoRows) {
		_ = tx.Rollback()
		return PlayerState{}, CombatResult{}, ErrIdentityNotFound
	}
	if err != nil {
		_ = tx.Rollback()
		return PlayerState{}, CombatResult{}, fmt.Errorf("get player AP for move: %w", err)
	}
	if calculateAP(unixSeconds(fullTimestamp), now) < route.APCost {
		_ = tx.Rollback()
		return PlayerState{}, CombatResult{}, ErrInsufficientAP
	}
	carriedWeight, err := carryingWeightTx(tx, userID)
	if err != nil {
		_ = tx.Rollback()
		return PlayerState{}, CombatResult{}, err
	}
	if carriedWeight > movementWeightThreshold {
		_ = tx.Rollback()
		return PlayerState{}, CombatResult{}, ErrOverweight
	}
	monsterState, err := settleLocationMonstersTx(tx, originID, now, s.monsterRoll)
	if err != nil {
		_ = tx.Rollback()
		return PlayerState{}, CombatResult{}, err
	}
	var interceptChanceBPS int
	err = tx.QueryRow(`SELECT intercept_chance_bps FROM location_monster_rules WHERE location_id = ?`, originID).Scan(&interceptChanceBPS)
	if errors.Is(err, sql.ErrNoRows) {
		interceptChanceBPS = 0
	} else if err != nil {
		_ = tx.Rollback()
		return PlayerState{}, CombatResult{}, fmt.Errorf("get move interception rule: %w", err)
	}
	combinedChanceBPS := combinedInterceptionChanceBPS(interceptChanceBPS, monsterState.MonsterCount)
	interception := &MonsterInterceptionComputation{LocationID: originID, MonsterCount: monsterState.MonsterCount, PerMonsterChanceBPS: interceptChanceBPS, CombinedChanceBPS: combinedChanceBPS, Outcome: "not_intercepted"}
	if combinedChanceBPS > 0 && normalizeCombatRoll(s.combatRandom()) < combinedChanceBPS {
		interception.Outcome = "intercepted"
		combat, combatErr := s.resolveCombatTx(tx, userID, originID, now)
		if combatErr != nil {
			_ = tx.Rollback()
			return PlayerState{}, CombatResult{}, combatErr
		}
		state, stateErr := s.getPlayerStateTx(tx, userID, now)
		if stateErr != nil {
			_ = tx.Rollback()
			return PlayerState{}, CombatResult{}, stateErr
		}
		state.MonsterInterception = interception
		prependItemDurabilityCleanups(&state, cleanupEvents)
		if err := tx.Commit(); err != nil {
			return PlayerState{}, CombatResult{}, fmt.Errorf("commit intercepted move: %w", err)
		}
		return state, combat, nil
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
		return PlayerState{}, CombatResult{}, fmt.Errorf("consume AP for move: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		_ = tx.Rollback()
		return PlayerState{}, CombatResult{}, fmt.Errorf("check move AP: %w", err)
	}
	if rows != 1 {
		_ = tx.Rollback()
		return PlayerState{}, CombatResult{}, ErrInsufficientAP
	}
	result, err = tx.Exec(`
UPDATE player_locations SET location_id = ?
WHERE user_id = ? AND location_id = ?`, route.DestinationID, userID, originID)
	if err != nil {
		_ = tx.Rollback()
		return PlayerState{}, CombatResult{}, fmt.Errorf("update player location: %w", err)
	}
	rows, err = result.RowsAffected()
	if err != nil {
		_ = tx.Rollback()
		return PlayerState{}, CombatResult{}, fmt.Errorf("check player location update: %w", err)
	}
	if rows != 1 {
		_ = tx.Rollback()
		return PlayerState{}, CombatResult{}, ErrRouteNotFound
	}
	state, err := s.getPlayerStateTx(tx, userID, now)
	if err != nil {
		_ = tx.Rollback()
		return PlayerState{}, CombatResult{}, err
	}
	state.MonsterInterception = interception
	prependItemDurabilityCleanups(&state, cleanupEvents)
	if err := tx.Commit(); err != nil {
		return PlayerState{}, CombatResult{}, fmt.Errorf("commit move: %w", err)
	}
	return state, CombatResult{}, nil
}

func conversionMethodForID(tx *sql.Tx, methodID string) (ConversionMethod, error) {
	var method ConversionMethod
	var essenceID, essenceDisplay sql.NullString
	var essenceWeight, essenceDurability sql.NullInt64
	err := tx.QueryRow(`
SELECT cm.id, cm.display_name, cm.ap_cost, ii.id, ii.display_name, ii.weight_units, ii.max_durability_seconds,
       cm.max_input_quantity, output.id, output.display_name, cm.resource_quantity_per_input,
       essence.id, essence.display_name, essence.weight_units, essence.max_durability_seconds,
       cm.essence_chance_bps, cm.essence_quantity
FROM conversion_methods cm JOIN items ii ON ii.id = cm.input_item_id
JOIN resource_types output ON output.id = cm.output_resource_id
LEFT JOIN items essence ON essence.id = cm.essence_item_id WHERE cm.id = ?`, methodID).Scan(
		&method.ID, &method.DisplayName, &method.APCost, &method.Input.ID, &method.Input.DisplayName,
		&method.Input.WeightUnits, &method.Input.MaxDurabilitySeconds, &method.MaxInputQuantity,
		&method.OutputResource.ID, &method.OutputResource.DisplayName, &method.ResourceQuantityPerInput,
		&essenceID, &essenceDisplay, &essenceWeight, &essenceDurability, &method.EssenceChanceBPS, &method.EssenceQuantity)
	if err != nil {
		return ConversionMethod{}, err
	}
	if essenceID.Valid {
		method.EssenceItem = &Item{ID: essenceID.String, DisplayName: essenceDisplay.String, WeightUnits: int(essenceWeight.Int64), MaxDurabilitySeconds: int(essenceDurability.Int64)}
	}
	return method, nil
}

// Convert accepts the legacy no-argument call and the methodID, quantity, providerExtensionID form.
func (s *Store) Convert(userID int64, params ...interface{}) (PlayerState, error) {
	if userID <= 0 {
		return PlayerState{}, fmt.Errorf("%w: user ID is required", ErrInvalidArgument)
	}
	tx, err := s.db.Begin()
	if err != nil {
		return PlayerState{}, fmt.Errorf("begin convert: %w", err)
	}
	now := s.now().UTC()
	cleanupEvents, err := normalizeItemHoldingsWithMetadataTx(tx, now, userID)
	if err != nil {
		_ = tx.Rollback()
		return PlayerState{}, err
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
	methodID, quantity, providerID := "hand_wood_t1", 1, int64(0)
	providerIDProvided := false
	legacy := len(params) == 0
	durabilityCost := 0
	var legacyOption ConversionOption
	if legacy {
		legacyOption, err = conversionOptionForLocation(tx, locationID)
		if errors.Is(err, sql.ErrNoRows) {
			_ = tx.Rollback()
			return PlayerState{}, ErrConversionNotFound
		}
		if err != nil {
			_ = tx.Rollback()
			return PlayerState{}, fmt.Errorf("get conversion rule: %w", err)
		}
		methodID, quantity = "legacy", legacyOption.InputQuantity
	}
	if len(params) > 0 {
		if len(params) < 2 {
			_ = tx.Rollback()
			return PlayerState{}, fmt.Errorf("%w: method and quantity are required", ErrInvalidArgument)
		}
		var ok bool
		methodID, ok = params[0].(string)
		if !ok {
			_ = tx.Rollback()
			return PlayerState{}, fmt.Errorf("%w: method ID is required", ErrInvalidArgument)
		}
		quantity, ok = params[1].(int)
		if !ok || quantity <= 0 {
			_ = tx.Rollback()
			return PlayerState{}, fmt.Errorf("%w: quantity must be positive", ErrInvalidArgument)
		}
		if len(params) > 2 && params[2] != nil {
			providerIDProvided = true
			switch v := params[2].(type) {
			case int64:
				providerID = v
			case int:
				providerID = int64(v)
			default:
				_ = tx.Rollback()
				return PlayerState{}, fmt.Errorf("%w: provider extension ID is invalid", ErrInvalidArgument)
			}
		}
	}
	method, err := conversionMethodForID(tx, methodID)
	if legacy {
		method = ConversionMethod{ID: methodID, APCost: legacyOption.APCost, Input: legacyOption.Item, MaxInputQuantity: legacyOption.InputQuantity, OutputResource: legacyOption.Resource, ResourceQuantityPerInput: legacyOption.ResourceYield}
		err = nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		_ = tx.Rollback()
		return PlayerState{}, ErrConversionNotFound
	}
	if err != nil {
		_ = tx.Rollback()
		return PlayerState{}, fmt.Errorf("get conversion method: %w", err)
	}
	if quantity > method.MaxInputQuantity {
		_ = tx.Rollback()
		return PlayerState{}, fmt.Errorf("%w: quantity exceeds method capacity", ErrInvalidArgument)
	}
	isGlobalMethod := false
	if !legacy {
		err = tx.QueryRow(`SELECT EXISTS (SELECT 1 FROM global_conversion_methods WHERE conversion_method_id = ?)`, methodID).Scan(&isGlobalMethod)
		if err != nil {
			_ = tx.Rollback()
			return PlayerState{}, fmt.Errorf("check global conversion method: %w", err)
		}
	}
	if !legacy && isGlobalMethod && providerIDProvided {
		_ = tx.Rollback()
		return PlayerState{}, fmt.Errorf("%w: global conversion methods do not accept provider extensions", ErrInvalidArgument)
	}
	if !legacy && !isGlobalMethod {
		if providerID <= 0 {
			_ = tx.Rollback()
			return PlayerState{}, ErrExtensionNotFound
		}
		var capability, buildingLocation, buildingStatus string
		var expiry sql.NullInt64
		err = tx.QueryRow(`SELECT c.conversion_method_id, b.location_id, b.status, b.durability_expires_at, c.building_durability_cost_seconds FROM building_extensions e JOIN extension_conversion_capabilities c ON c.extension_definition_id=e.definition_id AND c.conversion_method_id=? JOIN buildings b ON b.id=e.building_id WHERE e.id=? AND e.status='completed'`, methodID, providerID).Scan(&capability, &buildingLocation, &buildingStatus, &expiry, &durabilityCost)
		if errors.Is(err, sql.ErrNoRows) {
			_ = tx.Rollback()
			return PlayerState{}, ErrExtensionNotFound
		}
		if err != nil {
			_ = tx.Rollback()
			return PlayerState{}, fmt.Errorf("get conversion provider: %w", err)
		}
		if buildingLocation != locationID {
			_ = tx.Rollback()
			return PlayerState{}, ErrBuildingRemote
		}
		if buildingStatus != "completed" || !expiry.Valid || expiry.Int64 <= now.Unix() {
			_ = tx.Rollback()
			return PlayerState{}, ErrBuildingDisabled
		}
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
	err = tx.QueryRow(`SELECT quantity FROM player_inventory WHERE user_id = ? AND item_id = ? AND durability_status = 'active'`, userID, method.Input.ID).Scan(&itemQuantity)
	if errors.Is(err, sql.ErrNoRows) || itemQuantity < quantity {
		_ = tx.Rollback()
		return PlayerState{}, ErrInsufficientItem
	}
	if err != nil {
		_ = tx.Rollback()
		return PlayerState{}, fmt.Errorf("get conversion item: %w", err)
	}
	if calculateAP(unixSeconds(fullTimestamp), now) < method.APCost {
		_ = tx.Rollback()
		return PlayerState{}, ErrInsufficientAP
	}
	fullAt := unixSeconds(fullTimestamp)
	if fullAt.Before(now) {
		fullAt = now
	}
	nextFullTimestamp := fullAt.Add(time.Duration(method.APCost) * apRecoveryTime).Unix()
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
	if itemQuantity == quantity {
		_, err = tx.Exec(`DELETE FROM player_inventory WHERE user_id = ? AND item_id = ? AND durability_status = 'active'`, userID, method.Input.ID)
	} else {
		_, err = tx.Exec(`UPDATE player_inventory SET quantity = quantity - ? WHERE user_id = ? AND item_id = ? AND durability_status = 'active'`, quantity, userID, method.Input.ID)
	}
	if err != nil {
		_ = tx.Rollback()
		return PlayerState{}, fmt.Errorf("consume conversion item: %w", err)
	}
	_, err = tx.Exec(`
INSERT INTO player_resources (user_id, resource_id, quantity)
	VALUES (?, ?, ?)
	ON CONFLICT (user_id, resource_id) DO UPDATE SET quantity = player_resources.quantity + excluded.quantity`, userID, method.OutputResource.ID, quantity*method.ResourceQuantityPerInput)
	if err != nil {
		_ = tx.Rollback()
		return PlayerState{}, fmt.Errorf("add conversion resources: %w", err)
	}
	if method.EssenceItem != nil && s.essenceRoll != nil {
		essenceCount := 0
		for i := 0; i < quantity; i++ {
			if s.essenceRoll() < method.EssenceChanceBPS {
				essenceCount += method.EssenceQuantity
			}
		}
		if essenceCount > 0 {
			if err := addActiveItemHoldingTx(tx, "player_inventory", "user_id", userID, method.EssenceItem.ID, essenceCount, now.Unix()+int64(method.EssenceItem.MaxDurabilitySeconds)); err != nil {
				_ = tx.Rollback()
				return PlayerState{}, err
			}
		}
	}
	if !legacy && !isGlobalMethod {
		result, err := tx.Exec(`UPDATE buildings SET durability_expires_at = CASE WHEN durability_expires_at - ? > ? THEN durability_expires_at - ? ELSE ? END WHERE id = (SELECT building_id FROM building_extensions WHERE id = ?) AND durability_expires_at > ?`, durabilityCost, now.Unix(), durabilityCost, now.Unix(), providerID, now.Unix())
		if err != nil {
			_ = tx.Rollback()
			return PlayerState{}, fmt.Errorf("consume building durability: %w", err)
		}
		if rows, _ := result.RowsAffected(); rows != 1 {
			_ = tx.Rollback()
			return PlayerState{}, ErrBuildingDisabled
		}
	}
	state, err := s.getPlayerStateTx(tx, userID, now)
	if err != nil {
		_ = tx.Rollback()
		return PlayerState{}, err
	}
	prependItemDurabilityCleanups(&state, cleanupEvents)
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
	now := s.now().UTC()
	cleanupEvents, err := normalizeItemHoldingsWithMetadataTx(tx, now, userID)
	if err != nil {
		_ = tx.Rollback()
		return PlayerState{}, err
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
		err := tx.QueryRow(`SELECT quantity FROM player_inventory WHERE user_id = ? AND item_id = ? AND durability_status = 'active'`, userID, input.Item.ID).Scan(&quantity)
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
			result, err = tx.Exec(`DELETE FROM player_inventory WHERE user_id = ? AND item_id = ? AND durability_status = 'active'`, userID, input.Item.ID)
		} else {
			result, err = tx.Exec(`UPDATE player_inventory SET quantity = quantity - ? WHERE user_id = ? AND item_id = ? AND durability_status = 'active'`, input.Quantity, userID, input.Item.ID)
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
	if err := addActiveItemHoldingTx(tx, "player_inventory", "user_id", userID, recipe.Output.ID, recipe.OutputQuantity, now.Unix()+int64(recipe.Output.MaxDurabilitySeconds)); err != nil {
		_ = tx.Rollback()
		return PlayerState{}, fmt.Errorf("add crafted item: %w", err)
	}
	state, err := s.getPlayerStateTx(tx, userID, now)
	if err != nil {
		_ = tx.Rollback()
		return PlayerState{}, err
	}
	prependItemDurabilityCleanups(&state, cleanupEvents)
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
	cleanupEvents, err := normalizeItemHoldingsWithMetadataTx(tx, now, userID)
	if err != nil {
		_ = tx.Rollback()
		return PlayerState{}, err
	}
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
		err := tx.QueryRow(`SELECT quantity FROM player_inventory WHERE user_id = ? AND item_id = ? AND durability_status = 'active'`, userID, input.Item.ID).Scan(&quantity)
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
			result, err = tx.Exec(`DELETE FROM player_inventory WHERE user_id = ? AND item_id = ? AND durability_status = 'active'`, userID, input.Item.ID)
		} else {
			result, err = tx.Exec(`UPDATE player_inventory SET quantity = quantity - ? WHERE user_id = ? AND item_id = ? AND durability_status = 'active'`, input.Quantity, userID, input.Item.ID)
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
	prependItemDurabilityCleanups(&state, cleanupEvents)
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
	cleanupEvents, err := normalizeItemHoldingsWithMetadataTx(tx, now, userID)
	if err != nil {
		_ = tx.Rollback()
		return PlayerState{}, err
	}
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
	prependItemDurabilityCleanups(&state, cleanupEvents)
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
	cleanupEvents, err := normalizeItemHoldingsWithMetadataTx(tx, now, userID)
	if err != nil {
		_ = tx.Rollback()
		return PlayerState{}, err
	}
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
	prependItemDurabilityCleanups(&state, cleanupEvents)
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

func (s *Store) InstallExtension(userID, buildingID int64, slotIndex int, definitionID string) (PlayerState, error) {
	if userID <= 0 || buildingID <= 0 || slotIndex < 0 || strings.TrimSpace(definitionID) == "" {
		return PlayerState{}, fmt.Errorf("%w: user ID, building ID, slot index, and definition ID are required", ErrInvalidArgument)
	}
	tx, err := s.db.Begin()
	if err != nil {
		return PlayerState{}, fmt.Errorf("begin extension installation: %w", err)
	}
	now := s.now().UTC()
	cleanupEvents, err := normalizeItemHoldingsWithMetadataTx(tx, now, userID)
	if err != nil {
		_ = tx.Rollback()
		return PlayerState{}, err
	}
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
		return PlayerState{}, fmt.Errorf("get extension installer location: %w", err)
	}
	var ownerID int64
	var buildingLocation, buildingStatus string
	var durabilityExpiresAt sql.NullInt64
	var slotCount int
	err = tx.QueryRow(`
SELECT owner_id, location_id, status, extension_slot_count, durability_expires_at
FROM buildings WHERE id = ?`, buildingID).Scan(&ownerID, &buildingLocation, &buildingStatus, &slotCount, &durabilityExpiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		_ = tx.Rollback()
		return PlayerState{}, ErrBuildingNotFound
	}
	if err != nil {
		_ = tx.Rollback()
		return PlayerState{}, fmt.Errorf("get extension building: %w", err)
	}
	if ownerID != userID {
		_ = tx.Rollback()
		return PlayerState{}, ErrBuildingNotOwner
	}
	if playerLocation != buildingLocation {
		_ = tx.Rollback()
		return PlayerState{}, ErrBuildingRemote
	}
	if buildingStatus != "completed" {
		_ = tx.Rollback()
		return PlayerState{}, ErrBuildingUnderConstruction
	}
	if !durabilityExpiresAt.Valid || durabilityExpiresAt.Int64 <= now.Unix() {
		_ = tx.Rollback()
		return PlayerState{}, ErrBuildingDisabled
	}
	if slotIndex >= slotCount {
		_ = tx.Rollback()
		return PlayerState{}, fmt.Errorf("%w: slot index is outside building capacity", ErrInvalidArgument)
	}
	var definition BuildingExtensionDefinition
	err = tx.QueryRow(`
SELECT ed.id, ed.display_name, ed.tier, ed.package_item_id, i.display_name, i.weight_units, i.max_durability_seconds, ed.required_ap
FROM building_extension_definitions ed
JOIN items i ON i.id = ed.package_item_id
WHERE ed.id = ?`, definitionID).Scan(
		&definition.ID, &definition.DisplayName, &definition.Tier, &definition.PackageItem.ID,
		&definition.PackageItem.DisplayName, &definition.PackageItem.WeightUnits,
		&definition.PackageItem.MaxDurabilitySeconds, &definition.RequiredAP,
	)
	if errors.Is(err, sql.ErrNoRows) {
		_ = tx.Rollback()
		return PlayerState{}, ErrExtensionDefinitionNotFound
	}
	if err != nil {
		_ = tx.Rollback()
		return PlayerState{}, fmt.Errorf("get extension definition: %w", err)
	}
	var occupied int
	err = tx.QueryRow(`SELECT 1 FROM building_extensions WHERE building_id = ? AND slot_index = ?`, buildingID, slotIndex).Scan(&occupied)
	if err == nil {
		_ = tx.Rollback()
		return PlayerState{}, ErrExtensionOccupied
	}
	if !errors.Is(err, sql.ErrNoRows) {
		_ = tx.Rollback()
		return PlayerState{}, fmt.Errorf("check extension slot: %w", err)
	}
	var packageQuantity int
	err = tx.QueryRow(`SELECT quantity FROM player_inventory WHERE user_id = ? AND item_id = ? AND durability_status = 'active'`, userID, definition.PackageItem.ID).Scan(&packageQuantity)
	if errors.Is(err, sql.ErrNoRows) || packageQuantity < 1 {
		_ = tx.Rollback()
		return PlayerState{}, ErrInsufficientItem
	}
	if err != nil {
		_ = tx.Rollback()
		return PlayerState{}, fmt.Errorf("get extension package: %w", err)
	}
	var result sql.Result
	if packageQuantity == 1 {
		result, err = tx.Exec(`DELETE FROM player_inventory WHERE user_id = ? AND item_id = ? AND durability_status = 'active'`, userID, definition.PackageItem.ID)
	} else {
		result, err = tx.Exec(`UPDATE player_inventory SET quantity = quantity - 1 WHERE user_id = ? AND item_id = ? AND durability_status = 'active' AND quantity >= 1`, userID, definition.PackageItem.ID)
	}
	if err != nil {
		_ = tx.Rollback()
		return PlayerState{}, fmt.Errorf("consume extension package: %w", err)
	}
	if rows, rowsErr := result.RowsAffected(); rowsErr != nil || rows != 1 {
		_ = tx.Rollback()
		if rowsErr != nil {
			return PlayerState{}, fmt.Errorf("check extension package: %w", rowsErr)
		}
		return PlayerState{}, ErrInsufficientItem
	}
	if _, err := tx.Exec(`
INSERT INTO building_extensions (building_id, slot_index, definition_id, display_name, tier, required_ap, contributed_ap, status)
VALUES (?, ?, ?, ?, ?, ?, 0, 'under_construction')`, buildingID, slotIndex, definition.ID, definition.DisplayName, definition.Tier, definition.RequiredAP); err != nil {
		_ = tx.Rollback()
		return PlayerState{}, fmt.Errorf("create building extension: %w", err)
	}
	state, err := s.getPlayerStateTx(tx, userID, now)
	if err != nil {
		_ = tx.Rollback()
		return PlayerState{}, err
	}
	prependItemDurabilityCleanups(&state, cleanupEvents)
	if err := tx.Commit(); err != nil {
		return PlayerState{}, fmt.Errorf("commit extension installation: %w", err)
	}
	return state, nil
}

func (s *Store) ContributeExtensionConstruction(userID, extensionID int64, requestedAP int) (PlayerState, error) {
	if userID <= 0 || extensionID <= 0 || requestedAP <= 0 {
		return PlayerState{}, fmt.Errorf("%w: user ID, extension ID, and AP are required", ErrInvalidArgument)
	}
	computation := &ExtensionConstructionComputation{ExtensionID: extensionID, RequestedAP: requestedAP}
	failure := func(err error) (PlayerState, error) {
		return PlayerState{ExtensionConstructionComputation: computation}, err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return failure(fmt.Errorf("begin extension construction contribution: %w", err))
	}
	now := s.now().UTC()
	cleanupEvents, err := normalizeItemHoldingsWithMetadataTx(tx, now, userID)
	if err != nil {
		_ = tx.Rollback()
		return failure(err)
	}
	if err := deleteDestroyedBuildingsTx(tx, now); err != nil {
		_ = tx.Rollback()
		return failure(err)
	}
	var contributorLocation string
	if err := tx.QueryRow(`SELECT location_id FROM player_locations WHERE user_id = ?`, userID).Scan(&contributorLocation); errors.Is(err, sql.ErrNoRows) {
		_ = tx.Rollback()
		return failure(ErrIdentityNotFound)
	} else if err != nil {
		_ = tx.Rollback()
		return failure(fmt.Errorf("get extension contributor location: %w", err))
	}
	var buildingID int64
	var buildingLocation, buildingStatus, extensionStatus string
	var durabilityExpiresAt sql.NullInt64
	var contributedAP, requiredAP int
	err = tx.QueryRow(`
SELECT b.id, b.location_id, b.status, b.durability_expires_at, e.status, e.contributed_ap, e.required_ap
FROM building_extensions e
JOIN buildings b ON b.id = e.building_id
WHERE e.id = ?`, extensionID).Scan(&buildingID, &buildingLocation, &buildingStatus, &durabilityExpiresAt, &extensionStatus, &contributedAP, &requiredAP)
	if errors.Is(err, sql.ErrNoRows) {
		_ = tx.Rollback()
		return failure(ErrExtensionNotFound)
	}
	if err != nil {
		_ = tx.Rollback()
		return failure(fmt.Errorf("get extension construction target: %w", err))
	}
	computation.BuildingID = buildingID
	computation.ResultingProgress = contributedAP
	computation.RequiredAP = requiredAP
	computation.ResultingStatus = extensionStatus
	if extensionStatus == "completed" {
		_ = tx.Rollback()
		return failure(ErrExtensionCompleted)
	}
	if extensionStatus != "under_construction" {
		_ = tx.Rollback()
		return failure(ErrExtensionNotFound)
	}
	if contributorLocation != buildingLocation {
		_ = tx.Rollback()
		return failure(ErrBuildingRemote)
	}
	if buildingStatus != "completed" {
		_ = tx.Rollback()
		return failure(ErrBuildingUnderConstruction)
	}
	if !durabilityExpiresAt.Valid || durabilityExpiresAt.Int64 <= now.Unix() {
		_ = tx.Rollback()
		return failure(ErrBuildingDisabled)
	}
	remainingAP := requiredAP - contributedAP
	effectiveAP := requestedAP
	if effectiveAP > remainingAP {
		effectiveAP = remainingAP
	}
	var fullTimestamp int64
	if err := tx.QueryRow(`SELECT full_timestamp FROM player_ap WHERE user_id = ?`, userID).Scan(&fullTimestamp); errors.Is(err, sql.ErrNoRows) {
		_ = tx.Rollback()
		return failure(ErrIdentityNotFound)
	} else if err != nil {
		_ = tx.Rollback()
		return failure(fmt.Errorf("get extension contributor AP: %w", err))
	}
	if calculateAP(unixSeconds(fullTimestamp), now) < effectiveAP {
		_ = tx.Rollback()
		return failure(ErrInsufficientAP)
	}
	fullAt := unixSeconds(fullTimestamp)
	if fullAt.Before(now) {
		fullAt = now
	}
	result, err := tx.Exec(`UPDATE player_ap SET full_timestamp = ? WHERE user_id = ? AND full_timestamp = ?`, fullAt.Add(time.Duration(effectiveAP)*apRecoveryTime).Unix(), userID, fullTimestamp)
	if err != nil {
		_ = tx.Rollback()
		return failure(fmt.Errorf("consume extension construction AP: %w", err))
	}
	if rows, rowsErr := result.RowsAffected(); rowsErr != nil || rows != 1 {
		_ = tx.Rollback()
		if rowsErr != nil {
			return failure(fmt.Errorf("check extension construction AP: %w", rowsErr))
		}
		return failure(ErrInsufficientAP)
	}
	newProgress := contributedAP + effectiveAP
	newStatus := "under_construction"
	if newProgress == requiredAP {
		newStatus = "completed"
	}
	result, err = tx.Exec(`
UPDATE building_extensions SET contributed_ap = ?, status = ?
WHERE id = ? AND status = 'under_construction' AND contributed_ap = ?`, newProgress, newStatus, extensionID, contributedAP)
	if err != nil {
		_ = tx.Rollback()
		return failure(fmt.Errorf("update extension construction progress: %w", err))
	}
	if rows, rowsErr := result.RowsAffected(); rowsErr != nil || rows != 1 {
		_ = tx.Rollback()
		if rowsErr != nil {
			return failure(fmt.Errorf("check extension construction progress: %w", rowsErr))
		}
		return failure(ErrExtensionCompleted)
	}
	computation.EffectiveAP = effectiveAP
	computation.ResultingProgress = newProgress
	computation.ResultingStatus = newStatus
	state, err := s.getPlayerStateTx(tx, userID, now)
	if err != nil {
		_ = tx.Rollback()
		return failure(err)
	}
	prependItemDurabilityCleanups(&state, cleanupEvents)
	if err := tx.Commit(); err != nil {
		return failure(fmt.Errorf("commit extension construction contribution: %w", err))
	}
	state.ExtensionConstructionComputation = computation
	return state, nil
}

func (s *Store) RemoveExtension(userID, extensionID int64) (PlayerState, error) {
	if userID <= 0 || extensionID <= 0 {
		return PlayerState{}, fmt.Errorf("%w: user ID and extension ID are required", ErrInvalidArgument)
	}
	tx, err := s.db.Begin()
	if err != nil {
		return PlayerState{}, fmt.Errorf("begin extension removal: %w", err)
	}
	now := s.now().UTC()
	cleanupEvents, err := normalizeItemHoldingsWithMetadataTx(tx, now, userID)
	if err != nil {
		_ = tx.Rollback()
		return PlayerState{}, err
	}
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
		return PlayerState{}, fmt.Errorf("get extension remover location: %w", err)
	}
	var ownerID int64
	var buildingLocation string
	err = tx.QueryRow(`
SELECT b.owner_id, b.location_id
FROM building_extensions e
JOIN buildings b ON b.id = e.building_id
WHERE e.id = ?`, extensionID).Scan(&ownerID, &buildingLocation)
	if errors.Is(err, sql.ErrNoRows) {
		_ = tx.Rollback()
		return PlayerState{}, ErrExtensionNotFound
	}
	if err != nil {
		_ = tx.Rollback()
		return PlayerState{}, fmt.Errorf("get extension removal target: %w", err)
	}
	if ownerID != userID {
		_ = tx.Rollback()
		return PlayerState{}, ErrBuildingNotOwner
	}
	if playerLocation != buildingLocation {
		_ = tx.Rollback()
		return PlayerState{}, ErrBuildingRemote
	}
	result, err := tx.Exec(`DELETE FROM building_extensions WHERE id = ?`, extensionID)
	if err != nil {
		_ = tx.Rollback()
		return PlayerState{}, fmt.Errorf("remove building extension: %w", err)
	}
	if rows, rowsErr := result.RowsAffected(); rowsErr != nil || rows != 1 {
		_ = tx.Rollback()
		if rowsErr != nil {
			return PlayerState{}, fmt.Errorf("check extension removal: %w", rowsErr)
		}
		return PlayerState{}, ErrExtensionNotFound
	}
	state, err := s.getPlayerStateTx(tx, userID, now)
	if err != nil {
		_ = tx.Rollback()
		return PlayerState{}, err
	}
	prependItemDurabilityCleanups(&state, cleanupEvents)
	if err := tx.Commit(); err != nil {
		return PlayerState{}, fmt.Errorf("commit extension removal: %w", err)
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

func calculateHP(fullTimestamp, now time.Time, maxHP, recoveryIntervalSeconds int) int {
	if maxHP <= 0 || recoveryIntervalSeconds <= 0 {
		return 1
	}
	remaining := fullTimestamp.Sub(now)
	if remaining <= 0 {
		return maxHP
	}
	interval := time.Duration(recoveryIntervalSeconds) * time.Second
	missing := remaining / interval
	if remaining%interval != 0 {
		missing++
	}
	if missing >= time.Duration(maxHP-1) {
		return 1
	}
	return maxHP - int(missing)
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
