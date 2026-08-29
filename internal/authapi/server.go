package authapi

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

const defaultSessionCookieName = "mud_session"
const oauthFlowCookieName = "mud_oauth_flow"

// IdentityProvider supplies the provider-specific OAuth authorization and
// token exchange operations. The server only accepts verified identity data.
type IdentityProvider interface {
	AuthorizationURL(state, nonce, codeChallenge string) (string, error)
	Exchange(ctx context.Context, code, codeVerifier string) (ProviderIdentity, error)
}

type ProviderIdentity struct {
	Issuer      string
	Subject     string
	Email       string
	DisplayName string
	Nonce       string
}

type currentUserResponse struct {
	ID          int64  `json:"id"`
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
	playerStateResponse
}

type restResponse struct {
	AP int `json:"ap"`
}

type restConflictResponse struct {
	Error string `json:"error"`
	AP    int    `json:"ap"`
}

type locationResponse struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
}

type routeResponse struct {
	OriginID      string `json:"origin_id"`
	DestinationID string `json:"destination_id"`
	APCost        int    `json:"ap_cost"`
}

type playerStateResponse struct {
	Location                     locationResponse                      `json:"location"`
	Routes                       []routeResponse                       `json:"routes"`
	AP                           int                                   `json:"ap"`
	CarriedWeight                int                                   `json:"carried_weight"`
	MovementWeightThreshold      int                                   `json:"movement_weight_threshold"`
	Inventory                    []inventoryItemResponse               `json:"inventory"`
	GroundItems                  []groundItemResponse                  `json:"ground_items"`
	GroundResources              []groundResourceResponse              `json:"ground_resources"`
	GatheringOption              *gatheringOptionResponse              `json:"gathering_option"`
	ConversionOption             *conversionOptionResponse             `json:"conversion_option"`
	ConversionMethods            []conversionMethodResponse            `json:"conversion_methods"`
	BuildingExtensionDefinitions []buildingExtensionDefinitionResponse `json:"building_extension_definitions"`
	Resources                    []resourceResponse                    `json:"resources"`
	CraftingRecipes              []craftingRecipeResponse              `json:"crafting_recipes"`
	BuildingRecipes              []buildingRecipeResponse              `json:"building_recipes"`
	Buildings                    []buildingResponse                    `json:"buildings"`
}

type moveResponse struct {
	Error string `json:"error,omitempty"`
	playerStateResponse
}

type moveRequest struct {
	Target string
}

type itemResponse struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
}

type inventoryItemResponse struct {
	Item                       itemResponse `json:"item"`
	Quantity                   int          `json:"quantity"`
	DurabilityStatus           string       `json:"durability_status"`
	DurabilityRemainingSeconds *int         `json:"durability_remaining_seconds"`
	RetentionRemainingSeconds  *int         `json:"retention_remaining_seconds"`
}

type groundItemResponse struct {
	Item                       itemResponse `json:"item"`
	Quantity                   int          `json:"quantity"`
	DurabilityStatus           string       `json:"durability_status"`
	DurabilityRemainingSeconds *int         `json:"durability_remaining_seconds"`
	RetentionRemainingSeconds  *int         `json:"retention_remaining_seconds"`
}

type groundResourceResponse struct {
	Resource itemResponse `json:"resource"`
	Quantity int          `json:"quantity"`
}

type gatheringOptionResponse struct {
	Item     itemResponse `json:"item"`
	Quantity int          `json:"quantity"`
	APCost   int          `json:"ap_cost"`
}

type gatherResponse struct {
	Error string `json:"error,omitempty"`
	playerStateResponse
}

type convertResponse struct {
	Error            string `json:"error,omitempty"`
	MethodID         string `json:"method_id,omitempty"`
	Quantity         int    `json:"quantity,omitempty"`
	ResourceQuantity int    `json:"resource_quantity,omitempty"`
	EssenceQuantity  int    `json:"essence_quantity"`
	playerStateResponse
}

type craftResponse struct {
	Error string `json:"error,omitempty"`
	playerStateResponse
}

type buildResponse struct {
	Error string `json:"error,omitempty"`
	playerStateResponse
}

type repairBuildingResponse struct {
	Error string `json:"error,omitempty"`
	playerStateResponse
}

type contributeConstructionResponse struct {
	Error string `json:"error,omitempty"`
	playerStateResponse
}

type extensionActionResponse struct {
	Error string `json:"error,omitempty"`
	playerStateResponse
}

type conversionMethodResponse struct {
	ID                       string        `json:"id"`
	DisplayName              string        `json:"display_name"`
	APCost                   int           `json:"ap_cost"`
	Input                    itemResponse  `json:"input"`
	MaxInputQuantity         int           `json:"max_input_quantity"`
	OutputResource           itemResponse  `json:"output_resource"`
	ResourceQuantityPerInput int           `json:"resource_quantity_per_input"`
	EssenceItem              *itemResponse `json:"essence_item"`
	EssenceChanceBPS         int           `json:"essence_chance_bps"`
	EssenceQuantity          int           `json:"essence_quantity"`
}

type buildingExtensionDefinitionResponse struct {
	ID          string       `json:"id"`
	DisplayName string       `json:"display_name"`
	Tier        int          `json:"tier"`
	PackageItem itemResponse `json:"package_item"`
	RequiredAP  int          `json:"required_ap"`
}

type transferResponse struct {
	Error string `json:"error,omitempty"`
	playerStateResponse
}

type craftingResourceInputResponse struct {
	Resource itemResponse `json:"resource"`
	Quantity int          `json:"quantity"`
}

type craftingItemInputResponse struct {
	Item     itemResponse `json:"item"`
	Quantity int          `json:"quantity"`
}

type craftingRecipeResponse struct {
	ID             string                          `json:"id"`
	DisplayName    string                          `json:"display_name"`
	BaseAPCost     int                             `json:"base_ap_cost"`
	ResourceInputs []craftingResourceInputResponse `json:"resource_inputs"`
	ItemInputs     []craftingItemInputResponse     `json:"item_inputs"`
	Output         itemResponse                    `json:"output"`
	OutputQuantity int                             `json:"output_quantity"`
}

const (
	transferDropOperation            = "drop"
	transferPickupOperation          = "pickup"
	transferReasonInvalidJSON        = "invalid_json"
	transferReasonUnknownField       = "unknown_field"
	transferReasonDuplicate          = "duplicate_field"
	transferReasonExtraValue         = "extra_json_value"
	transferReasonMissingAssetType   = "missing_asset_type"
	transferReasonInvalidAssetType   = "invalid_asset_type"
	transferReasonMissingAssetID     = "missing_asset_id"
	transferReasonInvalidAssetID     = "invalid_asset_id"
	transferReasonMissingQuantity    = "missing_quantity"
	transferReasonInvalidQuantity    = "invalid_quantity"
	transferReasonMissingItemStatus  = "missing_item_status"
	transferReasonInvalidItemStatus  = "invalid_item_status"
	transferReasonResourceItemStatus = "resource_item_status_not_allowed"
	transferReasonExpiredItem        = "expired_item"

	moveAction                       = "move"
	moveReasonInvalidJSON            = "invalid_json"
	moveReasonUnknownField           = "unknown_field"
	moveReasonDuplicate              = "duplicate_field"
	moveReasonExtraValue             = "extra_json_value"
	moveReasonMissingTarget          = "missing_target"
	moveReasonInvalidTarget          = "invalid_target"
	moveReasonOverweight             = "overweight"
	moveReasonUnsupported            = "unsupported_action"
	gatherAction                     = "gather"
	gatherReasonInvalidJSON          = "invalid_json"
	gatherReasonUnknownField         = "unknown_field"
	gatherReasonDuplicate            = "duplicate_field"
	gatherReasonExtraValue           = "extra_json_value"
	gatherReasonInvalidLocation      = "invalid_location"
	convertAction                    = "convert"
	convertReasonInvalidJSON         = "invalid_json"
	convertReasonUnknownField        = "unknown_field"
	convertReasonDuplicate           = "duplicate_field"
	convertReasonExtraValue          = "extra_json_value"
	convertReasonInvalidQuantity     = "invalid_quantity"
	convertReasonInsufficientAP      = "insufficient_ap"
	convertReasonInvalidLocation     = "invalid_location"
	convertReasonInsufficientItem    = "insufficient_item"
	craftAction                      = "craft"
	craftReasonInvalidJSON           = "invalid_json"
	craftReasonUnknownField          = "unknown_field"
	craftReasonDuplicate             = "duplicate_field"
	craftReasonExtraValue            = "extra_json_value"
	craftReasonMissingRecipe         = "missing_recipe_id"
	craftReasonInvalidRecipe         = "invalid_recipe_id"
	craftReasonInsufficientAP        = "insufficient_ap"
	craftReasonInsufficientResource  = "insufficient_resource"
	craftReasonInsufficientItem      = "insufficient_item"
	craftReasonUnknownRecipe         = "unknown_recipe"
	buildAction                      = "build"
	buildReasonInvalidJSON           = "invalid_json"
	buildReasonUnknownField          = "unknown_field"
	buildReasonDuplicate             = "duplicate_field"
	buildReasonExtraValue            = "extra_json_value"
	buildReasonMissingRecipe         = "missing_recipe_id"
	buildReasonInvalidRecipe         = "invalid_recipe_id"
	buildReasonUnknownRecipe         = "unknown_recipe"
	buildReasonInsufficientResource  = "insufficient_resource"
	buildReasonInsufficientItem      = "insufficient_item"
	buildReasonOccupied              = "building_occupied"
	contributeConstructionAction     = "contribute-construction"
	contributeReasonInvalidJSON      = "invalid_json"
	contributeReasonUnknownField     = "unknown_field"
	contributeReasonDuplicate        = "duplicate_field"
	contributeReasonExtraValue       = "extra_json_value"
	contributeReasonMissingBuilding  = "missing_building_id"
	contributeReasonMissingAP        = "missing_ap"
	contributeReasonInvalidBuilding  = "invalid_building_id"
	contributeReasonInvalidAP        = "invalid_ap"
	contributeReasonUnknownBuilding  = "unknown_building"
	contributeReasonRemote           = "remote_building"
	contributeReasonCompleted        = "completed_building"
	contributeReasonInsufficientAP   = "insufficient_ap"
	repairBuildingAction             = "repair-building"
	repairReasonInvalidJSON          = "invalid_json"
	repairReasonUnknownField         = "unknown_field"
	repairReasonDuplicate            = "duplicate_field"
	repairReasonExtraValue           = "extra_json_value"
	repairReasonMissingBuilding      = "missing_building_id"
	repairReasonInvalidBuilding      = "invalid_building_id"
	repairReasonUnknownBuilding      = "unknown_building"
	repairReasonRemote               = "remote_building"
	repairReasonUnderConstruction    = "under_construction"
	repairReasonInsufficientAP       = "insufficient_ap"
	repairReasonInsufficientResource = "insufficient_resource"
)

type Config struct {
	FrontendURL       string
	CookieSecure      bool
	SessionTTL        time.Duration
	OAuthAttemptTTL   time.Duration
	SessionCookieName string
	Now               func() time.Time
}

type Server struct {
	store          *Store
	provider       IdentityProvider
	cfg            Config
	frontendOrigin string
}

