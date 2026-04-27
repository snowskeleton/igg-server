package handler

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/snowskeleton/igg-server/internal/middleware"
	"github.com/snowskeleton/igg-server/internal/model"
	"github.com/snowskeleton/igg-server/internal/store/postgres"
)

type DeviceHandler struct {
	store *postgres.Store
}

func NewDeviceHandler(store *postgres.Store) *DeviceHandler {
	return &DeviceHandler{store: store}
}

func (h *DeviceHandler) RegisterDevice() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r.Context())

		var req model.RegisterDeviceRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if req.DeviceID == "" || req.Token == "" {
			writeError(w, http.StatusBadRequest, "device_id and token are required")
			return
		}
		if req.Platform == "" {
			req.Platform = "ios"
		}
		if req.NotifyMode == "" {
			req.NotifyMode = "silent"
		}
		if req.NotifyMode != "silent" && req.NotifyMode != "visible" {
			writeError(w, http.StatusBadRequest, "notify_mode must be 'silent' or 'visible'")
			return
		}

		dt := &model.DeviceToken{
			UserID:     userID,
			DeviceID:   req.DeviceID,
			Token:      req.Token,
			Platform:   req.Platform,
			NotifyMode: req.NotifyMode,
		}

		if err := h.store.UpsertDeviceToken(r.Context(), dt); err != nil {
			log.Printf("devices: upsert token: %v", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

func (h *DeviceHandler) UnregisterDevice() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r.Context())

		var req model.UnregisterDeviceRequest
		// Body is optional — if no body or empty device_id, delete all tokens for user
		json.NewDecoder(r.Body).Decode(&req)

		var err error
		if req.DeviceID != "" {
			err = h.store.DeleteDeviceToken(r.Context(), userID, req.DeviceID)
		} else {
			err = h.store.DeleteAllDeviceTokensForUser(r.Context(), userID)
		}

		if err != nil {
			log.Printf("devices: delete token: %v", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
