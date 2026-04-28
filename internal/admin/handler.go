package admin

import (
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/snowskeleton/igg-server/internal/apns"
	"github.com/snowskeleton/igg-server/internal/auth"
	"github.com/snowskeleton/igg-server/internal/config"
	"github.com/snowskeleton/igg-server/internal/email"
	"github.com/snowskeleton/igg-server/internal/store/postgres"
)

const sessionDuration = 24 * time.Hour

// APNsReloadFunc is called after admin saves APNs config.
type APNsReloadFunc func(cfg *config.Config) error

type Handler struct {
	store      *postgres.Store
	cfg        *config.Config
	mailer     *email.Sender
	reloadAPNs APNsReloadFunc
}

func NewHandler(store *postgres.Store, cfg *config.Config, mailer *email.Sender, reload APNsReloadFunc) *Handler {
	return &Handler{store: store, cfg: cfg, mailer: mailer, reloadAPNs: reload}
}

// LoginPage serves the admin login form.
func (h *Handler) LoginPage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data := map[string]any{"Flash": nil, "FlashClass": ""}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		loginTmpl.Execute(w, data)
	}
}

// LoginSubmit handles the login form POST.
func (h *Handler) LoginSubmit() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		emailAddr := strings.ToLower(strings.TrimSpace(r.FormValue("email")))

		flash := "If that is the admin email, a login link has been sent."
		flashClass := "flash-success"

		if emailAddr != "" && h.cfg.AdminEmail != "" && emailAddr == h.cfg.AdminEmail {
			user, err := h.store.GetOrCreateUser(r.Context(), emailAddr)
			if err != nil {
				log.Printf("admin login: get user: %v", err)
				flash = "Internal error. Please try again."
				flashClass = "flash-error"
			} else {
				token, err := auth.GenerateMagicToken()
				if err != nil {
					log.Printf("admin login: generate token: %v", err)
					flash = "Internal error. Please try again."
					flashClass = "flash-error"
				} else {
					expiresAt := time.Now().Add(auth.MagicTokenExpiry)
					if err := h.store.CreateMagicToken(r.Context(), user.ID, token, expiresAt); err != nil {
						log.Printf("admin login: create token: %v", err)
						flash = "Internal error. Please try again."
						flashClass = "flash-error"
					} else {
						if err := h.mailer.SendAdminMagicLink(emailAddr, token); err != nil {
							log.Printf("admin login: send email: %v", err)
							flash = "Failed to send email. Please try again."
							flashClass = "flash-error"
						}
					}
				}
			}
		}

		data := map[string]any{"Flash": flash, "FlashClass": flashClass}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		loginTmpl.Execute(w, data)
	}
}

// Verify handles the admin magic link verification.
func (h *Handler) Verify() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := r.URL.Query().Get("token")
		if token == "" {
			RenderVerifyPage(w, VerifyPageData{
				Title: "Verification Failed",
				Error: "Missing token.",
			})
			return
		}

		mt, err := h.store.GetMagicToken(r.Context(), token)
		if err != nil || mt == nil {
			RenderVerifyPage(w, VerifyPageData{
				Title: "Verification Failed",
				Error: "Invalid or expired token.",
			})
			return
		}
		if mt.Used || time.Now().After(mt.ExpiresAt) {
			RenderVerifyPage(w, VerifyPageData{
				Title: "Verification Failed",
				Error: "This link has expired or already been used.",
			})
			return
		}

		// Mark used
		if err := h.store.MarkMagicTokenUsed(r.Context(), mt.ID); err != nil {
			RenderVerifyPage(w, VerifyPageData{
				Title: "Verification Failed",
				Error: "Internal error.",
			})
			return
		}

		// Verify the user is the admin
		user, err := h.store.GetUserByID(r.Context(), mt.UserID)
		if err != nil || user == nil || user.Email != h.cfg.AdminEmail {
			RenderVerifyPage(w, VerifyPageData{
				Title: "Verification Failed",
				Error: "Unauthorized.",
			})
			return
		}

		// Create admin session
		sessionToken, err := auth.GenerateSessionToken()
		if err != nil {
			RenderVerifyPage(w, VerifyPageData{
				Title: "Verification Failed",
				Error: "Internal error.",
			})
			return
		}

		tokenHash := auth.HashSessionToken(sessionToken)
		expiresAt := time.Now().Add(sessionDuration)
		if err := h.store.CreateAdminSession(r.Context(), user.ID, tokenHash, expiresAt); err != nil {
			RenderVerifyPage(w, VerifyPageData{
				Title: "Verification Failed",
				Error: "Internal error.",
			})
			return
		}

		// Clean up old sessions
		h.store.CleanExpiredAdminSessions(r.Context())

		http.SetCookie(w, &http.Cookie{
			Name:     "admin_session",
			Value:    sessionToken,
			Path:     "/admin",
			Expires:  expiresAt,
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
		})

		http.Redirect(w, r, "/admin/", http.StatusSeeOther)
	}
}

// Dashboard shows the overview stats page.
func (h *Handler) Dashboard() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		stats, err := h.store.GetDashboardStats(r.Context())
		if err != nil {
			log.Printf("admin dashboard: %v", err)
			http.Error(w, "Internal error", http.StatusInternalServerError)
			return
		}

		data := map[string]any{
			"Title": "Dashboard",
			"Nav":   "dashboard",
			"Stats": stats,
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		renderTemplate(w, dashboardTmpl, data)
	}
}