func NewServer(store *Store, provider IdentityProvider, cfg Config) (*Server, error) {
	if store == nil || provider == nil {
		return nil, errors.New("auth server requires store and identity provider")
	}
	frontend, err := url.Parse(cfg.FrontendURL)
	if err != nil || frontend.Scheme == "" || frontend.Host == "" || frontend.User != nil || frontend.RawQuery != "" || frontend.Fragment != "" {
		return nil, errors.New("auth server requires a valid frontend URL")
	}
	frontendOrigin, err := canonicalOrigin(frontend)
	if err != nil {
		return nil, err
	}
	if cfg.SessionTTL <= 0 {
		cfg.SessionTTL = 30 * 24 * time.Hour
	}
	if cfg.OAuthAttemptTTL <= 0 {
		cfg.OAuthAttemptTTL = 10 * time.Minute
	}
	if cfg.SessionCookieName == "" {
		cfg.SessionCookieName = defaultSessionCookieName
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	store.now = cfg.Now
	return &Server{store: store, provider: provider, cfg: cfg, frontendOrigin: frontendOrigin}, nil
}

func (s *Server) Routes(frontendFallback ...http.Handler) http.Handler {
	r := chi.NewRouter()
	r.Use(s.requestID)
	r.Use(s.accessLog)
	r.Use(s.cors)
	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		if len(frontendFallback) > 0 && frontendFallback[0] != nil && !isReservedPath(r.URL.Path) {
			frontendFallback[0].ServeHTTP(w, r)
			return
		}
		s.writeNotFound(w, r)
	})
	r.MethodNotAllowed(s.writeMethodNotAllowed)
	r.Get("/auth/google/login", s.login)
	r.Get("/auth/google/callback", s.callback)
	r.Get("/api/me", s.me)
	r.Post("/api/actions/rest", s.rest)
	r.Post("/api/actions/move", s.move)
	r.Post("/api/actions/gather", s.gather)
	r.Post("/api/actions/convert", s.convert)
	r.Post("/api/actions/craft", s.craft)
	r.Post("/api/actions/build", s.build)
	r.Post("/api/actions/contribute-construction", s.contributeConstruction)
	r.Post("/api/actions/install-extension", s.installExtension)
	r.Post("/api/actions/contribute-extension-construction", s.contributeExtensionConstruction)
	r.Post("/api/actions/remove-extension", s.removeExtension)
	r.Post("/api/actions/repair-building", s.repairBuilding)
	r.Post("/api/transfers/drop", s.drop)
	r.Post("/api/transfers/pickup", s.pickup)
	return r
}

func (s *Server) Handler(frontendFallback ...http.Handler) http.Handler {
	return s.Routes(frontendFallback...)
}

func isReservedPath(path string) bool {
	return path == "/api" || strings.HasPrefix(path, "/api/") || path == "/auth" || strings.HasPrefix(path, "/auth/")
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	state, err := randomString(32)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "login unavailable")
		return
	}
	nonce, err := randomString(32)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "login unavailable")
		return
	}
	verifier, err := randomString(32)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "login unavailable")
		return
	}
	challenge := pkceChallenge(verifier)
	authorizationURL, err := s.provider.AuthorizationURL(state, nonce, challenge)
	if err != nil || authorizationURL == "" {
		s.writeError(w, http.StatusBadGateway, "login unavailable")
		return
	}
	flowToken, err := randomString(32)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "login unavailable")
		return
	}
	if err := s.store.CreateOAuthAttempt(state, nonce, verifier, s.cfg.Now().UTC().Add(s.cfg.OAuthAttemptTTL), flowToken); err != nil {
		s.writeError(w, http.StatusInternalServerError, "login unavailable")
		return
	}
	http.SetCookie(w, &http.Cookie{Name: oauthFlowCookieName, Value: flowToken, Path: "/", MaxAge: int(s.cfg.OAuthAttemptTTL / time.Second), HttpOnly: true, Secure: s.cfg.CookieSecure, SameSite: http.SameSiteLaxMode})
	http.Redirect(w, r, authorizationURL, http.StatusFound)
}

func (s *Server) callback(w http.ResponseWriter, r *http.Request) {
	s.clearOAuthFlowCookie(w)
	state := r.URL.Query().Get("state")
	code := r.URL.Query().Get("code")
	if state == "" || code == "" {
		s.writeError(w, http.StatusBadRequest, "invalid login callback")
		return
	}
	flowCookie, err := r.Cookie(oauthFlowCookieName)
	if err != nil || flowCookie.Value == "" {
		s.writeError(w, http.StatusBadRequest, "invalid login state")
		return
	}
	attempt, err := s.store.ConsumeOAuthAttempt(state, flowCookie.Value)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid login state")
		return
	}
	identity, err := s.provider.Exchange(r.Context(), code, attempt.Verifier)
	if err != nil || !sameSecret(identity.Nonce, attempt.Nonce) {
		s.writeError(w, http.StatusBadRequest, "login verification failed")
		return
	}
	if strings.TrimSpace(identity.Issuer) == "" || strings.TrimSpace(identity.Subject) == "" {
		s.writeError(w, http.StatusBadRequest, "login verification failed")
		return
	}
	user, err := s.store.UpsertIdentity(identity.Issuer, identity.Subject, identity.Email, identity.DisplayName)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "login unavailable")
		return
	}
	sessionToken, err := randomString(32)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "login unavailable")
		return
	}
	expiresAt := s.cfg.Now().UTC().Add(s.cfg.SessionTTL)
	if err := s.store.CreateSession(user.ID, sessionToken, expiresAt); err != nil {
		s.writeError(w, http.StatusInternalServerError, "login unavailable")
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     s.cfg.SessionCookieName,
		Value:    sessionToken,
		Path:     "/",
		Expires:  expiresAt,
		MaxAge:   int(s.cfg.SessionTTL / time.Second),
		HttpOnly: true,
		Secure:   s.cfg.CookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, s.cfg.FrontendURL, http.StatusFound)
}

func (s *Server) clearOAuthFlowCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: oauthFlowCookieName, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: s.cfg.CookieSecure, SameSite: http.SameSiteLaxMode})
}

func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(s.cfg.SessionCookieName)
	if err != nil || cookie.Value == "" {
		s.writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	identity, err := s.store.GetIdentityForSession(cookie.Value)
	if err != nil {
		s.writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	state, err := s.store.GetPlayerState(identity.ID)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "current user unavailable")
		return
	}
	s.logComputation(r, identity.ID, "ap_calculation", "success", state.AP)
	s.logCarryingWeightComputation(r, identity.ID, state)
	s.logBuildingDurabilityComputation(r, identity.ID, state.Buildings)
	s.logItemDurability(r, fmt.Sprintf("%d", identity.ID), state)
	response := currentUserResponse{
		ID:                  identity.ID,
		DisplayName:         identity.DisplayName,
		Email:               identity.Email,
		playerStateResponse: playerStateResponseFromStore(state),
	}
	s.writeJSON(w, http.StatusOK, response)
}

func (s *Server) convert(w http.ResponseWriter, r *http.Request) {
	session, err := s.authenticatedSession(r)
	if err != nil {
		s.writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	request, reason := decodeConvertRequest(r.Body)
	if reason != "" {
		s.logRejection(r, session.UserID, convertAction, reason)
		state, stateErr := s.store.GetPlayerState(session.UserID)
		if stateErr != nil {
			s.writeError(w, http.StatusInternalServerError, "action unavailable")
			return
		}
		s.writeJSON(w, http.StatusBadRequest, convertResponse{Error: "invalid action input", playerStateResponse: s.playerStateResponse(r, session.UserID, state)})
		return
	}
	before, err := s.store.GetPlayerState(session.UserID)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "action unavailable")
		return
	}
	state, err := s.store.Convert(session.UserID, request.MethodID, request.Quantity, request.ProviderExtensionID)
	if errors.Is(err, ErrInsufficientAP) {
		state, stateErr := s.store.GetPlayerState(session.UserID)
		if stateErr != nil {
			s.writeError(w, http.StatusInternalServerError, "action unavailable")
			return
		}
		s.logComputation(r, session.UserID, "ap_calculation", "insufficient_ap", state.AP)
		s.logRejection(r, session.UserID, convertAction, convertReasonInsufficientAP)
		s.writeJSON(w, http.StatusConflict, convertResponse{Error: ErrInsufficientAP.Error(), playerStateResponse: s.playerStateResponse(r, session.UserID, state)})
		return
	}
	if errors.Is(err, ErrInsufficientItem) {
		state, stateErr := s.store.GetPlayerState(session.UserID)
		if stateErr != nil {
			s.writeError(w, http.StatusInternalServerError, "action unavailable")
			return
		}
		s.logRejection(r, session.UserID, convertAction, convertReasonInsufficientItem)
		s.writeJSON(w, http.StatusConflict, convertResponse{Error: ErrInsufficientItem.Error(), playerStateResponse: s.playerStateResponse(r, session.UserID, state)})
		return
	}
	if errors.Is(err, ErrConversionNotFound) {
		state, stateErr := s.store.GetPlayerState(session.UserID)
		if stateErr != nil {
			s.writeError(w, http.StatusInternalServerError, "action unavailable")
			return
		}
		s.logRejection(r, session.UserID, convertAction, convertReasonInvalidLocation)
		s.writeJSON(w, http.StatusBadRequest, convertResponse{Error: ErrConversionNotFound.Error(), playerStateResponse: s.playerStateResponse(r, session.UserID, state)})
		return
	}
	if errors.Is(err, ErrInvalidArgument) && strings.Contains(err.Error(), "capacity") {
		state, stateErr := s.store.GetPlayerState(session.UserID)
		if stateErr != nil {
			s.writeError(w, http.StatusInternalServerError, "action unavailable")
			return
		}
		s.logRejection(r, session.UserID, convertAction, convertReasonInvalidQuantity)
		s.writeJSON(w, http.StatusBadRequest, convertResponse{Error: "invalid action input", MethodID: request.MethodID, Quantity: request.Quantity, playerStateResponse: s.playerStateResponse(r, session.UserID, state)})
		return
	}
	if errors.Is(err, ErrInvalidArgument) || errors.Is(err, ErrExtensionNotFound) || errors.Is(err, ErrBuildingRemote) || errors.Is(err, ErrBuildingDisabled) {
		state, stateErr := s.store.GetPlayerState(session.UserID)
		if stateErr != nil {
			s.writeError(w, http.StatusInternalServerError, "action unavailable")
			return
		}
		s.logRejection(r, session.UserID, convertAction, "invalid_provider")
		s.writeJSON(w, http.StatusBadRequest, convertResponse{Error: "invalid provider extension", playerStateResponse: s.playerStateResponse(r, session.UserID, state)})
		return
	}
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "action unavailable")
		return
	}
	resourceQuantity := resourceDelta(before, state, request.MethodID, request.Quantity)
	essenceQuantity := itemDelta(before, state, request.MethodID)
	s.logConvertComputation(r, session.UserID, request, resourceQuantity, essenceQuantity, "success")
	s.logComputation(r, session.UserID, "ap_calculation", "success", state.AP)
	s.logAction(r, session.UserID, convertAction, "success")
	s.writeJSON(w, http.StatusOK, convertResponse{MethodID: request.MethodID, Quantity: request.Quantity, ResourceQuantity: resourceQuantity, EssenceQuantity: essenceQuantity, playerStateResponse: s.playerStateResponse(r, session.UserID, state)})
}

