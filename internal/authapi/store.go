package authapi

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

var (
	ErrInvalidArgument      = errors.New("invalid argument")
	ErrIdentityNotFound     = errors.New("identity not found")
	ErrOAuthAttemptNotFound = errors.New("oauth attempt not found")
	ErrOAuthAttemptExpired  = errors.New("oauth attempt expired")
	ErrOAuthAttemptConsumed = errors.New("oauth attempt already consumed")
	ErrSessionNotFound      = errors.New("session not found")
	ErrSessionExpired       = errors.New("session expired")
	ErrInsufficientAP       = errors.New("insufficient action points")
	ErrRouteNotFound        = errors.New("route not found")
	ErrGatheringNotFound    = errors.New("gathering not found")
	ErrConversionNotFound   = errors.New("conversion not found")
	ErrInsufficientItem     = errors.New("insufficient item")
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
	InputQuantity int
	ResourceYield int
	APCost        int
}

type PlayerState struct {
	Location         Location
	Routes           []Route
	AP               int
	Inventory        []InventoryItem
	GatheringOption  *GatheringOption
	ConversionOption *ConversionOption
	Resource         int
}

const (
	maxAP          = 3000
	apRecoveryTime = time.Minute
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
CREATE TABLE IF NOT EXISTS conversion_rules (
	location_id TEXT PRIMARY KEY REFERENCES locations(id),
	input_item_id TEXT NOT NULL REFERENCES items(id),
	input_quantity INTEGER NOT NULL CHECK (input_quantity > 0),
	resource_yield INTEGER NOT NULL CHECK (resource_yield > 0),
	ap_cost INTEGER NOT NULL CHECK (ap_cost > 0)
);
CREATE TABLE IF NOT EXISTS player_resources (
	user_id INTEGER PRIMARY KEY REFERENCES identities(id),
	balance INTEGER NOT NULL CHECK (balance >= 0)
);`); err != nil {
		_ = tx.Rollback()
		return nil, fmt.Errorf("initialize auth store: %w", err)
	}
	if _, err := tx.Exec(`
INSERT OR IGNORE INTO locations (id, display_name) VALUES
	('camp', 'Camp'),
	('forest_edge', 'Forest Edge');
INSERT OR IGNORE INTO routes (origin_id, destination_id, ap_cost) VALUES
	('camp', 'forest_edge', 20),
	('forest_edge', 'camp', 20);
INSERT OR IGNORE INTO items (id, display_name) VALUES
	('wood', 'Wood');
INSERT OR IGNORE INTO gathering_rules (location_id, item_id, quantity, ap_cost) VALUES
	('forest_edge', 'wood', 1, 10);
INSERT OR IGNORE INTO conversion_rules (location_id, input_item_id, input_quantity, resource_yield, ap_cost) VALUES
	('camp', 'wood', 1, 1, 1);`); err != nil {
		_ = tx.Rollback()
		return nil, fmt.Errorf("seed movement state: %w", err)
	}
	if _, err := tx.Exec(`
INSERT OR IGNORE INTO player_ap (user_id, full_timestamp)
SELECT id, ? FROM identities`, time.Now().UTC().UnixNano()); err != nil {
		_ = tx.Rollback()
		return nil, fmt.Errorf("backfill player AP: %w", err)
	}
	if _, err := tx.Exec(`
INSERT OR IGNORE INTO player_locations (user_id, location_id)
SELECT id, 'camp' FROM identities`); err != nil {
		_ = tx.Rollback()
		return nil, fmt.Errorf("backfill player locations: %w", err)
	}
	if _, err := tx.Exec(`
INSERT OR IGNORE INTO player_resources (user_id, balance)
SELECT id, 0 FROM identities`); err != nil {
		_ = tx.Rollback()
		return nil, fmt.Errorf("backfill player resources: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit auth store initialization: %w", err)
	}
	return &Store{db: db, now: time.Now}, nil
}

func (s *Store) UpsertIdentity(issuer, subject, email, displayName string) (Identity, error) {
	if strings.TrimSpace(issuer) == "" || strings.TrimSpace(subject) == "" {
		return Identity{}, fmt.Errorf("%w: issuer and subject are required", ErrInvalidArgument)
	}
	now := s.now().UTC().UnixNano()
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
	if _, err := tx.Exec(`
INSERT INTO player_resources (user_id, balance) VALUES (?, 0)
ON CONFLICT (user_id) DO NOTHING`, identity.ID); err != nil {
		_ = tx.Rollback()
		return Identity{}, fmt.Errorf("initialize player resources: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return Identity{}, fmt.Errorf("commit upsert identity: %w", err)
	}
	identity.CreatedAt = unixNano(createdAt)
	identity.UpdatedAt = unixNano(updatedAt)
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
	return calculateAP(unixNano(fullTimestamp), s.now().UTC()), nil
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
	if calculateAP(unixNano(fullTimestamp), now) == 0 {
		_ = tx.Rollback()
		return 0, ErrInsufficientAP
	}
	fullAt := unixNano(fullTimestamp)
	if fullAt.Before(now) {
		fullAt = now
	}
	nextFullTimestamp := fullAt.Add(apRecoveryTime).UnixNano()
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
	return calculateAP(unixNano(nextFullTimestamp), now), nil
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
	state := PlayerState{Routes: make([]Route, 0), Inventory: make([]InventoryItem, 0)}
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
	state.AP = calculateAP(unixNano(fullTimestamp), now)
	if err := tx.QueryRow(`SELECT balance FROM player_resources WHERE user_id = ?`, userID).Scan(&state.Resource); errors.Is(err, sql.ErrNoRows) {
		return PlayerState{}, ErrIdentityNotFound
	} else if err != nil {
		return PlayerState{}, fmt.Errorf("get player resources: %w", err)
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

func conversionOptionForLocation(tx *sql.Tx, locationID string) (ConversionOption, error) {
	var conversion ConversionOption
	err := tx.QueryRow(`
SELECT i.id, i.display_name, cr.input_quantity, cr.resource_yield, cr.ap_cost
FROM conversion_rules cr
JOIN items i ON i.id = cr.input_item_id
WHERE cr.location_id = ?`, locationID).Scan(
		&conversion.Item.ID, &conversion.Item.DisplayName, &conversion.InputQuantity,
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
	if calculateAP(unixNano(fullTimestamp), now) < option.APCost {
		_ = tx.Rollback()
		return PlayerState{}, ErrInsufficientAP
	}
	fullAt := unixNano(fullTimestamp)
	if fullAt.Before(now) {
		fullAt = now
	}
	nextFullTimestamp := fullAt.Add(time.Duration(option.APCost) * apRecoveryTime).UnixNano()
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
	if calculateAP(unixNano(fullTimestamp), now) < route.APCost {
		_ = tx.Rollback()
		return PlayerState{}, ErrInsufficientAP
	}
	fullAt := unixNano(fullTimestamp)
	if fullAt.Before(now) {
		fullAt = now
	}
	nextFullTimestamp := fullAt.Add(time.Duration(route.APCost) * apRecoveryTime).UnixNano()
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
	if calculateAP(unixNano(fullTimestamp), now) < option.APCost {
		_ = tx.Rollback()
		return PlayerState{}, ErrInsufficientAP
	}
	fullAt := unixNano(fullTimestamp)
	if fullAt.Before(now) {
		fullAt = now
	}
	nextFullTimestamp := fullAt.Add(time.Duration(option.APCost) * apRecoveryTime).UnixNano()
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
	_, err = tx.Exec(`UPDATE player_resources SET balance = balance + ? WHERE user_id = ?`, option.ResourceYield, userID)
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
	identity.CreatedAt = unixNano(createdAt)
	identity.UpdatedAt = unixNano(updatedAt)
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
VALUES (?, ?, ?, ?, ?)`, hashSecret(state), browserHash, nonce, verifier, expiresAt.UTC().UnixNano())
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
	nowNanos := now.UnixNano()
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
		AND (? = 0 OR browser_token_hash = ?)`, hashSecret(state), nowNanos, boundFlag, browserHash).Scan(
		&attempt.Nonce, &attempt.Verifier, &expiresAt,
	)
	if err == nil {
		result, updateErr := tx.Exec(`
UPDATE oauth_attempts
SET nonce = '', verifier = '', consumed_at = ?
WHERE state_hash = ? AND consumed_at IS NULL AND expires_at > ?
			AND (? = 0 OR browser_token_hash = ?)`, nowNanos, hashSecret(state), nowNanos, boundFlag, browserHash)
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
		attempt.ExpiresAt = unixNano(expiresAt)
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
	if expiresAt <= nowNanos {
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
VALUES (?, ?, ?, ?)`, hashSecret(token), userID, expiresAt.UTC().UnixNano(), s.now().UTC().UnixNano())
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
	session.ExpiresAt = unixNano(expiresAt)
	if expiresAt <= s.now().UTC().UnixNano() {
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

func unixNano(value int64) time.Time {
	return time.Unix(0, value).UTC()
}