// Users shows the user list page.
func (h *Handler) Users() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		users, err := h.store.GetAllUsers(ctx)
		if err != nil {
			log.Printf("admin users: %v", err)
			http.Error(w, "Internal error", http.StatusInternalServerError)
			return
		}

		devices, err := h.store.GetDeviceTokensByUser(ctx)
		if err != nil {
			log.Printf("admin users devices: %v", err)
		}
		if devices == nil {
			devices = make(map[string][]postgres.AdminDevice)
		}

		notifications, err := h.store.GetRecentNotificationsByUser(ctx)
		if err != nil {
			log.Printf("admin users notifications: %v", err)
		}
		if notifications == nil {
			notifications = make(map[string][]postgres.AdminNotification)
		}

		data := map[string]any{
			"Title":         "Users",
			"Nav":           "users",
			"Users":         users,
			"Devices":       devices,
			"Notifications": notifications,
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		renderTemplate(w, usersTmpl, data)
	}
}

// Cars shows the car list page.
func (h *Handler) Cars() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cars, err := h.store.GetAllCars(r.Context())
		if err != nil {
			log.Printf("admin cars: %v", err)
			http.Error(w, "Internal error", http.StatusInternalServerError)
			return
		}

		data := map[string]any{
			"Title": "Cars",
			"Nav":   "cars",
			"Cars":  cars,
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		renderTemplate(w, carsTmpl, data)
	}
}

// ConfigPage shows the APNs config form.
func (h *Handler) ConfigPage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cfgMap, err := h.store.GetAllServerConfig(r.Context())
		if err != nil {
			log.Printf("admin config: %v", err)
		}
		if cfgMap == nil {
			cfgMap = make(map[string]string)
		}

		apnsCfg := struct {
			APNsKeyID      string
			APNsTeamID     string
			APNsKeyContent string
			APNsBundleID   string
			APNsProduction bool
		}{
			APNsKeyID:      configOrDefault(cfgMap, "apns_key_id", h.cfg.APNsKeyID),
			APNsTeamID:     configOrDefault(cfgMap, "apns_team_id", h.cfg.APNsTeamID),
			APNsKeyContent: configOrDefault(cfgMap, "apns_key_content", h.cfg.APNsKeyContent),
			APNsBundleID:   configOrDefault(cfgMap, "apns_bundle_id", h.cfg.APNsBundleID),
			APNsProduction: configOrDefault(cfgMap, "apns_production", boolToStr(h.cfg.APNsProduction)) == "true",
		}

		flash, flashClass := consumeFlash(r, w)

		data := map[string]any{
			"Title":      "Config",
			"Nav":        "config",
			"Config":     apnsCfg,
			"Flash":      flash,
			"FlashClass": flashClass,
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		renderTemplate(w, configTmpl, data)
	}
}

// ConfigSave handles the config form POST.
func (h *Handler) ConfigSave() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		keys := map[string]string{
			"apns_key_id":      r.FormValue("apns_key_id"),
			"apns_team_id":     r.FormValue("apns_team_id"),
			"apns_key_content": r.FormValue("apns_key_content"),
			"apns_bundle_id":   r.FormValue("apns_bundle_id"),
			"apns_production":  r.FormValue("apns_production"),
		}

		for k, v := range keys {
			if err := h.store.SetServerConfig(r.Context(), k, v); err != nil {
				log.Printf("admin config save %s: %v", k, err)
				setFlash(w, "Failed to save config.", "flash-error")
				http.Redirect(w, r, "/admin/config", http.StatusSeeOther)
				return
			}
		}

		// Build an effective config for APNs reload
		effectiveCfg := *h.cfg
		effectiveCfg.APNsKeyID = keys["apns_key_id"]
		effectiveCfg.APNsTeamID = keys["apns_team_id"]
		effectiveCfg.APNsKeyContent = keys["apns_key_content"]
		effectiveCfg.APNsBundleID = keys["apns_bundle_id"]
		effectiveCfg.APNsProduction = keys["apns_production"] == "true"

		if h.reloadAPNs != nil {
			if err := h.reloadAPNs(&effectiveCfg); err != nil {
				log.Printf("admin: APNs reload failed: %v", err)
				setFlash(w, "Config saved but APNs reload failed.", "flash-error")
				http.Redirect(w, r, "/admin/config", http.StatusSeeOther)
				return
			}
			log.Printf("admin: APNs client reloaded successfully")
		}

		setFlash(w, "Config saved and APNs reloaded.", "flash-success")
		http.Redirect(w, r, "/admin/config", http.StatusSeeOther)
	}
}

// Logout clears the admin session.
func (h *Handler) Logout() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("admin_session")
		if err == nil {
			tokenHash := auth.HashSessionToken(cookie.Value)
			h.store.DeleteAdminSession(r.Context(), tokenHash)
		}

		http.SetCookie(w, &http.Cookie{
			Name:     "admin_session",
			Value:    "",
			Path:     "/admin",
			MaxAge:   -1,
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
		})

		http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
	}
}

// BuildAPNsClient creates a new APNs client from config. Exported for use in server.go.
func BuildAPNsClient(cfg *config.Config) (*apns.Client, error) {
	return apns.NewClient(cfg)
}

func setFlash(w http.ResponseWriter, msg, class string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "admin_flash",
		Value:    class + "|" + msg,
		Path:     "/admin",
		MaxAge:   10,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func consumeFlash(r *http.Request, w http.ResponseWriter) (string, string) {
	cookie, err := r.Cookie("admin_flash")
	if err != nil || cookie.Value == "" {
		return "", ""
	}
	// Clear the cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "admin_flash",
		Value:    "",
		Path:     "/admin",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	parts := strings.SplitN(cookie.Value, "|", 2)
	if len(parts) != 2 {
		return "", ""
	}
	return parts[1], parts[0]
}

func configOrDefault(m map[string]string, key, fallback string) string {
	if v, ok := m[key]; ok && v != "" {
		return v
	}
	return fallback
}

func boolToStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