func (s *Server) craft(w http.ResponseWriter, r *http.Request) {
	session, err := s.authenticatedSession(r)
	if err != nil {
		s.writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	request, reason := decodeCraftRequest(r.Body)
	if reason != "" {
		s.logRejection(r, session.UserID, craftAction, reason)
		s.writeCraftState(w, r, session.UserID, http.StatusBadRequest, "invalid action input")
		return
	}
	state, err := s.store.Craft(session.UserID, request.RecipeID)
	if errors.Is(err, ErrInsufficientAP) {
		s.logComputation(r, session.UserID, "ap_calculation", "insufficient_ap", currentAP(state, s.store, session.UserID))
		s.logRejection(r, session.UserID, craftAction, craftReasonInsufficientAP)
		s.writeCraftState(w, r, session.UserID, http.StatusConflict, ErrInsufficientAP.Error())
		return
	}
	if errors.Is(err, ErrInsufficientResource) {
		s.logRejection(r, session.UserID, craftAction, craftReasonInsufficientResource)
		s.writeCraftState(w, r, session.UserID, http.StatusConflict, ErrInsufficientResource.Error())
		return
	}
	if errors.Is(err, ErrInsufficientItem) {
		s.logRejection(r, session.UserID, craftAction, craftReasonInsufficientItem)
		s.writeCraftState(w, r, session.UserID, http.StatusConflict, ErrInsufficientItem.Error())
		return
	}
	if errors.Is(err, ErrCraftingNotFound) {
		s.logRejection(r, session.UserID, craftAction, craftReasonUnknownRecipe)
		s.writeCraftState(w, r, session.UserID, http.StatusBadRequest, ErrCraftingNotFound.Error())
		return
	}
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "action unavailable")
		return
	}
	s.logComputation(r, session.UserID, "ap_calculation", "success", state.AP)
	s.logAction(r, session.UserID, craftAction, "success")
	s.writeJSON(w, http.StatusOK, s.playerStateResponse(r, session.UserID, state))
}

func (s *Server) writeCraftState(w http.ResponseWriter, r *http.Request, userID int64, status int, message string) {
	state, err := s.store.GetPlayerState(userID)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "action unavailable")
		return
	}
	s.writeJSON(w, status, craftResponse{Error: message, playerStateResponse: s.playerStateResponse(r, userID, state)})
}

type buildRequest struct {
	RecipeID string
}

func (s *Server) build(w http.ResponseWriter, r *http.Request) {
	session, err := s.authenticatedSession(r)
	if err != nil {
		s.writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	request, reason := decodeBuildRequest(r.Body)
	if reason != "" {
		s.logRejection(r, session.UserID, buildAction, reason)
		s.writeBuildingState(w, r, session.UserID, http.StatusBadRequest, "invalid action input")
		return
	}
	state, err := s.store.Build(session.UserID, request.RecipeID)
	if errors.Is(err, ErrBuildingNotFound) {
		s.logRejection(r, session.UserID, buildAction, buildReasonUnknownRecipe)
		s.writeBuildingState(w, r, session.UserID, http.StatusBadRequest, err.Error())
		return
	}
	if errors.Is(err, ErrBuildingOccupied) {
		s.logRejection(r, session.UserID, buildAction, buildReasonOccupied)
		s.writeBuildingState(w, r, session.UserID, http.StatusConflict, err.Error())
		return
	}
	if errors.Is(err, ErrInsufficientResource) {
		s.logRejection(r, session.UserID, buildAction, buildReasonInsufficientResource)
		s.writeBuildingState(w, r, session.UserID, http.StatusConflict, err.Error())
		return
	}
	if errors.Is(err, ErrInsufficientItem) {
		s.logRejection(r, session.UserID, buildAction, buildReasonInsufficientItem)
		s.writeBuildingState(w, r, session.UserID, http.StatusConflict, err.Error())
		return
	}
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "action unavailable")
		return
	}
	s.logAction(r, session.UserID, buildAction, "success")
	s.writeJSON(w, http.StatusOK, s.playerStateResponse(r, session.UserID, state))
}

func (s *Server) writeBuildingState(w http.ResponseWriter, r *http.Request, userID int64, status int, message string) {
	state, err := s.store.GetPlayerState(userID)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "action unavailable")
		return
	}
	s.writeJSON(w, status, buildResponse{Error: message, playerStateResponse: s.playerStateResponse(r, userID, state)})
}

type installExtensionRequest struct {
	BuildingID   int64
	SlotIndex    int
	DefinitionID string
}
type extensionIDRequest struct {
	ExtensionID int64
	AP          int
}

type extensionActionMetadata struct {
	BuildingID  int64
	ExtensionID int64
	AP          int
	Computation *ExtensionConstructionComputation
}

func (s *Server) installExtension(w http.ResponseWriter, r *http.Request) {
	request, reason := decodeInstallExtensionRequest(r.Body)
	s.extensionActionWithMeta(w, r, "install-extension", func(userID int64) (PlayerState, extensionActionMetadata, error) {
		if reason != "" {
			return PlayerState{}, extensionActionMetadata{}, fmt.Errorf("%s", reason)
		}
		metadata := extensionActionMetadata{BuildingID: request.BuildingID}
		state, err := s.store.InstallExtension(userID, request.BuildingID, request.SlotIndex, request.DefinitionID)
		if err != nil {
			return PlayerState{}, metadata, err
		}
		metadata.ExtensionID = extensionIDAtSlot(state, request.BuildingID, request.SlotIndex)
		return state, metadata, nil
	})
}

func (s *Server) contributeExtensionConstruction(w http.ResponseWriter, r *http.Request) {
	request, reason := decodeExtensionIDRequest(r.Body, true)
	s.extensionActionWithMeta(w, r, "contribute-extension-construction", func(userID int64) (PlayerState, extensionActionMetadata, error) {
		if reason != "" {
			return PlayerState{}, extensionActionMetadata{}, fmt.Errorf("%s", reason)
		}
		metadata := extensionActionMetadata{ExtensionID: request.ExtensionID}
		state, err := s.store.ContributeExtensionConstruction(userID, request.ExtensionID, request.AP)
		if computation := state.ExtensionConstructionComputation; computation != nil {
			metadata.BuildingID = computation.BuildingID
			metadata.ExtensionID = computation.ExtensionID
			metadata.AP = computation.EffectiveAP
			metadata.Computation = computation
		}
		return state, metadata, err
	})
}

func (s *Server) removeExtension(w http.ResponseWriter, r *http.Request) {
	request, reason := decodeExtensionIDRequest(r.Body, false)
	s.extensionActionWithMeta(w, r, "remove-extension", func(userID int64) (PlayerState, extensionActionMetadata, error) {
		if reason != "" {
			return PlayerState{}, extensionActionMetadata{}, fmt.Errorf("%s", reason)
		}
		metadata := extensionActionMetadata{ExtensionID: request.ExtensionID}
		var err error
		metadata.BuildingID, err = s.extensionBuildingID(request.ExtensionID)
		if err != nil {
			return PlayerState{}, metadata, err
		}
		state, err := s.store.RemoveExtension(userID, request.ExtensionID)
		return state, metadata, err
	})
}

func (s *Server) extensionActionWithMeta(w http.ResponseWriter, r *http.Request, action string, execute func(int64) (PlayerState, extensionActionMetadata, error)) {
	session, err := s.authenticatedSession(r)
	if err != nil {
		s.writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	state, metadata, err := execute(session.UserID)
	if err != nil {
		s.logExtensionAction(r, session.UserID, action, metadata.BuildingID, metadata.ExtensionID, metadata.AP, "error", metadata.Computation)
		current, stateErr := s.store.GetPlayerState(session.UserID)
		if stateErr != nil {
			s.writeError(w, http.StatusInternalServerError, "action unavailable")
			return
		}
		s.writeJSON(w, http.StatusConflict, extensionActionResponse{Error: "invalid action", playerStateResponse: s.playerStateResponse(r, session.UserID, current)})
		return
	}
	s.logExtensionAction(r, session.UserID, action, metadata.BuildingID, metadata.ExtensionID, metadata.AP, "success", metadata.Computation)
	s.writeJSON(w, http.StatusOK, extensionActionResponse{playerStateResponse: s.playerStateResponse(r, session.UserID, state)})
}

func extensionIDAtSlot(state PlayerState, buildingID int64, slotIndex int) int64 {
	for _, building := range state.Buildings {
		if building.ID != buildingID {
			continue
		}
		for _, extension := range building.Extensions {
			if extension.SlotIndex == slotIndex {
				return extension.ID
			}
		}
	}
	return 0
}

func (s *Server) extensionBuildingID(extensionID int64) (int64, error) {
	var buildingID int64
	err := s.store.db.QueryRow(`SELECT building_id FROM building_extensions WHERE id = ?`, extensionID).Scan(&buildingID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("get extension building: %w", err)
	}
	return buildingID, nil
}

func decodeInstallExtensionRequest(body io.Reader) (installExtensionRequest, string) {
	var value installExtensionRequest
	seenSlot := false
	if err := decodeStrictFields(body, map[string]bool{"building_id": true, "slot_index": true, "definition_id": true}, func(field string, decoder *json.Decoder) error {
		switch field {
		case "building_id":
			return decoder.Decode(&value.BuildingID)
		case "slot_index":
			seenSlot = true
			return decoder.Decode(&value.SlotIndex)
		case "definition_id":
			return decoder.Decode(&value.DefinitionID)
		}
		return nil
	}); err != nil {
		return value, convertReasonInvalidJSON
	}
	if !seenSlot || value.BuildingID <= 0 || value.SlotIndex < 0 || strings.TrimSpace(value.DefinitionID) == "" {
		return value, convertReasonInvalidQuantity
	}
	return value, ""
}

func decodeExtensionIDRequest(body io.Reader, withAP bool) (extensionIDRequest, string) {
	var value extensionIDRequest
	allowed := map[string]bool{"extension_id": true}
	if withAP {
		allowed["ap"] = true
	}
	if err := decodeStrictFields(body, allowed, func(field string, decoder *json.Decoder) error {
		switch field {
		case "extension_id":
			return decoder.Decode(&value.ExtensionID)
		case "ap":
			return decoder.Decode(&value.AP)
		}
		return nil
	}); err != nil {
		return value, convertReasonInvalidJSON
	}
	if value.ExtensionID <= 0 || (withAP && value.AP <= 0) {
		return value, convertReasonInvalidQuantity
	}
	return value, ""
}

func decodeStrictFields(body io.Reader, allowed map[string]bool, decode func(string, *json.Decoder) error) error {
	decoder := json.NewDecoder(body)
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	if delim, ok := token.(json.Delim); !ok || delim != '{' {
		return errors.New("object required")
	}
	seen := map[string]bool{}
	for decoder.More() {
		key, err := decoder.Token()
		if err != nil {
			return err
		}
		field, ok := key.(string)
		if !ok || !allowed[field] || seen[field] {
			return errors.New("invalid field")
		}
		seen[field] = true
		if err := decode(field, decoder); err != nil {
			return err
		}
	}
	token, err = decoder.Token()
	if err != nil {
		return err
	}
	if delim, ok := token.(json.Delim); !ok || delim != '}' {
		return errors.New("object required")
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return errors.New("extra value")
	}
	return nil
}

type repairBuildingRequest struct {
	BuildingID int64
}

func (s *Server) repairBuilding(w http.ResponseWriter, r *http.Request) {
	session, err := s.authenticatedSession(r)
	if err != nil {
		s.writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	request, reason := decodeRepairBuildingRequest(r.Body)
	if reason != "" {
		s.logRejection(r, session.UserID, repairBuildingAction, reason)
		s.writeRepairBuildingState(w, r, session.UserID, http.StatusBadRequest, "invalid action input")
		return
	}
	state, err := s.store.RepairBuilding(session.UserID, request.BuildingID)
	if errors.Is(err, ErrBuildingNotFound) {
		s.logRejection(r, session.UserID, repairBuildingAction, repairReasonUnknownBuilding)
		s.writeRepairBuildingState(w, r, session.UserID, http.StatusBadRequest, err.Error())
		return
	}
	if errors.Is(err, ErrBuildingRemote) {
		s.logRejection(r, session.UserID, repairBuildingAction, repairReasonRemote)
		s.writeRepairBuildingState(w, r, session.UserID, http.StatusConflict, err.Error())
		return
	}
	if errors.Is(err, ErrBuildingUnderConstruction) {
		s.logRejection(r, session.UserID, repairBuildingAction, repairReasonUnderConstruction)
		s.writeRepairBuildingState(w, r, session.UserID, http.StatusConflict, err.Error())
		return
	}
	if errors.Is(err, ErrInsufficientAP) {
		s.logRejection(r, session.UserID, repairBuildingAction, repairReasonInsufficientAP)
		s.writeRepairBuildingState(w, r, session.UserID, http.StatusConflict, err.Error())
		return
	}
	if errors.Is(err, ErrInsufficientResource) {
		s.logRejection(r, session.UserID, repairBuildingAction, repairReasonInsufficientResource)
		s.writeRepairBuildingState(w, r, session.UserID, http.StatusConflict, err.Error())
		return
	}
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "action unavailable")
		return
	}
	if computation := state.RepairComputation; computation != nil {
		s.logRepairComputation(r, session.UserID, computation)
	}
	s.logAction(r, session.UserID, repairBuildingAction, "success")
	s.writeJSON(w, http.StatusOK, s.playerStateResponse(r, session.UserID, state))
}

