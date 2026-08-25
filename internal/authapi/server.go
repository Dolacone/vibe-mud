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

func (s *Server) Routes() http.Handler {
	r := chi.NewRouter()
	r.Use(s.requestID)
	r.Use(s.accessLog)
	r.Use(s.cors)
	r.NotFound(s.writeNotFound)
	r.MethodNotAllowed(s.writeMethodNotAllowed)
	r.Get("/auth/google/login", s.login)
	r.Get("/auth/google/callback", s.callback)
	r.Get("/api/me", s.me)
	r.Post("/api/actions/rest", s.rest)
	return r
}

func (s *Server) Handler() http.Handler {
	return s.Routes()
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
	ap, err := s.store.GetAP(identity.ID)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "current user unavailable")
		return
	}
	s.logComputation(r, identity.ID, "ap_calculation", "success", ap)
	s.writeJSON(w, http.StatusOK, struct {
		ID          int64  `json:"id"`
		DisplayName string `json:"display_name"`
		Email       string `json:"email"`
		AP          int    `json:"ap"`
	}{identity.ID, identity.DisplayName, identity.Email, ap})
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
		s.writeJSON(w, http.StatusConflict, struct {
			Error string `json:"error"`
			AP    int    `json:"ap"`
		}{err.Error(), ap})
		return
	}
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "action unavailable")
		return
	}
	s.logComputation(r, session.UserID, "ap_calculation", "success", ap)
	s.logAction(r, session.UserID, "rest", "success")
	s.writeJSON(w, http.StatusOK, struct {
		AP int `json:"ap"`
	}{ap})
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
		s.logActionWithID(requestID(r), userID, r.Method+" "+r.URL.Path, outcome)
	})
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

func (s *Server) writeNotFound(w http.ResponseWriter, _ *http.Request) {
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
