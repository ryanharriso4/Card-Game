package main

import (
	"context"
	"database/sql"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"time"

	"cardgame.ryanharris.net/internal/data"
	"github.com/alexedwards/scs/postgresstore"
	"github.com/alexedwards/scs/v2"
	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gorilla/websocket"
	_ "github.com/lib/pq"
	"golang.org/x/oauth2"
)

const version = "1.0.0"

type config struct {
	port int
	env  string
	db   struct {
		dsn          string
		maxOpenConns int
		maxIdleConns int
		maxIdleTime  time.Duration
	}

	auth struct {
		googleClientID     string
		googleClientSecret string
		googleRedirectURL  string
	}

	limiter struct {
		rps     float64
		burst   int
		enabled bool
	}

	upgrader websocket.Upgrader
}

type application struct {
	config         config
	logger         *slog.Logger
	db             *sql.DB
	sessionManager *scs.SessionManager
	models         data.Models

	auth struct {
		oauthConfig *oauth2.Config
		verifier    *oidc.IDTokenVerifier
		provider    *oidc.Provider
	}

	hub *Hub

	wsSemaphore chan struct{}
}

func main() {
	var cfg config

	flag.IntVar(&cfg.port, "port", 2000, "API server port")
	flag.StringVar(&cfg.env, "env", "development", "Environment (development|staging|production)")

	flag.StringVar(&cfg.db.dsn, "db-dsn", os.Getenv("CARDGAME_DB_DSN"), "PostgreSQL DSN")
	flag.IntVar(&cfg.db.maxOpenConns, "db-max-open-conns", 25, "PostgreSQL max open connections")
	flag.IntVar(&cfg.db.maxIdleConns, "db-max-idle-conns", 25, "PostgreSQL max idle connections")
	flag.DurationVar(&cfg.db.maxIdleTime, "db-max-idle-time", 15*time.Minute, "PostgreSQL max connection idle time")

	flag.StringVar(&cfg.auth.googleClientID, "oidc-id", os.Getenv("GOOGLE_OAUTH2_CLIENT_ID"), "OIDC client id")
	flag.StringVar(&cfg.auth.googleClientSecret, "oidc-secret", os.Getenv("GOOGLE_OAUTH2_CLIENT_SECRET"), "OIDC client secret")
	flag.StringVar(&cfg.auth.googleRedirectURL, "oidc-redirect", os.Getenv("GOOGLE_OAUTH2_REDIRECT"), "OIDC redirect")

	flag.Float64Var(&cfg.limiter.rps, "limiter-rps", 2, "Rate limiter maximum requests per second")
	flag.IntVar(&cfg.limiter.burst, "limiter-burst", 4, "Rate limiter maximum burst")
	flag.BoolVar(&cfg.limiter.enabled, "limiter-enabled", true, "Enable rate limiter")

	flag.IntVar(&cfg.upgrader.ReadBufferSize, "buffer-read", 1024, "Read buffer size")
	flag.IntVar(&cfg.upgrader.WriteBufferSize, "buffer-write", 1024, "Write buffer size")

	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	db, err := openDB(cfg)
	if err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}

	defer db.Close()

	logger.Info("database connection pool established")

	ctx := context.Background()
	provider, err := oidc.NewProvider(ctx, "https://accounts.google.com")

	if err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}

	verifier := provider.Verifier(&oidc.Config{ClientID: cfg.auth.googleClientID})

	oauthConfig := &oauth2.Config{
		ClientID:     cfg.auth.googleClientID,
		ClientSecret: cfg.auth.googleClientSecret,
		Endpoint:     provider.Endpoint(),
		RedirectURL:  cfg.auth.googleRedirectURL,
		Scopes: []string{
			oidc.ScopeOpenID,
			"profile",
			"email",
		},
	}

	gameHub := newHub()

	app := &application{
		config:         cfg,
		logger:         logger,
		db:             db,
		sessionManager: sessionManagerSetUp(cfg, db),
		models:         data.NewModels(db),
		hub:            gameHub,
		wsSemaphore:    make(chan struct{}, 1000),
	}

	app.auth.provider = provider
	app.auth.verifier = verifier
	app.auth.oauthConfig = oauthConfig

	go app.run()

	err = app.serve()
	if err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}

}

func openDB(cfg config) (*sql.DB, error) {
	db, err := sql.Open("postgres", cfg.db.dsn)
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(cfg.db.maxOpenConns)
	db.SetMaxIdleConns(cfg.db.maxIdleConns)
	db.SetConnMaxIdleTime(cfg.db.maxIdleTime)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = db.PingContext(ctx)
	if err != nil {
		db.Close()
		return nil, err
	}

	return db, nil
}

func sessionManagerSetUp(cfg config, db *sql.DB) *scs.SessionManager {

	if db == nil {
		panic("sessionManagerSetUp: database connection pool is nil")
	}

	sessionManager := scs.New()
	sessionManager.Store = postgresstore.New(db)

	sessionManager.Lifetime = 24 * time.Hour
	sessionManager.IdleTimeout = 30 * time.Minute
	sessionManager.Cookie.Name = "session_id"
	sessionManager.Cookie.HttpOnly = true
	sessionManager.Cookie.Secure = (cfg.env == "production")
	sessionManager.Cookie.SameSite = http.SameSiteLaxMode

	return sessionManager

}