type transferRequest struct {
	AssetType  string
	AssetID    string
	Quantity   int
	ItemStatus string
}

func (s *Server) drop(w http.ResponseWriter, r *http.Request) {
	s.transfer(w, r, transferDropOperation, false)
}

func (s *Server) pickup(w http.ResponseWriter, r *http.Request) {
	s.transfer(w, r, transferPickupOperation, true)
}

func (s *Server) transfer(w http.ResponseWriter, r *http.Request, operation string, pickup bool) {
	session, err := s.authenticatedSession(r)
	if err != nil {
		s.writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	request, reason := decodeTransferRequest(r.Body)
	if reason != "" {
		s.writeTransferState(w, r, session.UserID, operation, request, http.StatusBadRequest, "invalid transfer input", reason)
		return
	}

	var state PlayerState
	if pickup {
		if request.AssetType == "item" {
			state, err = s.store.Pickup(session.UserID, request.AssetType, request.AssetID, request.Quantity, request.ItemStatus)
		} else {
			state, err = s.store.Pickup(session.UserID, request.AssetType, request.AssetID, request.Quantity, "")
		}
	} else {
		if request.AssetType == "item" {
			state, err = s.store.Drop(session.UserID, request.AssetType, request.AssetID, request.Quantity, request.ItemStatus)
		} else {
			state, err = s.store.Drop(session.UserID, request.AssetType, request.AssetID, request.Quantity, "")
		}
	}
	if err == nil {
		s.logTransfer(r, session.UserID, operation, state.Location.ID, request.AssetType, request.AssetID, request.Quantity, "success", "", request.ItemStatus)
		s.writeJSON(w, http.StatusOK, transferResponse{playerStateResponse: s.playerStateResponse(r, session.UserID, state)})
		return
	}
	if errors.Is(err, ErrTransferAssetNotFound) {
		s.writeTransferState(w, r, session.UserID, operation, request, http.StatusBadRequest, err.Error(), "unknown_asset")
		return
	}
	if errors.Is(err, ErrInvalidArgument) {
		s.writeTransferState(w, r, session.UserID, operation, request, http.StatusBadRequest, err.Error(), "invalid_argument")
		return
	}
	if errors.Is(err, ErrInsufficientTransferAsset) {
		reason := "insufficient_source"
		if pickup && request.AssetType == "item" && request.ItemStatus == "expired" {
			reason = transferReasonExpiredItem
		}
		s.writeTransferState(w, r, session.UserID, operation, request, http.StatusConflict, err.Error(), reason)
		return
	}
	s.logTransfer(r, session.UserID, operation, "unknown", request.AssetType, "unknown", request.Quantity, "error", "internal_error", request.ItemStatus)
	s.writeError(w, http.StatusInternalServerError, "transfer unavailable")
}

func (s *Server) writeTransferState(w http.ResponseWriter, r *http.Request, userID int64, operation string, request transferRequest, status int, message, reason string) {
	state, err := s.store.GetPlayerState(userID)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "transfer unavailable")
		return
	}
	assetID := request.AssetID
	if reason == "unknown_asset" || reason == "invalid_argument" || strings.HasPrefix(reason, "invalid_") || strings.HasSuffix(reason, "_field") || reason == "extra_json_value" || reason == transferReasonMissingItemStatus || reason == transferReasonResourceItemStatus {
		assetID = "unknown"
	}
	s.logTransfer(r, userID, operation, state.Location.ID, request.AssetType, assetID, request.Quantity, "error", reason, request.ItemStatus)
	s.writeJSON(w, status, transferResponse{Error: message, playerStateResponse: s.playerStateResponse(r, userID, state)})
}

func (s *Server) writeRepairBuildingState(w http.ResponseWriter, r *http.Request, userID int64, status int, message string) {
	state, err := s.store.GetPlayerState(userID)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "action unavailable")
		return
	}
	s.writeJSON(w, status, repairBuildingResponse{Error: message, playerStateResponse: s.playerStateResponse(r, userID, state)})
}

type contributeConstructionRequest struct {
	BuildingID int64
	AP         int
}

func (s *Server) contributeConstruction(w http.ResponseWriter, r *http.Request) {
	session, err := s.authenticatedSession(r)
	if err != nil {
		s.writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	request, reason := decodeContributeConstructionRequest(r.Body)
	if reason != "" {
		s.logRejection(r, session.UserID, contributeConstructionAction, reason)
		s.writeBuildingState(w, r, session.UserID, http.StatusBadRequest, "invalid action input")
		return
	}
	state, err := s.store.ContributeConstruction(session.UserID, request.BuildingID, request.AP)
	if errors.Is(err, ErrBuildingNotFound) {
		s.logRejection(r, session.UserID, contributeConstructionAction, contributeReasonUnknownBuilding)
		s.writeBuildingState(w, r, session.UserID, http.StatusBadRequest, err.Error())
		return
	}
	if errors.Is(err, ErrBuildingRemote) {
		s.logRejection(r, session.UserID, contributeConstructionAction, contributeReasonRemote)
		s.writeBuildingState(w, r, session.UserID, http.StatusConflict, err.Error())
		return
	}
	if errors.Is(err, ErrBuildingCompleted) {
		s.logRejection(r, session.UserID, contributeConstructionAction, contributeReasonCompleted)
		s.writeBuildingState(w, r, session.UserID, http.StatusConflict, err.Error())
		return
	}
	if errors.Is(err, ErrInsufficientAP) {
		s.logRejection(r, session.UserID, contributeConstructionAction, contributeReasonInsufficientAP)
		s.writeBuildingState(w, r, session.UserID, http.StatusConflict, err.Error())
		return
	}
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "action unavailable")
		return
	}
	if computation := state.ConstructionComputation; computation != nil {
		s.logConstructionComputation(r, session.UserID, computation)
	}
	s.logAction(r, session.UserID, contributeConstructionAction, "success")
	s.writeJSON(w, http.StatusOK, s.playerStateResponse(r, session.UserID, state))
}

func currentAP(state PlayerState, store *Store, userID int64) int {
	if state.AP != 0 || store == nil {
		return state.AP
	}
	ap, err := store.GetAP(userID)
	if err != nil {
		return state.AP
	}
	return ap
}

func (s *Server) gather(w http.ResponseWriter, r *http.Request) {
	session, err := s.authenticatedSession(r)
	if err != nil {
		s.writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	if reason := decodeGatherRequest(r.Body); reason != "" {
		s.logRejection(r, session.UserID, gatherAction, reason)
		state, stateErr := s.store.GetPlayerState(session.UserID)
		if stateErr != nil {
			s.writeError(w, http.StatusInternalServerError, "action unavailable")
			return
		}
		s.writeJSON(w, http.StatusBadRequest, gatherResponse{Error: "invalid action input", playerStateResponse: s.playerStateResponse(r, session.UserID, state)})
		return
	}
	state, err := s.store.Gather(session.UserID)
	if errors.Is(err, ErrInsufficientAP) {
		state, stateErr := s.store.GetPlayerState(session.UserID)
		if stateErr != nil {
			s.writeError(w, http.StatusInternalServerError, "action unavailable")
			return
		}
		s.logComputation(r, session.UserID, "ap_calculation", "insufficient_ap", state.AP)
		s.logAction(r, session.UserID, gatherAction, "insufficient_ap")
		s.writeJSON(w, http.StatusConflict, gatherResponse{Error: ErrInsufficientAP.Error(), playerStateResponse: s.playerStateResponse(r, session.UserID, state)})
		return
	}
	if errors.Is(err, ErrGatheringNotFound) {
		state, stateErr := s.store.GetPlayerState(session.UserID)
		if stateErr != nil {
			s.writeError(w, http.StatusInternalServerError, "action unavailable")
			return
		}
		s.logComputation(r, session.UserID, "ap_calculation", "invalid_location", state.AP)
		s.logRejection(r, session.UserID, gatherAction, gatherReasonInvalidLocation)
		s.writeJSON(w, http.StatusBadRequest, gatherResponse{Error: ErrGatheringNotFound.Error(), playerStateResponse: s.playerStateResponse(r, session.UserID, state)})
		return
	}
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "action unavailable")
		return
	}
	s.logComputation(r, session.UserID, "ap_calculation", "success", state.AP)
	s.logAction(r, session.UserID, gatherAction, "success")
	s.writeJSON(w, http.StatusOK, s.playerStateResponse(r, session.UserID, state))
}

func (s *Server) rest(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(s.cfg.SessionCookieName)
	if err != nil || cookie.Value == "" {
		s.writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	session, err := s.store.GetSession(cookie.Value)
	if err != nil {
		s.writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	ap, err := s.store.Rest(session.UserID)
	if errors.Is(err, ErrInsufficientAP) {
		ap, apErr := s.store.GetAP(session.UserID)
		if apErr != nil {
			s.writeError(w, http.StatusInternalServerError, "action unavailable")
			return
		}
		s.logComputation(r, session.UserID, "ap_calculation", "insufficient_ap", ap)
		s.logAction(r, session.UserID, "rest", "insufficient_ap")
		s.writeJSON(w, http.StatusConflict, restConflictResponse{Error: err.Error(), AP: ap})
		return
	}
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "action unavailable")
		return
	}
	s.logComputation(r, session.UserID, "ap_calculation", "success", ap)
	s.logAction(r, session.UserID, "rest", "success")
	s.writeJSON(w, http.StatusOK, restResponse{AP: ap})
}

