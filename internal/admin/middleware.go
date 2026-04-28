package admin

import (
	"context"
	"net/http"
	"time"

	"github.com/snowskeleton/igg-server/internal/auth"
	"github.com/snowskeleton/igg-server/internal/store/postgres"
)

type contextKey string

const adminUserIDKey contextKey = "adminUserID"

// RequireSession is middleware that validates the admin session cookie.
func RequireSession(store *postgres.Store) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie("admin_session")
			if err != nil {
				http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
				return
			}

			tokenHash := auth.HashSessionToken(cookie.Value)
			sess, err := store.GetAdminSession(r.Context(), tokenHash)
			if err != nil || sess == nil || time.Now().After(sess.ExpiresAt) {
				// Clear invalid cookie
				http.SetCookie(w, &http.Cookie{
					Name:     "admin_session",
					Value:    "",
					Path:     "/admin",
					MaxAge:   -1,
					HttpOnly: true,
					SameSite: http.SameSiteLaxMode,
				})
				http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
				return
			}

			ctx := context.WithValue(r.Context(), adminUserIDKey, sess.UserID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
