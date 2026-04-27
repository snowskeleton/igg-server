package server

import (
	"log"
	"net/http"

	"github.com/snowskeleton/igg-server/internal/apns"
	"github.com/snowskeleton/igg-server/internal/config"
	"github.com/snowskeleton/igg-server/internal/email"
	"github.com/snowskeleton/igg-server/internal/handler"
	"github.com/snowskeleton/igg-server/internal/middleware"
	"github.com/snowskeleton/igg-server/internal/store/postgres"
)

func New(cfg *config.Config, store *postgres.Store) http.Handler {
	mailer := email.NewSender(cfg)

	// APNs client (nil if not configured)
	apnsClient, err := apns.NewClient(cfg)
	if err != nil {
		log.Printf("WARNING: APNs client init failed: %v (push notifications disabled)", err)
	}
	if apnsClient != nil {
		log.Printf("APNs push notifications enabled")
	}

	authH := handler.NewAuthHandler(store, cfg, mailer)
	syncH := handler.NewSyncHandler(store, apnsClient)
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

	// Middleware chain
	rl := middleware.NewRateLimiter(10, 30) // 10 req/s, burst 30
	var h http.Handler = mux
	h = middleware.RateLimit(rl)(h)
	h = middleware.Logging(h)

	return h
}