func (s *Server) move(w http.ResponseWriter, r *http.Request) {
	session, err := s.authenticatedSession(r)
	if err != nil {
		s.writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	request, reason := decodeMoveRequest(r.Body)
	if reason != "" {
		s.logRejection(r, session.UserID, moveAction, reason)
		s.writeError(w, http.StatusBadRequest, "invalid action input")
		return
	}
	state, err := s.store.Move(session.UserID, request.Target)
	if errors.Is(err, ErrInsufficientAP) {
		state, stateErr := s.store.GetPlayerState(session.UserID)
		if stateErr != nil {
			s.writeError(w, http.StatusInternalServerError, "action unavailable")
			return
		}
		s.logComputation(r, session.UserID, "ap_calculation", "insufficient_ap", state.AP)
		s.logAction(r, session.UserID, moveAction, "insufficient_ap")
		s.writeJSON(w, http.StatusConflict, moveResponse{Error: ErrInsufficientAP.Error(), playerStateResponse: s.playerStateResponse(r, session.UserID, state)})
		return
	}
	if errors.Is(err, ErrOverweight) {
		state, stateErr := s.store.GetPlayerState(session.UserID)
		if stateErr != nil {
			s.writeError(w, http.StatusInternalServerError, "action unavailable")
			return
		}
		s.logWeightRejection(r, session.UserID, moveAction, moveReasonOverweight, state)
		s.writeJSON(w, http.StatusConflict, moveResponse{Error: ErrOverweight.Error(), playerStateResponse: s.playerStateResponse(r, session.UserID, state)})
		return
	}
	if errors.Is(err, ErrRouteNotFound) {
		state, stateErr := s.store.GetPlayerState(session.UserID)
		if stateErr != nil {
			s.writeError(w, http.StatusInternalServerError, "action unavailable")
			return
		}
		s.logComputation(r, session.UserID, "ap_calculation", "invalid_target", state.AP)
		s.logRejection(r, session.UserID, moveAction, moveReasonInvalidTarget)
		s.writeJSON(w, http.StatusBadRequest, moveResponse{Error: "invalid target", playerStateResponse: s.playerStateResponse(r, session.UserID, state)})
		return
	}
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "action unavailable")
		return
	}
	s.logComputation(r, session.UserID, "ap_calculation", "success", state.AP)
	s.logAction(r, session.UserID, moveAction, "success")
	s.writeJSON(w, http.StatusOK, s.playerStateResponse(r, session.UserID, state))
}

func (s *Server) authenticatedSession(r *http.Request) (Session, error) {
	cookie, err := r.Cookie(s.cfg.SessionCookieName)
	if err != nil || cookie.Value == "" {
		return Session{}, ErrSessionNotFound
	}
	return s.store.GetSession(cookie.Value)
}

func decodeMoveRequest(body io.Reader) (moveRequest, string) {
	decoder := json.NewDecoder(body)
	token, err := decoder.Token()
	if err != nil {
		return moveRequest{}, moveReasonInvalidJSON
	}
	delim, ok := token.(json.Delim)
	if !ok || delim != '{' {
		return moveRequest{}, moveReasonInvalidJSON
	}
	var request moveRequest
	seenTarget := false
	for decoder.More() {
		key, err := decoder.Token()
		if err != nil {
			return moveRequest{}, moveReasonInvalidJSON
		}
		field, ok := key.(string)
		if !ok {
			return moveRequest{}, moveReasonInvalidJSON
		}
		if field != "target" {
			return moveRequest{}, moveReasonUnknownField
		}
		if seenTarget {
			return moveRequest{}, moveReasonDuplicate
		}
		seenTarget = true
		if err := decoder.Decode(&request.Target); err != nil {
			return moveRequest{}, moveReasonInvalidTarget
		}
	}
	if token, err = decoder.Token(); err != nil {
		return moveRequest{}, moveReasonInvalidJSON
	} else if delim, ok = token.(json.Delim); !ok || delim != '}' {
		return moveRequest{}, moveReasonInvalidJSON
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return moveRequest{}, moveReasonExtraValue
	}
	if !seenTarget {
		return moveRequest{}, moveReasonMissingTarget
	}
	if strings.TrimSpace(request.Target) == "" {
		return moveRequest{}, moveReasonInvalidTarget
	}
	return request, ""
}

func decodeTransferRequest(body io.Reader) (transferRequest, string) {
	decoder := json.NewDecoder(body)
	token, err := decoder.Token()
	if err != nil {
		return transferRequest{}, transferReasonInvalidJSON
	}
	delim, ok := token.(json.Delim)
	if !ok || delim != '{' {
		return transferRequest{}, transferReasonInvalidJSON
	}
	var request transferRequest
	seenType, seenID, seenQuantity, seenStatus := false, false, false, false
	for decoder.More() {
		key, err := decoder.Token()
		if err != nil {
			return transferRequest{}, transferReasonInvalidJSON
		}
		field, ok := key.(string)
		if !ok {
			return transferRequest{}, transferReasonInvalidJSON
		}
		switch field {
		case "asset_type":
			if seenType {
				return transferRequest{}, transferReasonDuplicate
			}
			seenType = true
			if err := decoder.Decode(&request.AssetType); err != nil {
				return transferRequest{}, transferReasonInvalidAssetType
			}
		case "asset_id":
			if seenID {
				return transferRequest{}, transferReasonDuplicate
			}
			seenID = true
			if err := decoder.Decode(&request.AssetID); err != nil {
				return transferRequest{}, transferReasonInvalidAssetID
			}
		case "quantity":
			if seenQuantity {
				return transferRequest{}, transferReasonDuplicate
			}
			seenQuantity = true
			if err := decoder.Decode(&request.Quantity); err != nil {
				return transferRequest{}, transferReasonInvalidQuantity
			}
		case "item_status":
			if seenStatus {
				return transferRequest{}, transferReasonDuplicate
			}
			seenStatus = true
			if err := decoder.Decode(&request.ItemStatus); err != nil {
				return transferRequest{}, transferReasonInvalidItemStatus
			}
		default:
			var ignored json.RawMessage
			if err := decoder.Decode(&ignored); err != nil {
				return transferRequest{}, transferReasonInvalidJSON
			}
			return transferRequest{}, transferReasonUnknownField
		}
	}
	if token, err = decoder.Token(); err != nil {
		return transferRequest{}, transferReasonInvalidJSON
	} else if delim, ok = token.(json.Delim); !ok || delim != '}' {
		return transferRequest{}, transferReasonInvalidJSON
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return transferRequest{}, transferReasonExtraValue
	}
	if !seenType {
		return transferRequest{}, transferReasonMissingAssetType
	}
	if request.AssetType != "item" && request.AssetType != "resource" {
		return transferRequest{}, transferReasonInvalidAssetType
	}
	if !seenID {
		return transferRequest{}, transferReasonMissingAssetID
	}
	if strings.TrimSpace(request.AssetID) == "" {
		return transferRequest{}, transferReasonInvalidAssetID
	}
	if !seenQuantity {
		return transferRequest{}, transferReasonMissingQuantity
	}
	if request.Quantity <= 0 {
		return transferRequest{}, transferReasonInvalidQuantity
	}
	if request.AssetType == "resource" {
		if seenStatus {
			return transferRequest{}, transferReasonResourceItemStatus
		}
	} else {
		if !seenStatus {
			return transferRequest{}, transferReasonMissingItemStatus
		}
		if request.ItemStatus != "active" && request.ItemStatus != "expired" {
			return transferRequest{}, transferReasonInvalidItemStatus
		}
	}
	return request, ""
}

func decodeGatherRequest(body io.Reader) string {
	decoder := json.NewDecoder(body)
	token, err := decoder.Token()
	if err != nil {
		return gatherReasonInvalidJSON
	}
	delim, ok := token.(json.Delim)
	if !ok || delim != '{' {
		return gatherReasonInvalidJSON
	}
	seen := make(map[string]struct{})
	reason := ""
	for decoder.More() {
		key, err := decoder.Token()
		if err != nil {
			return gatherReasonInvalidJSON
		}
		field, ok := key.(string)
		if !ok {
			return gatherReasonInvalidJSON
		}
		if _, exists := seen[field]; exists {
			return gatherReasonDuplicate
		}
		seen[field] = struct{}{}
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return gatherReasonInvalidJSON
		}
		if reason == "" {
			reason = gatherReasonUnknownField
		}
	}
	if token, err = decoder.Token(); err != nil {
		return gatherReasonInvalidJSON
	} else if delim, ok = token.(json.Delim); !ok || delim != '}' {
		return gatherReasonInvalidJSON
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return gatherReasonExtraValue
	}
	return reason
}

type convertRequest struct {
	MethodID            string
	Quantity            int
	ProviderExtensionID int64
}

func decodeConvertRequest(body io.Reader) (convertRequest, string) {
	decoder := json.NewDecoder(body)
	token, err := decoder.Token()
	if err != nil {
		return convertRequest{}, convertReasonInvalidJSON
	}
	delim, ok := token.(json.Delim)
	if !ok || delim != '{' {
		return convertRequest{}, convertReasonInvalidJSON
	}
	var request convertRequest
	seen := make(map[string]bool)
	for decoder.More() {
		key, err := decoder.Token()
		if err != nil {
			return convertRequest{}, convertReasonInvalidJSON
		}
		field, ok := key.(string)
		if !ok {
			return convertRequest{}, convertReasonInvalidJSON
		}
		if seen[field] {
			return convertRequest{}, convertReasonDuplicate
		}
		seen[field] = true
		switch field {
		case "method_id":
			if err := decoder.Decode(&request.MethodID); err != nil {
				return convertRequest{}, convertReasonInvalidJSON
			}
		case "quantity":
			if err := decoder.Decode(&request.Quantity); err != nil {
				return convertRequest{}, convertReasonInvalidJSON
			}
		case "provider_extension_id":
			if err := decoder.Decode(&request.ProviderExtensionID); err != nil {
				return convertRequest{}, convertReasonInvalidJSON
			}
		default:
			var ignored json.RawMessage
			if err := decoder.Decode(&ignored); err != nil {
				return convertRequest{}, convertReasonInvalidJSON
			}
			return convertRequest{}, convertReasonUnknownField
		}
	}
	if token, err = decoder.Token(); err != nil {
		return convertRequest{}, convertReasonInvalidJSON
	} else if delim, ok = token.(json.Delim); !ok || delim != '}' {
		return convertRequest{}, convertReasonInvalidJSON
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return convertRequest{}, convertReasonExtraValue
	}
	if !seen["method_id"] || strings.TrimSpace(request.MethodID) == "" || !seen["quantity"] || request.Quantity <= 0 {
		return convertRequest{}, convertReasonInvalidQuantity
	}
	if request.ProviderExtensionID < 0 {
		return convertRequest{}, convertReasonInvalidQuantity
	}
	return request, ""
}

