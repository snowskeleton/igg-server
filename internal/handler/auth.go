package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/snowskeleton/igg-server/internal/auth"
	"github.com/snowskeleton/igg-server/internal/config"
	"github.com/snowskeleton/igg-server/internal/email"
	"github.com/snowskeleton/igg-server/internal/model"
	"github.com/snowskeleton/igg-server/internal/store/postgres"
)

type AuthHandler struct {
	store  *postgres.Store
	cfg    *config.Config
	mailer *email.Sender
}

func NewAuthHandler(store *postgres.Store, cfg *config.Config, mailer *email.Sender) *AuthHandler {
	return &AuthHandler{store: store, cfg: cfg, mailer: mailer}
}

func (h *AuthHandler) RequestMagicLink() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body model.AuthRequestBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		body.Email = strings.ToLower(strings.TrimSpace(body.Email))
		if body.Email == "" {
			writeError(w, http.StatusBadRequest, "email is required")
			return
		}

		// Get or create the user
		user, err := h.store.GetOrCreateUser(r.Context(), body.Email)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		// Generate magic token and poll token
		token, err := auth.GenerateMagicToken()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		pollToken, err := auth.GeneratePollToken()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		expiresAt := time.Now().Add(auth.MagicTokenExpiry)
		if err := h.store.CreateMagicTokenWithPoll(r.Context(), user.ID, token, pollToken, expiresAt); err != nil {
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		// Send email
		if err := h.mailer.SendMagicLink(body.Email, token); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to send email")
			return
		}

		writeJSON(w, http.StatusOK, model.AuthRequestResponse{
			Message:   "if an account exists, a login link has been sent",
			PollToken: pollToken,
		})
	}
}

func (h *AuthHandler) VerifyMagicLink() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := r.URL.Query().Get("token")
		if token == "" {
			writeError(w, http.StatusBadRequest, "token is required")
			return
		}

		mt, err := h.store.GetMagicToken(r.Context(), token)
		if err != nil || mt == nil {
			writeError(w, http.StatusUnauthorized, "invalid or expired token")
			return
		}
		if mt.Used || time.Now().After(mt.ExpiresAt) {
			writeError(w, http.StatusUnauthorized, "invalid or expired token")
			return
		}

		// Mark used
		if err := h.store.MarkMagicTokenUsed(r.Context(), mt.ID); err != nil {
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		// Get user
		user, err := h.store.GetUserByID(r.Context(), mt.UserID)
		if err != nil || user == nil {
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		// Generate tokens
		resp, err := h.generateTokenPair(r.Context(), user)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		// Build deep link URL
		deepLink := fmt.Sprintf("igg://auth/callback?access_token=%s&refresh_token=%s&expires_in=%d",
			url.QueryEscape(resp.AccessToken),
			url.QueryEscape(resp.RefreshToken),
			resp.ExpiresIn,
		)

		// Serve HTML page that auto-redirects to the app
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta http-equiv="refresh" content="0;url=%s">
<title>Login Successful</title>
<style>
  body { font-family: -apple-system, system-ui, sans-serif; text-align: center; padding: 60px 20px; background: #f5f5f7; }
  .card { max-width: 400px; margin: 0 auto; background: white; border-radius: 16px; padding: 40px; box-shadow: 0 2px 12px rgba(0,0,0,0.1); }
  h1 { font-size: 24px; margin-bottom: 12px; }
  p { color: #666; margin-bottom: 24px; }
  a { display: inline-block; background: #007AFF; color: white; text-decoration: none; padding: 12px 32px; border-radius: 10px; font-weight: 600; }
</style>
</head>
<body>
<div class="card">
  <h1>Login Successful!</h1>
  <p>You should be redirected to I Got Gas automatically.</p>
  <a href="%s">Open I Got Gas</a>
</div>
<script>window.location.href = %q;</script>
</body>
</html>`, deepLink, deepLink, deepLink)
	}
}

func (h *AuthHandler) Refresh() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body model.RefreshRequestBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if body.RefreshToken == "" {
			writeError(w, http.StatusBadRequest, "refresh_token is required")
			return
		}

		tokenHash := auth.HashRefreshToken(body.RefreshToken)
		rt, err := h.store.GetRefreshToken(r.Context(), tokenHash)
		if err != nil || rt == nil {
			writeError(w, http.StatusUnauthorized, "invalid refresh token")
			return
		}

		// Theft detection: if token was already revoked, revoke ALL tokens for this user
		if rt.Revoked {
			h.store.RevokeAllRefreshTokens(r.Context(), rt.UserID)
			writeError(w, http.StatusUnauthorized, "token reuse detected, all sessions revoked")
			return
		}

		// Revoke the used refresh token
		if err := h.store.RevokeRefreshToken(r.Context(), rt.ID); err != nil {
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		user, err := h.store.GetUserByID(r.Context(), rt.UserID)
		if err != nil || user == nil {
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		resp, err := h.generateTokenPair(r.Context(), user)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		writeJSON(w, http.StatusOK, resp)
	}
}

func (h *AuthHandler) Logout() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body model.RefreshRequestBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if body.RefreshToken != "" {
			tokenHash := auth.HashRefreshToken(body.RefreshToken)
			rt, _ := h.store.GetRefreshToken(r.Context(), tokenHash)
			if rt != nil {
				h.store.RevokeRefreshToken(r.Context(), rt.ID)
			}
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func (h *AuthHandler) PollAuth() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body model.PollRequestBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if body.PollToken == "" {
			writeError(w, http.StatusBadRequest, "poll_token is required")
			return
		}

		mt, err := h.store.GetMagicTokenByPollToken(r.Context(), body.PollToken)
		if err != nil || mt == nil {
			writeError(w, http.StatusNotFound, "invalid poll token")
			return
		}

		// Check expiry
		if time.Now().After(mt.ExpiresAt) {
			writeJSON(w, http.StatusGone, model.PollResponse{Status: "expired"})
			return
		}

		// Not yet verified
		if !mt.Used {
			writeJSON(w, http.StatusAccepted, model.PollResponse{Status: "pending"})
			return
		}

		// Magic token was used — generate a fresh token pair for the polling client
		user, err := h.store.GetUserByID(r.Context(), mt.UserID)
		if err != nil || user == nil {
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		resp, err := h.generateTokenPair(r.Context(), user)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		writeJSON(w, http.StatusOK, resp)
	}
}

func (h *AuthHandler) generateTokenPair(ctx context.Context, user *model.User) (*model.AuthTokenResponse, error) {
	accessToken, err := auth.GenerateAccessToken(h.cfg.JWTSecret, user.ID, user.Email)
	if err != nil {
		return nil, err
	}

	refreshToken, err := auth.GenerateRefreshToken()
	if err != nil {
		return nil, err
	}
	tokenHash := auth.HashRefreshToken(refreshToken)

	if err := h.store.CreateRefreshToken(ctx, user.ID, tokenHash); err != nil {
		return nil, err
	}

	return &model.AuthTokenResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int(auth.AccessTokenExpiry.Seconds()),
	}, nil
}
