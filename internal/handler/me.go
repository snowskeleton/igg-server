package handler

import (
	"net/http"

	"github.com/snowskeleton/igg-server/internal/middleware"
	"github.com/snowskeleton/igg-server/internal/model"
	"github.com/snowskeleton/igg-server/internal/store/postgres"
)

type MeHandler struct {
	store *postgres.Store
}

func NewMeHandler(store *postgres.Store) *MeHandler {
	return &MeHandler{store: store}
}

func (h *MeHandler) GetMe() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r.Context())
		user, err := h.store.GetUserByID(r.Context(), userID)
		if err != nil || user == nil {
			writeError(w, http.StatusNotFound, "user not found")
			return
		}
		writeJSON(w, http.StatusOK, model.MeResponse{ID: user.ID, Email: user.Email})
	}
}

func (h *MeHandler) DeleteMe() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r.Context())
		if err := h.store.DeleteUser(r.Context(), userID); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to delete account")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
