package authapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

func TestNewGoogleProviderRejectsIncompleteConfigBeforeDiscovery(t *testing.T) {
	if _, err := NewGoogleProvider(context.Background(), GoogleConfig{}); err == nil {
		t.Fatal("incomplete Google configuration was accepted")
	}
}

func TestGoogleAuthorizationURLIncludesNonceAndS256PKCE(t *testing.T) {
	provider := &GoogleProvider{
		oauth:    oauth2.Config{Endpoint: oauth2.Endpoint{AuthURL: "https://accounts.google.com/o/oauth2/auth"}},
		verifier: &oidc.IDTokenVerifier{},
	}

	got, err := provider.AuthorizationURL("state-value", "nonce-value", "challenge-value")
	if err != nil {
		t.Fatalf("AuthorizationURL() error = %v", err)
	}
	values, err := url.ParseQuery(strings.SplitN(got, "?", 2)[1])
	if err != nil {
		t.Fatalf("parse authorization URL: %v", err)
	}
	if values.Get("nonce") != "nonce-value" {
		t.Fatalf("nonce = %q", values.Get("nonce"))
	}
	if values.Get("code_challenge") != "challenge-value" {
		t.Fatalf("code_challenge = %q", values.Get("code_challenge"))
	}
	if values.Get("code_challenge_method") != "S256" {
		t.Fatalf("code_challenge_method = %q", values.Get("code_challenge_method"))
	}
}

func TestGoogleExchangeRejectsMissingIDToken(t *testing.T) {
	provider := testGoogleProvider(t, map[string]any{"access_token": "access-token", "token_type": "Bearer"})

	_, err := provider.Exchange(context.Background(), "authorization-code", "pkce-verifier")
	if err == nil || !strings.Contains(err.Error(), "no ID token") {
		t.Fatalf("Exchange() error = %v, want missing ID token rejection", err)
	}
}

func TestGoogleExchangeRejectsInvalidIDToken(t *testing.T) {
	provider := testGoogleProvider(t, map[string]any{"access_token": "access-token", "token_type": "Bearer", "id_token": "not-a-jwt"})

	_, err := provider.Exchange(context.Background(), "authorization-code", "pkce-verifier")
	if err == nil || !strings.Contains(err.Error(), "verify Google ID token") {
		t.Fatalf("Exchange() error = %v, want invalid ID token rejection", err)
	}
}

func testGoogleProvider(t *testing.T, tokenResponse map[string]any) *GoogleProvider {
	t.Helper()
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(tokenResponse)
	}))
	t.Cleanup(tokenServer.Close)
	return &GoogleProvider{
		oauth:    oauth2.Config{Endpoint: oauth2.Endpoint{TokenURL: tokenServer.URL}},
		verifier: oidc.NewVerifier("https://accounts.google.com", testKeySet{}, &oidc.Config{ClientID: "client-id"}),
	}
}

type testKeySet struct{}

func (testKeySet) VerifySignature(_ context.Context, _ string) ([]byte, error) {
	return nil, errors.New("invalid signature")
}
