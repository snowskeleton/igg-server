package handler

import (
	"context"
	"encoding/json"
	"net/http"
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

		// Generate and store magic token
		token, err := auth.GenerateMagicToken()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		expiresAt := time.Now().Add(auth.MagicTokenExpiry)
		if err := h.store.CreateMagicToken(r.Context(), user.ID, token, expiresAt); err != nil {
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		// Send email (async-ish, but we wait for it here for simplicity)
		if err := h.mailer.SendMagicLink(body.Email, token); err != nil {
			// Log but don't expose to user
			writeError(w, http.StatusInternalServerError, "failed to send email")
			return
		}

		// Always return 200 to avoid email enumeration
		writeJSON(w, http.StatusOK, map[string]string{"message": "if an account exists, a login link has been sent"})
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
		writeJSON(w, http.StatusOK, resp)
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