func resourceDelta(before, after PlayerState, methodID string, quantity int) int {
	for _, method := range after.ConversionMethods {
		if method.ID == methodID {
			beforeQ, afterQ := 0, 0
			for _, value := range before.Resources {
				if value.Resource.ID == method.OutputResource.ID {
					beforeQ = value.Quantity
				}
			}
			for _, value := range after.Resources {
				if value.Resource.ID == method.OutputResource.ID {
					afterQ = value.Quantity
				}
			}
			return afterQ - beforeQ
		}
	}
	return quantity
}

func itemDelta(before, after PlayerState, methodID string) int {
	var itemID string
	for _, method := range after.ConversionMethods {
		if method.ID == methodID && method.EssenceItem != nil {
			itemID = method.EssenceItem.ID
		}
	}
	if itemID == "" {
		return 0
	}
	quantity := func(state PlayerState) int {
		total := 0
		for _, item := range state.Inventory {
			if item.Item.ID == itemID && item.DurabilityStatus == "active" {
				total += item.Quantity
			}
		}
		return total
	}
	return quantity(after) - quantity(before)
}

type craftRequest struct {
	RecipeID string
}

func decodeBuildRequest(body io.Reader) (buildRequest, string) {
	decoder := json.NewDecoder(body)
	token, err := decoder.Token()
	if err != nil {
		return buildRequest{}, buildReasonInvalidJSON
	}
	delim, ok := token.(json.Delim)
	if !ok || delim != '{' {
		return buildRequest{}, buildReasonInvalidJSON
	}
	var request buildRequest
	seen := false
	for decoder.More() {
		key, err := decoder.Token()
		if err != nil {
			return buildRequest{}, buildReasonInvalidJSON
		}
		field, ok := key.(string)
		if !ok {
			return buildRequest{}, buildReasonInvalidJSON
		}
		if field != "recipe_id" {
			return buildRequest{}, buildReasonUnknownField
		}
		if seen {
			return buildRequest{}, buildReasonDuplicate
		}
		seen = true
		if err := decoder.Decode(&request.RecipeID); err != nil {
			return buildRequest{}, buildReasonInvalidRecipe
		}
	}
	if token, err = decoder.Token(); err != nil {
		return buildRequest{}, buildReasonInvalidJSON
	} else if delim, ok = token.(json.Delim); !ok || delim != '}' {
		return buildRequest{}, buildReasonInvalidJSON
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return buildRequest{}, buildReasonExtraValue
	}
	if !seen {
		return buildRequest{}, buildReasonMissingRecipe
	}
	if strings.TrimSpace(request.RecipeID) == "" {
		return buildRequest{}, buildReasonInvalidRecipe
	}
	return request, ""
}

func decodeContributeConstructionRequest(body io.Reader) (contributeConstructionRequest, string) {
	decoder := json.NewDecoder(body)
	token, err := decoder.Token()
	if err != nil {
		return contributeConstructionRequest{}, contributeReasonInvalidJSON
	}
	delim, ok := token.(json.Delim)
	if !ok || delim != '{' {
		return contributeConstructionRequest{}, contributeReasonInvalidJSON
	}
	var request contributeConstructionRequest
	seenBuilding, seenAP := false, false
	for decoder.More() {
		key, err := decoder.Token()
		if err != nil {
			return contributeConstructionRequest{}, contributeReasonInvalidJSON
		}
		field, ok := key.(string)
		if !ok {
			return contributeConstructionRequest{}, contributeReasonInvalidJSON
		}
		switch field {
		case "building_id":
			if seenBuilding {
				return contributeConstructionRequest{}, contributeReasonDuplicate
			}
			seenBuilding = true
			if err := decoder.Decode(&request.BuildingID); err != nil {
				return contributeConstructionRequest{}, contributeReasonInvalidBuilding
			}
		case "ap":
			if seenAP {
				return contributeConstructionRequest{}, contributeReasonDuplicate
			}
			seenAP = true
			if err := decoder.Decode(&request.AP); err != nil {
				return contributeConstructionRequest{}, contributeReasonInvalidAP
			}
		default:
			return contributeConstructionRequest{}, contributeReasonUnknownField
		}
	}
	if token, err = decoder.Token(); err != nil {
		return contributeConstructionRequest{}, contributeReasonInvalidJSON
	} else if delim, ok = token.(json.Delim); !ok || delim != '}' {
		return contributeConstructionRequest{}, contributeReasonInvalidJSON
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return contributeConstructionRequest{}, contributeReasonExtraValue
	}
	if !seenBuilding {
		return contributeConstructionRequest{}, contributeReasonMissingBuilding
	}
	if !seenAP {
		return contributeConstructionRequest{}, contributeReasonMissingAP
	}
	if request.BuildingID <= 0 {
		return contributeConstructionRequest{}, contributeReasonInvalidBuilding
	}
	if request.AP <= 0 {
		return contributeConstructionRequest{}, contributeReasonInvalidAP
	}
	return request, ""
}

func decodeRepairBuildingRequest(body io.Reader) (repairBuildingRequest, string) {
	decoder := json.NewDecoder(body)
	token, err := decoder.Token()
	if err != nil {
		return repairBuildingRequest{}, repairReasonInvalidJSON
	}
	delim, ok := token.(json.Delim)
	if !ok || delim != '{' {
		return repairBuildingRequest{}, repairReasonInvalidJSON
	}
	var request repairBuildingRequest
	seenBuilding := false
	for decoder.More() {
		key, err := decoder.Token()
		if err != nil {
			return repairBuildingRequest{}, repairReasonInvalidJSON
		}
		field, ok := key.(string)
		if !ok {
			return repairBuildingRequest{}, repairReasonInvalidJSON
		}
		if field != "building_id" {
			return repairBuildingRequest{}, repairReasonUnknownField
		}
		if seenBuilding {
			return repairBuildingRequest{}, repairReasonDuplicate
		}
		seenBuilding = true
		if err := decoder.Decode(&request.BuildingID); err != nil {
			return repairBuildingRequest{}, repairReasonInvalidBuilding
		}
	}
	if token, err = decoder.Token(); err != nil {
		return repairBuildingRequest{}, repairReasonInvalidJSON
	} else if delim, ok = token.(json.Delim); !ok || delim != '}' {
		return repairBuildingRequest{}, repairReasonInvalidJSON
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return repairBuildingRequest{}, repairReasonExtraValue
	}
	if !seenBuilding {
		return repairBuildingRequest{}, repairReasonMissingBuilding
	}
	if request.BuildingID <= 0 {
		return repairBuildingRequest{}, repairReasonInvalidBuilding
	}
	return request, ""
}

func decodeCraftRequest(body io.Reader) (craftRequest, string) {
	decoder := json.NewDecoder(body)
	token, err := decoder.Token()
	if err != nil {
		return craftRequest{}, craftReasonInvalidJSON
	}
	delim, ok := token.(json.Delim)
	if !ok || delim != '{' {
		return craftRequest{}, craftReasonInvalidJSON
	}
	var request craftRequest
	seen := false
	for decoder.More() {
		key, err := decoder.Token()
		if err != nil {
			return craftRequest{}, craftReasonInvalidJSON
		}
		field, ok := key.(string)
		if !ok {
			return craftRequest{}, craftReasonInvalidJSON
		}
		if field != "recipe_id" {
			return craftRequest{}, craftReasonUnknownField
		}
		if seen {
			return craftRequest{}, craftReasonDuplicate
		}
		seen = true
		if err := decoder.Decode(&request.RecipeID); err != nil {
			return craftRequest{}, craftReasonInvalidRecipe
		}
	}
	if token, err = decoder.Token(); err != nil {
		return craftRequest{}, craftReasonInvalidJSON
	} else if delim, ok = token.(json.Delim); !ok || delim != '}' {
		return craftRequest{}, craftReasonInvalidJSON
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return craftRequest{}, craftReasonExtraValue
	}
	if !seen {
		return craftRequest{}, craftReasonMissingRecipe
	}
	if strings.TrimSpace(request.RecipeID) == "" {
		return craftRequest{}, craftReasonInvalidRecipe
	}
	return request, ""
}

func locationResponseFromStore(location Location) locationResponse {
	return locationResponse{ID: location.ID, DisplayName: location.DisplayName}
}

func routeResponsesFromStore(routes []Route) []routeResponse {
	responses := make([]routeResponse, 0, len(routes))
	for _, route := range routes {
		responses = append(responses, routeResponse{OriginID: route.OriginID, DestinationID: route.DestinationID, APCost: route.APCost})
	}
	return responses
}

func playerStateResponseFromStore(state PlayerState) playerStateResponse {
	return playerStateResponse{
		Location:                     locationResponseFromStore(state.Location),
		Routes:                       routeResponsesFromStore(state.Routes),
		AP:                           state.AP,
		CarriedWeight:                state.CarriedWeight,
		MovementWeightThreshold:      state.MovementWeightThreshold,
		Inventory:                    inventoryResponsesFromStore(state.Inventory),
		GroundItems:                  groundItemResponsesFromStore(state.GroundItems),
		GroundResources:              groundResourceResponsesFromStore(state.GroundResources),
		GatheringOption:              gatheringOptionResponseFromStore(state.GatheringOption),
		ConversionOption:             conversionOptionResponseFromStore(state.ConversionOption),
		ConversionMethods:            conversionMethodResponsesFromStore(state.ConversionMethods),
		BuildingExtensionDefinitions: buildingExtensionDefinitionResponsesFromStore(state.BuildingExtensionDefinitions),
		Resources:                    resourceResponsesFromStore(state.Resources),
		CraftingRecipes:              craftingRecipeResponsesFromStore(state.CraftingRecipes),
		BuildingRecipes:              buildingRecipeResponsesFromStore(state.BuildingRecipes),
		Buildings:                    buildingResponsesFromStore(state.Buildings),
	}
}

func buildingExtensionDefinitionResponsesFromStore(definitions []BuildingExtensionDefinition) []buildingExtensionDefinitionResponse {
	responses := make([]buildingExtensionDefinitionResponse, 0, len(definitions))
	for _, definition := range definitions {
		responses = append(responses, buildingExtensionDefinitionResponse{ID: definition.ID, DisplayName: definition.DisplayName, Tier: definition.Tier, PackageItem: itemResponse{ID: definition.PackageItem.ID, DisplayName: definition.PackageItem.DisplayName}, RequiredAP: definition.RequiredAP})
	}
	return responses
}

func conversionMethodResponsesFromStore(methods []ConversionMethod) []conversionMethodResponse {
	responses := make([]conversionMethodResponse, 0, len(methods))
	for _, method := range methods {
		var essence *itemResponse
		if method.EssenceItem != nil {
			value := itemResponse{ID: method.EssenceItem.ID, DisplayName: method.EssenceItem.DisplayName}
			essence = &value
		}
		responses = append(responses, conversionMethodResponse{ID: method.ID, DisplayName: method.DisplayName, APCost: method.APCost, Input: itemResponse{ID: method.Input.ID, DisplayName: method.Input.DisplayName}, MaxInputQuantity: method.MaxInputQuantity, OutputResource: itemResponse{ID: method.OutputResource.ID, DisplayName: method.OutputResource.DisplayName}, ResourceQuantityPerInput: method.ResourceQuantityPerInput, EssenceItem: essence, EssenceChanceBPS: method.EssenceChanceBPS, EssenceQuantity: method.EssenceQuantity})
	}
	return responses
}

