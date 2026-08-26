package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	_ "modernc.org/sqlite"
	"mud/internal/authapi"
)

type runtimeConfig struct {
	Port         string
	DatabasePath string
	FrontendURL  string
	CookieSecure bool
	authapi.GoogleConfig
}

const frontendDistPath = "/web/dist"

func loadConfig(getenv func(string) string) (runtimeConfig, error) {
	cfg := runtimeConfig{Port: getenv("PORT"), DatabasePath: getenv("DATABASE_PATH"), FrontendURL: getenv("FRONTEND_URL"), GoogleConfig: authapi.GoogleConfig{
		ClientID: getenv("GOOGLE_CLIENT_ID"), ClientSecret: getenv("GOOGLE_CLIENT_SECRET"), RedirectURL: getenv("GOOGLE_REDIRECT_URL"),
	}}
	if cfg.Port == "" {
		cfg.Port = "8080"
	}
	if cfg.DatabasePath == "" {
		cfg.DatabasePath = "/data/mud.db"
	}
	if cfg.FrontendURL == "" || cfg.ClientID == "" || cfg.ClientSecret == "" || cfg.RedirectURL == "" {
		return runtimeConfig{}, errors.New("FRONTEND_URL, GOOGLE_CLIENT_ID, GOOGLE_CLIENT_SECRET, and GOOGLE_REDIRECT_URL are required")
	}
	frontendOrigin, err := originOnlyURL(cfg.FrontendURL)
	if err != nil {
		return runtimeConfig{}, fmt.Errorf("invalid FRONTEND_URL: %w", err)
	}
	redirectURL, err := callbackURL(cfg.RedirectURL, frontendOrigin)
	if err != nil {
		return runtimeConfig{}, fmt.Errorf("invalid GOOGLE_REDIRECT_URL: %w", err)
	}
	cfg.FrontendURL = frontendOrigin
	cfg.RedirectURL = redirectURL
	cfg.CookieSecure = true
	return cfg, nil
}

func originOnlyURL(raw string) (string, error) {
	if strings.TrimSpace(raw) != raw {
		return "", errors.New("must not contain surrounding whitespace")
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.Opaque != "" || parsed.ForceQuery || parsed.RawQuery != "" || parsed.Fragment != "" || (parsed.Path != "" && parsed.Path != "/") || parsed.RawPath != "" {
		return "", errors.New("must be an HTTPS origin without a path, query, or fragment")
	}
	if parsed.Hostname() == "" {
		return "", errors.New("must include a host")
	}
	parsed.Scheme = "https"
	host := strings.ToLower(parsed.Hostname())
	if port := parsed.Port(); port != "" && port != "443" {
		host = host + ":" + port
	}
	parsed.Host = host
	parsed.Path = ""
	return parsed.String(), nil
}

func callbackURL(raw, frontendOrigin string) (string, error) {
	if strings.TrimSpace(raw) != raw {
		return "", errors.New("must not contain surrounding whitespace")
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.Opaque != "" || parsed.RawQuery != "" || parsed.ForceQuery || parsed.Fragment != "" || parsed.Path != "/auth/google/callback" || parsed.RawPath != "" {
		return "", errors.New("must be the HTTPS origin plus /auth/google/callback without a query or fragment")
	}
	callbackOrigin, err := originOnlyURL(parsed.Scheme + "://" + parsed.Host)
	if err != nil {
		return "", err
	}
	if callbackOrigin != frontendOrigin {
		return "", errors.New("must use the same origin as FRONTEND_URL")
	}
	return callbackOrigin + parsed.Path, nil
}

func run(ctx context.Context, getenv func(string) string) error {
	cfg, err := loadConfig(getenv)
	if err != nil {
		return err
	}
	db, err := sql.Open("sqlite", cfg.DatabasePath)
	if err != nil {
		return fmt.Errorf("open SQLite database: %w", err)
	}
	defer db.Close()
	store, err := authapi.NewStore(db)
	if err != nil {
		return err
	}
	provider, err := authapi.NewGoogleProvider(ctx, cfg.GoogleConfig)
	if err != nil {
		return err
	}
	server, err := authapi.NewServer(store, provider, authapi.Config{FrontendURL: cfg.FrontendURL, CookieSecure: cfg.CookieSecure})
	if err != nil {
		return err
	}
	port := strings.TrimPrefix(cfg.Port, ":")
	if port == "" {
		return errors.New("PORT must not be empty")
	}
	frontend, err := newStaticHandler(frontendDistPath)
	if err != nil {
		return err
	}
	httpServer := &http.Server{Addr: ":" + port, Handler: server.Handler(frontend), ReadHeaderTimeout: 5 * time.Second}
	go func() { <-ctx.Done(); _ = httpServer.Shutdown(context.Background()) }()
	log.Printf("listening on %s", httpServer.Addr)
	if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func main() {
	if err := run(context.Background(), os.Getenv); err != nil {
		log.Fatal(err)
	}
}
