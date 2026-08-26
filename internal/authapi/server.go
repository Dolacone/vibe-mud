package authapi

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
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
	ID               int64                     `json:"id"`
	DisplayName      string                    `json:"display_name"`
	Email            string                    `json:"email"`
	AP               int                       `json:"ap"`
	Location         locationResponse          `json:"location"`
	Routes           []routeResponse           `json:"routes"`
	Inventory        []inventoryItemResponse   `json:"inventory"`
	GatheringOption  *gatheringOptionResponse  `json:"gathering_option"`
	ConversionOption *conversionOptionResponse `json:"conversion_option"`
	Resource         int                       `json:"resource"`
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
	Location         locationResponse          `json:"location"`
	Routes           []routeResponse           `json:"routes"`
	AP               int                       `json:"ap"`
	Inventory        []inventoryItemResponse   `json:"inventory"`
	GatheringOption  *gatheringOptionResponse  `json:"gathering_option"`
	ConversionOption *conversionOptionResponse `json:"conversion_option"`
	Resource         int                       `json:"resource"`
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
	Item     itemResponse `json:"item"`
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
	Error string `json:"error,omitempty"`
	playerStateResponse
}

const (
	moveAction                    = "move"
	moveReasonInvalidJSON         = "invalid_json"
	moveReasonUnknownField        = "unknown_field"
	moveReasonDuplicate           = "duplicate_field"
	moveReasonExtraValue          = "extra_json_value"
	moveReasonMissingTarget       = "missing_target"
	moveReasonInvalidTarget       = "invalid_target"
	moveReasonUnsupported         = "unsupported_action"
	gatherAction                  = "gather"
	gatherReasonInvalidJSON       = "invalid_json"
	gatherReasonUnknownField      = "unknown_field"
	gatherReasonDuplicate         = "duplicate_field"
	gatherReasonExtraValue        = "extra_json_value"
	gatherReasonInvalidLocation   = "invalid_location"
	convertAction                 = "convert"
	convertReasonInvalidJSON      = "invalid_json"
	convertReasonUnknownField     = "unknown_field"
	convertReasonDuplicate        = "duplicate_field"
	convertReasonExtraValue       = "extra_json_value"
	convertReasonInsufficientAP   = "insufficient_ap"
	convertReasonInvalidLocation  = "invalid_location"
	convertReasonInsufficientItem = "insufficient_item"
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
	response := currentUserResponse{
		ID:               identity.ID,
		DisplayName:      identity.DisplayName,
		Email:            identity.Email,
		AP:               state.AP,
		Location:         locationResponseFromStore(state.Location),
		Routes:           routeResponsesFromStore(state.Routes),
		Inventory:        inventoryResponsesFromStore(state.Inventory),
		GatheringOption:  gatheringOptionResponseFromStore(state.GatheringOption),
		ConversionOption: conversionOptionResponseFromStore(state.ConversionOption),
		Resource:         state.Resource,
	}
	s.writeJSON(w, http.StatusOK, response)
}