func (s *Server) playerStateResponse(r *http.Request, userID int64, state PlayerState) playerStateResponse {
	s.logCarryingWeightComputation(r, userID, state)
	s.logBuildingDurabilityComputation(r, userID, state.Buildings)
	s.logItemDurability(r, fmt.Sprintf("%d", userID), state)
	return playerStateResponseFromStore(state)
}

func groundItemResponsesFromStore(items []GroundItem) []groundItemResponse {
	responses := make([]groundItemResponse, 0, len(items))
	for _, item := range items {
		responses = append(responses, groundItemResponse{Item: itemResponse{ID: item.Item.ID, DisplayName: item.Item.DisplayName}, Quantity: item.Quantity, DurabilityStatus: item.DurabilityStatus, DurabilityRemainingSeconds: item.DurabilityRemainingSeconds, RetentionRemainingSeconds: item.RetentionRemainingSeconds})
	}
	return responses
}

func groundResourceResponsesFromStore(resources []GroundResource) []groundResourceResponse {
	responses := make([]groundResourceResponse, 0, len(resources))
	for _, resource := range resources {
		responses = append(responses, groundResourceResponse{
			Resource: itemResponse{ID: resource.Resource.ID, DisplayName: resource.Resource.DisplayName},
			Quantity: resource.Quantity,
		})
	}
	return responses
}

type buildingResourceInputResponse struct {
	Resource itemResponse `json:"resource"`
	Quantity int          `json:"quantity"`
}

type buildingItemInputResponse struct {
	Item     itemResponse `json:"item"`
	Quantity int          `json:"quantity"`
}

type buildingRecipeResponse struct {
	ID                 string                          `json:"id"`
	DisplayName        string                          `json:"display_name"`
	BuildingLevel      int                             `json:"building_level"`
	RequiredAP         int                             `json:"required_ap"`
	ExtensionSlotCount int                             `json:"extension_slot_count"`
	ResourceInputs     []buildingResourceInputResponse `json:"resource_inputs"`
	ItemInputs         []buildingItemInputResponse     `json:"item_inputs"`
}

type buildingResponse struct {
	ID                         int64                          `json:"id"`
	Owner                      buildingOwnerResponse          `json:"owner"`
	Recipe                     buildingRecipeIdentityResponse `json:"recipe"`
	BuildingLevel              int                            `json:"building_level"`
	RequiredAP                 int                            `json:"required_ap"`
	ContributedAP              int                            `json:"contributed_ap"`
	Status                     string                         `json:"status"`
	ExtensionSlotCount         int                            `json:"extension_slot_count"`
	MaxDurabilitySeconds       int                            `json:"max_durability_seconds"`
	DurabilityStatus           *string                        `json:"durability_status"`
	DurabilityRemainingSeconds *int                           `json:"durability_remaining_seconds"`
	Extensions                 []buildingExtensionResponse    `json:"extensions"`
}

type buildingExtensionResponse struct {
	ID            int64  `json:"id"`
	SlotIndex     int    `json:"slot_index"`
	DefinitionID  string `json:"definition_id"`
	DisplayName   string `json:"display_name"`
	Tier          int    `json:"tier"`
	RequiredAP    int    `json:"required_ap"`
	ContributedAP int    `json:"contributed_ap"`
	Status        string `json:"status"`
}

type buildingOwnerResponse struct {
	ID          int64  `json:"id"`
	DisplayName string `json:"display_name"`
}

type buildingRecipeIdentityResponse struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
}

func buildingRecipeResponsesFromStore(recipes []BuildingRecipe) []buildingRecipeResponse {
	responses := make([]buildingRecipeResponse, 0, len(recipes))
	for _, recipe := range recipes {
		responses = append(responses, buildingRecipeResponseFromStore(recipe))
	}
	return responses
}

func buildingRecipeResponseFromStore(recipe BuildingRecipe) buildingRecipeResponse {
	resourceInputs := make([]buildingResourceInputResponse, 0, len(recipe.ResourceInputs))
	for _, input := range recipe.ResourceInputs {
		resourceInputs = append(resourceInputs, buildingResourceInputResponse{
			Resource: itemResponse{ID: input.Resource.ID, DisplayName: input.Resource.DisplayName},
			Quantity: input.Quantity,
		})
	}
	itemInputs := make([]buildingItemInputResponse, 0, len(recipe.ItemInputs))
	for _, input := range recipe.ItemInputs {
		itemInputs = append(itemInputs, buildingItemInputResponse{
			Item:     itemResponse{ID: input.Item.ID, DisplayName: input.Item.DisplayName},
			Quantity: input.Quantity,
		})
	}
	return buildingRecipeResponse{
		ID:                 recipe.ID,
		DisplayName:        recipe.DisplayName,
		BuildingLevel:      recipe.BuildingLevel,
		RequiredAP:         recipe.RequiredAP,
		ExtensionSlotCount: recipe.ExtensionSlotCount,
		ResourceInputs:     resourceInputs,
		ItemInputs:         itemInputs,
	}
}

func buildingResponsesFromStore(buildings []Building) []buildingResponse {
	responses := make([]buildingResponse, 0, len(buildings))
	for _, building := range buildings {
		responses = append(responses, buildingResponseFromStore(building))
	}
	return responses
}

func buildingResponseFromStore(building Building) buildingResponse {
	response := buildingResponse{
		ID: building.ID,
		Owner: buildingOwnerResponse{
			ID:          building.Owner.ID,
			DisplayName: building.Owner.DisplayName,
		},
		Recipe: buildingRecipeIdentityResponse{
			ID:          building.Recipe.ID,
			DisplayName: building.Recipe.DisplayName,
		},
		BuildingLevel:        building.BuildingLevel,
		RequiredAP:           building.RequiredAP,
		ContributedAP:        building.ContributedAP,
		Status:               building.Status,
		ExtensionSlotCount:   building.ExtensionSlotCount,
		MaxDurabilitySeconds: building.MaxDurabilitySeconds,
		Extensions:           make([]buildingExtensionResponse, 0, len(building.Extensions)),
	}
	for _, extension := range building.Extensions {
		response.Extensions = append(response.Extensions, buildingExtensionResponse{ID: extension.ID, SlotIndex: extension.SlotIndex, DefinitionID: extension.DefinitionID, DisplayName: extension.DisplayName, Tier: extension.Tier, RequiredAP: extension.RequiredAP, ContributedAP: extension.ContributedAP, Status: extension.Status})
	}
	if building.DurabilityStatus != "" {
		status := building.DurabilityStatus
		remaining := building.DurabilityRemainingSeconds
		response.DurabilityStatus = &status
		response.DurabilityRemainingSeconds = &remaining
	}
	return response
}

func craftingRecipeResponsesFromStore(recipes []CraftingRecipe) []craftingRecipeResponse {
	responses := make([]craftingRecipeResponse, 0, len(recipes))
	for _, recipe := range recipes {
		resourceInputs := make([]craftingResourceInputResponse, 0, len(recipe.ResourceInputs))
		for _, input := range recipe.ResourceInputs {
			resourceInputs = append(resourceInputs, craftingResourceInputResponse{
				Resource: itemResponse{ID: input.Resource.ID, DisplayName: input.Resource.DisplayName},
				Quantity: input.Quantity,
			})
		}
		itemInputs := make([]craftingItemInputResponse, 0, len(recipe.ItemInputs))
		for _, input := range recipe.ItemInputs {
			itemInputs = append(itemInputs, craftingItemInputResponse{
				Item:     itemResponse{ID: input.Item.ID, DisplayName: input.Item.DisplayName},
				Quantity: input.Quantity,
			})
		}
		responses = append(responses, craftingRecipeResponse{
			ID: recipe.ID, DisplayName: recipe.DisplayName, BaseAPCost: recipe.BaseAPCost,
			ResourceInputs: resourceInputs, ItemInputs: itemInputs,
			Output:         itemResponse{ID: recipe.Output.ID, DisplayName: recipe.Output.DisplayName},
			OutputQuantity: recipe.OutputQuantity,
		})
	}
	return responses
}

func inventoryResponsesFromStore(items []InventoryItem) []inventoryItemResponse {
	responses := make([]inventoryItemResponse, 0, len(items))
	for _, item := range items {
		responses = append(responses, inventoryItemResponse{Item: itemResponse{ID: item.Item.ID, DisplayName: item.Item.DisplayName}, Quantity: item.Quantity, DurabilityStatus: item.DurabilityStatus, DurabilityRemainingSeconds: item.DurabilityRemainingSeconds, RetentionRemainingSeconds: item.RetentionRemainingSeconds})
	}
	return responses
}

func gatheringOptionResponseFromStore(option *GatheringOption) *gatheringOptionResponse {
	if option == nil {
		return nil
	}
	return &gatheringOptionResponse{
		Item:     itemResponse{ID: option.Item.ID, DisplayName: option.Item.DisplayName},
		Quantity: option.Quantity,
		APCost:   option.APCost,
	}
}

type conversionOptionResponse struct {
	Item          itemResponse `json:"item"`
	Resource      itemResponse `json:"resource"`
	InputQuantity int          `json:"input_quantity"`
	ResourceYield int          `json:"resource_yield"`
	APCost        int          `json:"ap_cost"`
}

func conversionOptionResponseFromStore(option *ConversionOption) *conversionOptionResponse {
	if option == nil {
		return nil
	}
	return &conversionOptionResponse{
		Item:          itemResponse{ID: option.Item.ID, DisplayName: option.Item.DisplayName},
		Resource:      itemResponse{ID: option.Resource.ID, DisplayName: option.Resource.DisplayName},
		InputQuantity: option.InputQuantity,
		ResourceYield: option.ResourceYield,
		APCost:        option.APCost,
	}
}

type resourceResponse struct {
	Resource itemResponse `json:"resource"`
	Quantity int          `json:"quantity"`
}

func resourceResponsesFromStore(resources []PlayerResource) []resourceResponse {
	responses := make([]resourceResponse, 0, len(resources))
	for _, resource := range resources {
		responses = append(responses, resourceResponse{
			Resource: itemResponse{ID: resource.Resource.ID, DisplayName: resource.Resource.DisplayName},
			Quantity: resource.Quantity,
		})
	}
	return responses
}

func (s *Server) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" && origin != s.frontendOrigin && r.Method == http.MethodPost {
			s.writeError(w, http.StatusForbidden, "origin not allowed")
			return
		}
		if origin == s.frontendOrigin {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Add("Vary", "Origin")
			if r.Method == http.MethodOptions {
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
				w.WriteHeader(http.StatusNoContent)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := strings.TrimSpace(r.Header.Get("X-Request-ID"))
		if !validRequestID(requestID) {
			requestID, _ = randomString(16)
			if requestID == "" {
				requestID = "unavailable"
			}
		}
		w.Header().Set("X-Request-ID", requestID)
		ctx := context.WithValue(r.Context(), requestIDContextKey{}, requestID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *Server) accessLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		recorder := &statusRecorder{ResponseWriter: w}
		next.ServeHTTP(recorder, r)
		userID := "anonymous"
		if cookie, err := r.Cookie(s.cfg.SessionCookieName); err == nil && cookie.Value != "" {
			if session, err := s.store.GetSession(cookie.Value); err == nil {
				userID = fmt.Sprintf("%d", session.UserID)
			}
		}
		status := recorder.status
		if status == 0 {
			status = http.StatusOK
		}
		outcome := "success"
		if status >= http.StatusBadRequest {
			outcome = http.StatusText(status)
		}
		s.logActionWithID(requestID(r), userID, accessLogAction(r), outcome)
	})
}

