package authapi

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

type GoogleConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
}

type GoogleProvider struct {
	oauth    oauth2.Config
	verifier *oidc.IDTokenVerifier
}

func NewGoogleProvider(ctx context.Context, cfg GoogleConfig) (*GoogleProvider, error) {
	if strings.TrimSpace(cfg.ClientID) == "" || strings.TrimSpace(cfg.ClientSecret) == "" || strings.TrimSpace(cfg.RedirectURL) == "" {
		return nil, errors.New("google oauth requires client ID, client secret, and redirect URL")
	}
	provider, err := oidc.NewProvider(ctx, "https://accounts.google.com")
	if err != nil {
		return nil, fmt.Errorf("discover Google OIDC provider: %w", err)
	}
	return &GoogleProvider{
		oauth: oauth2.Config{
			ClientID: cfg.ClientID, ClientSecret: cfg.ClientSecret, Endpoint: provider.Endpoint(), RedirectURL: cfg.RedirectURL,
			Scopes: []string{oidc.ScopeOpenID, "email", "profile"},
		},
		verifier: provider.Verifier(&oidc.Config{ClientID: cfg.ClientID}),
	}, nil
}

func (p *GoogleProvider) AuthorizationURL(state, nonce, codeChallenge string) (string, error) {
	if p == nil || p.verifier == nil || state == "" || nonce == "" || codeChallenge == "" {
		return "", errors.New("invalid Google authorization request")
	}
	return p.oauth.AuthCodeURL(state,
		oauth2.SetAuthURLParam("nonce", nonce),
		oauth2.SetAuthURLParam("code_challenge", codeChallenge),
		oauth2.SetAuthURLParam("code_challenge_method", "S256")), nil
}

func (p *GoogleProvider) Exchange(ctx context.Context, code, codeVerifier string) (ProviderIdentity, error) {
	if p == nil || p.verifier == nil || code == "" || codeVerifier == "" {
		return ProviderIdentity{}, errors.New("invalid Google token exchange request")
	}
	token, err := p.oauth.Exchange(ctx, code, oauth2.VerifierOption(codeVerifier))
	if err != nil {
		return ProviderIdentity{}, fmt.Errorf("exchange Google authorization code: %w", err)
	}
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok || rawIDToken == "" {
		return ProviderIdentity{}, errors.New("Google token response has no ID token")
	}
	idToken, err := p.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return ProviderIdentity{}, fmt.Errorf("verify Google ID token: %w", err)
	}
	var claims struct {
		Issuer        string `json:"iss"`
		Subject       string `json:"sub"`
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
		Name          string `json:"name"`
		Nonce         string `json:"nonce"`
	}
	if err := idToken.Claims(&claims); err != nil {
		return ProviderIdentity{}, fmt.Errorf("read Google ID token claims: %w", err)
	}
	if !claims.EmailVerified || claims.Email == "" || claims.Issuer == "" || claims.Subject == "" || claims.Nonce == "" {
		return ProviderIdentity{}, errors.New("Google ID token has incomplete verified claims")
	}
	return ProviderIdentity{Issuer: claims.Issuer, Subject: claims.Subject, Email: claims.Email, DisplayName: claims.Name, Nonce: claims.Nonce}, nil
}