func (s *Server) convert(w http.ResponseWriter, r *http.Request) {
	session, err := s.authenticatedSession(r)
	if err != nil {
		s.writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	if reason := decodeConvertRequest(r.Body); reason != "" {
		s.logRejection(r, session.UserID, convertAction, reason)
		state, stateErr := s.store.GetPlayerState(session.UserID)
		if stateErr != nil {
			s.writeError(w, http.StatusInternalServerError, "action unavailable")
			return
		}
		s.writeJSON(w, http.StatusBadRequest, convertResponse{Error: "invalid action input", playerStateResponse: playerStateResponseFromStore(state)})
		return
	}
	state, err := s.store.Convert(session.UserID)
	if errors.Is(err, ErrInsufficientAP) {
		state, stateErr := s.store.GetPlayerState(session.UserID)
		if stateErr != nil {
			s.writeError(w, http.StatusInternalServerError, "action unavailable")
			return
		}
		s.logComputation(r, session.UserID, "ap_calculation", "insufficient_ap", state.AP)
		s.logRejection(r, session.UserID, convertAction, convertReasonInsufficientAP)
		s.writeJSON(w, http.StatusConflict, convertResponse{Error: ErrInsufficientAP.Error(), playerStateResponse: playerStateResponseFromStore(state)})
		return
	}
	if errors.Is(err, ErrInsufficientItem) {
		state, stateErr := s.store.GetPlayerState(session.UserID)
		if stateErr != nil {
			s.writeError(w, http.StatusInternalServerError, "action unavailable")
			return
		}
		s.logRejection(r, session.UserID, convertAction, convertReasonInsufficientItem)
		s.writeJSON(w, http.StatusConflict, convertResponse{Error: ErrInsufficientItem.Error(), playerStateResponse: playerStateResponseFromStore(state)})
		return
	}
	if errors.Is(err, ErrConversionNotFound) {
		state, stateErr := s.store.GetPlayerState(session.UserID)
		if stateErr != nil {
			s.writeError(w, http.StatusInternalServerError, "action unavailable")
			return
		}
		s.logRejection(r, session.UserID, convertAction, convertReasonInvalidLocation)
		s.writeJSON(w, http.StatusBadRequest, convertResponse{Error: ErrConversionNotFound.Error(), playerStateResponse: playerStateResponseFromStore(state)})
		return
	}
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "action unavailable")
		return
	}
	s.logComputation(r, session.UserID, "ap_calculation", "success", state.AP)
	s.logAction(r, session.UserID, convertAction, "success")
	s.writeJSON(w, http.StatusOK, playerStateResponseFromStore(state))
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
		s.writeJSON(w, http.StatusBadRequest, gatherResponse{Error: "invalid action input", playerStateResponse: playerStateResponseFromStore(state)})
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
		s.writeJSON(w, http.StatusConflict, gatherResponse{Error: ErrInsufficientAP.Error(), playerStateResponse: playerStateResponseFromStore(state)})
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
		s.writeJSON(w, http.StatusBadRequest, gatherResponse{Error: ErrGatheringNotFound.Error(), playerStateResponse: playerStateResponseFromStore(state)})
		return
	}
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "action unavailable")
		return
	}
	s.logComputation(r, session.UserID, "ap_calculation", "success", state.AP)
	s.logAction(r, session.UserID, gatherAction, "success")
	s.writeJSON(w, http.StatusOK, playerStateResponseFromStore(state))
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
		s.writeJSON(w, http.StatusConflict, moveResponse{Error: ErrInsufficientAP.Error(), playerStateResponse: playerStateResponseFromStore(state)})
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
		s.writeJSON(w, http.StatusBadRequest, moveResponse{Error: "invalid target", playerStateResponse: playerStateResponseFromStore(state)})
		return
	}
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "action unavailable")
		return
	}
	s.logComputation(r, session.UserID, "ap_calculation", "success", state.AP)
	s.logAction(r, session.UserID, moveAction, "success")
	s.writeJSON(w, http.StatusOK, playerStateResponseFromStore(state))
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

func decodeConvertRequest(body io.Reader) string {
	decoder := json.NewDecoder(body)
	token, err := decoder.Token()
	if err != nil {
		return convertReasonInvalidJSON
	}
	delim, ok := token.(json.Delim)
	if !ok || delim != '{' {
		return convertReasonInvalidJSON
	}
	seen := make(map[string]struct{})
	reason := ""
	for decoder.More() {
		key, err := decoder.Token()
		if err != nil {
			return convertReasonInvalidJSON
		}
		field, ok := key.(string)
		if !ok {
			return convertReasonInvalidJSON
		}
		if _, exists := seen[field]; exists {
			return convertReasonDuplicate
		}
		seen[field] = struct{}{}
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return convertReasonInvalidJSON
		}
		if reason == "" {
			reason = convertReasonUnknownField
		}
	}
	if token, err = decoder.Token(); err != nil {
		return convertReasonInvalidJSON
	} else if delim, ok = token.(json.Delim); !ok || delim != '}' {
		return convertReasonInvalidJSON
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return convertReasonExtraValue
	}
	return reason
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
		Location:         locationResponseFromStore(state.Location),
		Routes:           routeResponsesFromStore(state.Routes),
		AP:               state.AP,
		Inventory:        inventoryResponsesFromStore(state.Inventory),
		GatheringOption:  gatheringOptionResponseFromStore(state.GatheringOption),
		ConversionOption: conversionOptionResponseFromStore(state.ConversionOption),
		Resource:         state.Resource,
	}
}

func inventoryResponsesFromStore(items []InventoryItem) []inventoryItemResponse {
	responses := make([]inventoryItemResponse, 0, len(items))
	for _, item := range items {
		responses = append(responses, inventoryItemResponse{Item: itemResponse{ID: item.Item.ID, DisplayName: item.Item.DisplayName}, Quantity: item.Quantity})
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
		InputQuantity: option.InputQuantity,
		ResourceYield: option.ResourceYield,
		APCost:        option.APCost,
	}
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
		default:
			return "unknown"
		}
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
