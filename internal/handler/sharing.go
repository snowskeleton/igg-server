package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/snowskeleton/igg-server/internal/auth"
	"github.com/snowskeleton/igg-server/internal/config"
	"github.com/snowskeleton/igg-server/internal/email"
	"github.com/snowskeleton/igg-server/internal/middleware"
	"github.com/snowskeleton/igg-server/internal/model"
	"github.com/snowskeleton/igg-server/internal/store/postgres"
)

type SharingHandler struct {
	store  *postgres.Store
	cfg    *config.Config
	mailer *email.Sender
}

func NewSharingHandler(store *postgres.Store, cfg *config.Config, mailer *email.Sender) *SharingHandler {
	return &SharingHandler{store: store, cfg: cfg, mailer: mailer}
}

// CreateShare — POST /v1/cars/{carId}/shares
func (h *SharingHandler) CreateShare() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r.Context())
		carID := r.PathValue("carId")

		car, err := h.store.GetCarByID(r.Context(), carID)
		if err != nil || car == nil {
			writeError(w, http.StatusNotFound, "car not found")
			return
		}
		if car.OwnerID != userID {
			writeError(w, http.StatusForbidden, "only the owner can share this car")
			return
		}

		var body model.CreateShareRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		body.Email = strings.ToLower(strings.TrimSpace(body.Email))
		if body.Email == "" {
			writeError(w, http.StatusBadRequest, "email is required")
			return
		}

		token, _ := auth.GenerateMagicToken()

		// Check if the invited user already exists
		invitedUser, _ := h.store.GetUserByEmail(r.Context(), body.Email)
		var sharedWithID *string
		if invitedUser != nil {
			sharedWithID = &invitedUser.ID
		}

		share := &model.CarShare{
			CarID:        carID,
			SharedByID:   userID,
			SharedWithID: sharedWithID,
			InvitedEmail: body.Email,
			Status:       "pending",
			Token:        token,
		}

		if err := h.store.CreateShare(r.Context(), share); err != nil {
			if strings.Contains(err.Error(), "duplicate key") {
				writeError(w, http.StatusConflict, "already shared with this email")
				return
			}
			writeError(w, http.StatusInternalServerError, "failed to create share")
			return
		}

		carName := car.Name
		if carName == "" {
			carName = car.Make + " " + car.Model
		}
		ownerEmail := middleware.GetUserEmail(r.Context())
		h.mailer.SendShareInvitation(body.Email, ownerEmail, carName, token)

		writeJSON(w, http.StatusCreated, model.ShareResponse{
			ID:           share.ID,
			CarID:        share.CarID,
			InvitedEmail: share.InvitedEmail,
			Status:       share.Status,
			CreatedAt:    share.CreatedAt,
		})
	}
}

// ListShares — GET /v1/cars/{carId}/shares
func (h *SharingHandler) ListShares() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r.Context())
		carID := r.PathValue("carId")

		car, err := h.store.GetCarByID(r.Context(), carID)
		if err != nil || car == nil {
			writeError(w, http.StatusNotFound, "car not found")
			return
		}
		if car.OwnerID != userID {
			writeError(w, http.StatusForbidden, "only the owner can view shares")
			return
		}

		shares, err := h.store.GetSharesForCar(r.Context(), carID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to list shares")
			return
		}

		var out []model.ShareResponse
		for _, s := range shares {
			out = append(out, model.ShareResponse{
				ID:           s.ID,
				CarID:        s.CarID,
				InvitedEmail: s.InvitedEmail,
				Status:       s.Status,
				CreatedAt:    s.CreatedAt,
			})
		}
		if out == nil {
			out = []model.ShareResponse{}
		}
		writeJSON(w, http.StatusOK, out)
	}
}

// RevokeShare — DELETE /v1/cars/{carId}/shares/{shareId}
func (h *SharingHandler) RevokeShare() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r.Context())
		carID := r.PathValue("carId")

		car, err := h.store.GetCarByID(r.Context(), carID)
		if err != nil || car == nil {
			writeError(w, http.StatusNotFound, "car not found")
			return
		}
		if car.OwnerID != userID {
			writeError(w, http.StatusForbidden, "only the owner can revoke shares")
			return
		}

		shareID := r.PathValue("shareId")
		if err := h.store.UpdateShareStatus(r.Context(), shareID, "revoked", nil); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to revoke share")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// PendingShares — GET /v1/shares/pending
func (h *SharingHandler) PendingShares() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userEmail := middleware.GetUserEmail(r.Context())
		shares, err := h.store.GetPendingSharesForEmail(r.Context(), userEmail)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to get pending shares")
			return
		}

		var out []model.ShareResponse
		for _, s := range shares {
			out = append(out, model.ShareResponse{
				ID:           s.ID,
				CarID:        s.CarID,
				InvitedEmail: s.InvitedEmail,
				Status:       s.Status,
				CreatedAt:    s.CreatedAt,
			})
		}
		if out == nil {
			out = []model.ShareResponse{}
		}
		writeJSON(w, http.StatusOK, out)
	}
}

// AcceptShare — POST /v1/shares/{shareId}/accept
func (h *SharingHandler) AcceptShare() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r.Context())
		userEmail := middleware.GetUserEmail(r.Context())
		shareID := r.PathValue("shareId")

		share, err := h.store.GetShareByID(r.Context(), shareID)
		if err != nil || share == nil {
			writeError(w, http.StatusNotFound, "share not found")
			return
		}
		if share.InvitedEmail != userEmail {
			writeError(w, http.StatusForbidden, "this share is not for you")
			return
		}
		if share.Status != "pending" {
			writeError(w, http.StatusBadRequest, "share is not pending")
			return
		}

		if err := h.store.UpdateShareStatus(r.Context(), shareID, "accepted", &userID); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to accept share")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "accepted"})
	}
}

// DeclineShare — POST /v1/shares/{shareId}/decline
func (h *SharingHandler) DeclineShare() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userEmail := middleware.GetUserEmail(r.Context())
		shareID := r.PathValue("shareId")

		share, err := h.store.GetShareByID(r.Context(), shareID)
		if err != nil || share == nil {
			writeError(w, http.StatusNotFound, "share not found")
			return
		}
		if share.InvitedEmail != userEmail {
			writeError(w, http.StatusForbidden, "this share is not for you")
			return
		}

		if err := h.store.UpdateShareStatus(r.Context(), shareID, "declined", nil); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to decline share")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "declined"})
	}
}
