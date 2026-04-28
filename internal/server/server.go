package server

import (
	"context"
	"log"
	"net/http"
	"sync/atomic"

	"github.com/snowskeleton/igg-server/internal/admin"
	"github.com/snowskeleton/igg-server/internal/apns"
	"github.com/snowskeleton/igg-server/internal/config"
	"github.com/snowskeleton/igg-server/internal/email"
	"github.com/snowskeleton/igg-server/internal/handler"
	"github.com/snowskeleton/igg-server/internal/middleware"
	"github.com/snowskeleton/igg-server/internal/store/postgres"
)

func New(cfg *config.Config, store *postgres.Store) http.Handler {
	mailer := email.NewSender(cfg)

	// APNs client with atomic pointer for hot-reload
	var apnsPtr atomic.Pointer[apns.Client]

	// Load APNs config: DB values override env vars
	effectiveCfg := buildEffectiveConfig(cfg, store)
	apnsClient, err := apns.NewClient(effectiveCfg)
	if err != nil {
		log.Printf("WARNING: APNs client init failed: %v (push notifications disabled)", err)
	}
	if apnsClient != nil {
		apnsPtr.Store(apnsClient)
		log.Printf("APNs push notifications enabled")
	}

	authH := handler.NewAuthHandler(store, cfg, mailer)
	syncH := handler.NewSyncHandler(store, &apnsPtr)
	sharingH := handler.NewSharingHandler(store, cfg, mailer)
	meH := handler.NewMeHandler(store)
	deviceH := handler.NewDeviceHandler(store)

	mux := http.NewServeMux()

	// Public routes (no auth)
	mux.HandleFunc("GET /v1/health", handler.Health())
	mux.HandleFunc("POST /v1/auth/request", authH.RequestMagicLink())
	mux.HandleFunc("GET /v1/auth/verify", authH.VerifyMagicLink())
	mux.HandleFunc("POST /v1/auth/refresh", authH.Refresh())
	mux.HandleFunc("POST /v1/auth/logout", authH.Logout())
	mux.HandleFunc("POST /v1/auth/poll", authH.PollAuth())

	// Authenticated routes
	authed := http.NewServeMux()
	authed.HandleFunc("POST /v1/sync", syncH.Sync())
	authed.HandleFunc("POST /v1/cars/{carId}/shares", sharingH.CreateShare())
	authed.HandleFunc("GET /v1/cars/{carId}/shares", sharingH.ListShares())
	authed.HandleFunc("DELETE /v1/cars/{carId}/shares/{shareId}", sharingH.RevokeShare())
	authed.HandleFunc("GET /v1/shares/pending", sharingH.PendingShares())
	authed.HandleFunc("GET /v1/shares/received", sharingH.ReceivedShares())
	authed.HandleFunc("POST /v1/shares/{shareId}/accept", sharingH.AcceptShare())
	authed.HandleFunc("POST /v1/shares/{shareId}/decline", sharingH.DeclineShare())
	authed.HandleFunc("POST /v1/shares/{shareId}/leave", sharingH.LeaveShare())
	authed.HandleFunc("GET /v1/me", meH.GetMe())
	authed.HandleFunc("DELETE /v1/me", meH.DeleteMe())
	authed.HandleFunc("PUT /v1/devices", deviceH.RegisterDevice())
	authed.HandleFunc("DELETE /v1/devices", deviceH.UnregisterDevice())

	mux.Handle("/v1/sync", middleware.Auth(cfg.JWTSecret)(authed))
	mux.Handle("/v1/cars/", middleware.Auth(cfg.JWTSecret)(authed))
	mux.Handle("/v1/shares/", middleware.Auth(cfg.JWTSecret)(authed))
	mux.Handle("/v1/me", middleware.Auth(cfg.JWTSecret)(authed))
	mux.Handle("/v1/devices", middleware.Auth(cfg.JWTSecret)(authed))

	// Admin routes
	reloadAPNs := func(newCfg *config.Config) error {
		client, err := admin.BuildAPNsClient(newCfg)
		if err != nil {
			return err
		}
		if client != nil {
			apnsPtr.Store(client)
		}
		return nil
	}

	adminH := admin.NewHandler(store, cfg, mailer, reloadAPNs)

	// Public admin routes
	mux.HandleFunc("GET /admin/login", adminH.LoginPage())
	mux.HandleFunc("POST /admin/login", adminH.LoginSubmit())
	mux.HandleFunc("GET /admin/verify", adminH.Verify())

	// Protected admin routes
	adminMux := http.NewServeMux()
	adminMux.HandleFunc("GET /admin/", adminH.Dashboard())
	adminMux.HandleFunc("GET /admin/users", adminH.Users())
	adminMux.HandleFunc("GET /admin/cars", adminH.Cars())
	adminMux.HandleFunc("GET /admin/config", adminH.ConfigPage())
	adminMux.HandleFunc("POST /admin/config", adminH.ConfigSave())
	adminMux.HandleFunc("POST /admin/logout", adminH.Logout())

	mux.Handle("/admin/", admin.RequireSession(store)(adminMux))

	// Middleware chain
	rl := middleware.NewRateLimiter(10, 30) // 10 req/s, burst 30
	var h http.Handler = mux
	h = middleware.RateLimit(rl)(h)
	h = middleware.Logging(h)

	return h
}

// buildEffectiveConfig merges DB server_config values over env-based config.
func buildEffectiveConfig(cfg *config.Config, store *postgres.Store) *config.Config {
	eCfg := *cfg
	ctx := context.Background()
	dbCfg, err := store.GetAllServerConfig(ctx)
	if err != nil {
		log.Printf("WARNING: failed to load server_config from DB: %v", err)
		return &eCfg
	}
	if v, ok := dbCfg["apns_key_id"]; ok && v != "" {
		eCfg.APNsKeyID = v
	}
	if v, ok := dbCfg["apns_team_id"]; ok && v != "" {
		eCfg.APNsTeamID = v
	}
	if v, ok := dbCfg["apns_key_content"]; ok && v != "" {
		eCfg.APNsKeyContent = v
	}
	if v, ok := dbCfg["apns_bundle_id"]; ok && v != "" {
		eCfg.APNsBundleID = v
	}
	if v, ok := dbCfg["apns_production"]; ok && v != "" {
		eCfg.APNsProduction = v == "true"
	}
	return &eCfg
}