func accessLogAction(r *http.Request) string {
	if strings.HasPrefix(r.URL.Path, "/api/actions/") {
		switch r.URL.Path {
		case "/api/actions/rest":
			return "rest"
		case "/api/actions/move":
			return "move"
		case "/api/actions/gather":
			return "gather"
		case "/api/actions/convert":
			return "convert"
		case "/api/actions/craft":
			return "craft"
		case "/api/actions/build":
			return "build"
		case "/api/actions/contribute-construction":
			return "contribute-construction"
		case "/api/actions/repair-building":
			return "repair-building"
		case "/api/actions/install-extension":
			return "install-extension"
		case "/api/actions/contribute-extension-construction":
			return "contribute-extension-construction"
		case "/api/actions/remove-extension":
			return "remove-extension"
		default:
			return "unknown"
		}
	}
	if r.URL.Path == "/api/transfers/drop" {
		return "transfer-drop"
	}
	if r.URL.Path == "/api/transfers/pickup" {
		return "transfer-pickup"
	}
	return r.Method + " " + r.URL.Path
}

type requestIDContextKey struct{}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (w *statusRecorder) WriteHeader(status int) {
	if w.status != 0 {
		return
	}
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusRecorder) Write(body []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(body)
}

func requestID(r *http.Request) string {
	if value, ok := r.Context().Value(requestIDContextKey{}).(string); ok && value != "" {
		return value
	}
	return "unavailable"
}

func validRequestID(value string) bool {
	if value == "" || len(value) > 128 {
		return false
	}
	for _, char := range value {
		if (char < 'a' || char > 'z') && (char < 'A' || char > 'Z') && (char < '0' || char > '9') && char != '-' && char != '_' && char != '.' {
			return false
		}
	}
	return true
}

func (s *Server) logAction(r *http.Request, userID int64, action, outcome string) {
	s.logActionWithID(requestID(r), fmt.Sprintf("%d", userID), action, outcome)
}

func (s *Server) logActionWithID(requestID, userID, action, outcome string) {
	fmt.Fprintf(os.Stdout, "user_id=%s action=%s outcome=%s request_id=%s\n", userID, action, outcome, requestID)
}

func (s *Server) logRejection(r *http.Request, userID int64, action, reason string) {
	s.logRejectionWithID(requestID(r), fmt.Sprintf("%d", userID), action, reason)
}

func (s *Server) logRejectionWithID(requestID, userID, action, reason string) {
	fmt.Fprintf(os.Stdout, "user_id=%s action=%s outcome=error reason=%s request_id=%s\n", userID, action, reason, requestID)
}

func (s *Server) logComputation(r *http.Request, userID int64, action, outcome string, ap int) {
	fmt.Fprintf(os.Stdout, "user_id=%d action=%s outcome=%s ap=%d request_id=%s\n", userID, action, outcome, ap, requestID(r))
}

func (s *Server) logConvertComputation(r *http.Request, userID int64, request convertRequest, resourceQuantity, essenceQuantity int, outcome string) {
	fmt.Fprintf(os.Stdout, "user_id=%d action=convert method_id=%s quantity=%d resource_quantity=%d essence_quantity=%d essence_result=reported outcome=%s request_id=%s\n", userID, sanitizeLogValue(request.MethodID), request.Quantity, resourceQuantity, essenceQuantity, sanitizeLogValue(outcome), requestID(r))
}

func (s *Server) logExtensionAction(r *http.Request, userID int64, action string, buildingID, extensionID int64, ap int, outcome string, computation *ExtensionConstructionComputation) {
	format := "user_id=%d action=%s building_id=%d extension_id=%d ap=%d outcome=%s request_id=%s"
	if computation != nil {
		format += " requested_ap=%d effective_ap=%d resulting_progress=%d/%d status=%s"
		fmt.Fprintf(os.Stdout, format+"\n", userID, sanitizeLogValue(action), buildingID, extensionID, ap, sanitizeLogValue(outcome), requestID(r), computation.RequestedAP, computation.EffectiveAP, computation.ResultingProgress, computation.RequiredAP, sanitizeLogValue(computation.ResultingStatus))
		return
	}
	fmt.Fprintf(os.Stdout, format+"\n", userID, sanitizeLogValue(action), buildingID, extensionID, ap, sanitizeLogValue(outcome), requestID(r))
}

func (s *Server) logCarryingWeightComputation(r *http.Request, userID int64, state PlayerState) {
	fmt.Fprintf(os.Stdout, "user_id=%d action=carrying_weight_calculation outcome=success carried_weight=%d movement_weight_threshold=%d request_id=%s\n", userID, state.CarriedWeight, state.MovementWeightThreshold, requestID(r))
}

func (s *Server) logWeightRejection(r *http.Request, userID int64, action, reason string, state PlayerState) {
	fmt.Fprintf(os.Stdout, "user_id=%d action=%s outcome=error reason=%s carried_weight=%d movement_weight_threshold=%d request_id=%s\n", userID, action, reason, state.CarriedWeight, state.MovementWeightThreshold, requestID(r))
}

func (s *Server) logTransfer(r *http.Request, userID int64, operation, locationID, assetType, assetID string, quantity int, outcome, reason string, itemStatus ...string) {
	assetID = sanitizeLogValue(assetID)
	if locationID == "" {
		locationID = "unknown"
	}
	if assetType != "item" && assetType != "resource" {
		assetType = "unknown"
	}
	if reason == "" {
		reason = "none"
	}
	if assetType == "item" && len(itemStatus) == 1 {
		fmt.Fprintf(os.Stdout, "user_id=%d action=transfer-%s location_id=%s asset_type=%s asset_id=%s quantity=%d outcome=%s reason=%s request_id=%s item_status=%s\n", userID, operation, sanitizeLogValue(locationID), assetType, assetID, quantity, outcome, reason, requestID(r), sanitizeLogValue(itemStatus[0]))
		return
	}
	fmt.Fprintf(os.Stdout, "user_id=%d action=transfer-%s location_id=%s asset_type=%s asset_id=%s quantity=%d outcome=%s reason=%s request_id=%s\n", userID, operation, sanitizeLogValue(locationID), assetType, assetID, quantity, outcome, reason, requestID(r))
}

func sanitizeLogValue(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 64 {
		return "unknown"
	}
	for _, char := range value {
		if (char < 'a' || char > 'z') && (char < 'A' || char > 'Z') && (char < '0' || char > '9') && char != '-' && char != '_' && char != '.' {
			return "unknown"
		}
	}
	return value
}

func (s *Server) logConstructionComputation(r *http.Request, userID int64, computation *ConstructionComputation) {
	fmt.Fprintf(os.Stdout, "user_id=%d action=%s outcome=success building_id=%d effective_ap=%d resulting_progress=%d/%d completion=%s request_id=%s\n", userID, contributeConstructionAction, computation.BuildingID, computation.EffectiveAP, computation.ResultingProgress, computation.RequiredAP, computation.CompletionOutcome, requestID(r))
}

func (s *Server) logRepairComputation(r *http.Request, userID int64, computation *RepairComputation) {
	fmt.Fprintf(os.Stdout, "user_id=%d action=%s outcome=success building_id=%d prior_durability_status=%s added_seconds=%d resulting_remaining_seconds=%d ap_cost=%d wood_cost=%d request_id=%s\n", userID, repairBuildingAction, computation.BuildingID, computation.PriorDurabilityStatus, computation.AddedSeconds, computation.ResultingRemainingSeconds, computation.APCost, computation.WoodCost, requestID(r))
}

func (s *Server) logBuildingDurabilityComputation(r *http.Request, userID int64, buildings []Building) {
	for _, building := range buildings {
		if building.Status != "completed" || building.DurabilityStatus == "" {
			continue
		}
		fmt.Fprintf(os.Stdout, "user_id=%d action=building_durability_calculation outcome=success building_id=%d durability_status=%s remaining_seconds=%d request_id=%s\n", userID, building.ID, building.DurabilityStatus, building.DurabilityRemainingSeconds, requestID(r))
	}
}

func (s *Server) logItemDurability(r *http.Request, userID string, state PlayerState) {
	for _, computation := range state.ItemDurabilityComputations {
		durabilityRemaining := "null"
		if computation.DurabilityRemainingSeconds != nil {
			durabilityRemaining = fmt.Sprintf("%d", *computation.DurabilityRemainingSeconds)
		}
		retentionRemaining := "null"
		if computation.RetentionRemainingSeconds != nil {
			retentionRemaining = fmt.Sprintf("%d", *computation.RetentionRemainingSeconds)
		}
		fmt.Fprintf(os.Stdout, "user_id=%s action=item_durability_calculation outcome=success holding=%s item_id=%s quantity=%d durability_status=%s durability_remaining_seconds=%s retention_remaining_seconds=%s request_id=%s\n", userID, sanitizeLogValue(computation.Holding), sanitizeLogValue(computation.ItemID), computation.Quantity, sanitizeLogValue(computation.DurabilityStatus), durabilityRemaining, retentionRemaining, requestID(r))
	}
	for _, cleanup := range state.ItemDurabilityCleanups {
		fmt.Fprintf(os.Stdout, "user_id=%s action=item_durability_cleanup outcome=success holding=%s item_id=%s quantity=%d cleanup_action=%s expired_at=%d retention_expires_at=%d request_id=%s\n", userID, sanitizeLogValue(cleanup.Holding), sanitizeLogValue(cleanup.ItemID), cleanup.Quantity, sanitizeLogValue(cleanup.Action), cleanup.ExpiredAt, cleanup.RetentionExpiresAt, requestID(r))
	}
}

func canonicalOrigin(frontend *url.URL) (string, error) {
	scheme := strings.ToLower(frontend.Scheme)
	if scheme != "http" && scheme != "https" {
		return "", errors.New("auth server requires an HTTP frontend URL")
	}
	hostname := strings.ToLower(frontend.Hostname())
	if hostname == "" {
		return "", errors.New("auth server requires a frontend host")
	}
	port := frontend.Port()
	if (scheme == "http" && port == "80") || (scheme == "https" && port == "443") {
		port = ""
	}
	host := hostname
	if strings.Contains(host, ":") {
		host = "[" + host + "]"
	}
	if port != "" {
		host += ":" + port
	}
	return scheme + "://" + host, nil
}

func (s *Server) writeNotFound(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/actions/") {
		userID := "anonymous"
		if session, err := s.authenticatedSession(r); err == nil {
			userID = fmt.Sprintf("%d", session.UserID)
		}
		s.logRejectionWithID(requestID(r), userID, "unknown", moveReasonUnsupported)
	}
	s.writeError(w, http.StatusNotFound, "not found")
}

func (s *Server) writeMethodNotAllowed(w http.ResponseWriter, _ *http.Request) {
	s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
}

func (s *Server) writeError(w http.ResponseWriter, status int, message string) {
	s.writeJSON(w, status, struct {
		Error string `json:"error"`
	}{message})
}

func (s *Server) writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func randomString(size int) (string, error) {
	b := make([]byte, size)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func pkceChallenge(verifier string) string {
	digest := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(digest[:])
}

func sameSecret(left, right string) bool {
	if left == "" || right == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(left), []byte(right)) == 1
}
