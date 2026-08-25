package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
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
	cfg.CookieSecure = true
	return cfg, nil
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
	httpServer := &http.Server{Addr: ":" + port, Handler: server.Handler(), ReadHeaderTimeout: 5 * time.Second}
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
