package handler

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/snowskeleton/igg-server/internal/middleware"
	"github.com/snowskeleton/igg-server/internal/model"
	"github.com/snowskeleton/igg-server/internal/store/postgres"
)

type SyncHandler struct {
	store *postgres.Store
}

func NewSyncHandler(store *postgres.Store) *SyncHandler {
	return &SyncHandler{store: store}
}

func (h *SyncHandler) Sync() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r.Context())
		ctx := r.Context()

		var req model.SyncRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if req.DeviceID == "" {
			writeError(w, http.StatusBadRequest, "device_id is required")
			return
		}

		syncTime := time.Now().UTC()

		// Determine the cursor: use client-provided last_synced_at, or zero time for first sync
		var since time.Time
		if req.LastSyncedAt != nil {
			since = *req.LastSyncedAt
		}

		// Get the sets of car IDs the user can access
		ownedIDs, err := h.store.GetOwnedCarIDs(ctx, userID)
		if err != nil {
			log.Printf("sync: get owned car IDs: %v", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		accessibleIDs, err := h.store.GetAccessibleCarIDs(ctx, userID)
		if err != nil {
			log.Printf("sync: get accessible car IDs: %v", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		// ── Push: apply client changes ──
		tx, err := h.store.BeginTx(ctx)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		defer tx.Rollback()

		// Cars: only owners can upsert
		for i := range req.Changes.Cars {
			c := &req.Changes.Cars[i]
			c.OwnerID = userID // force owner
			if err := h.store.UpsertCar(ctx, tx, c); err != nil {
				log.Printf("sync: upsert car %s: %v", c.ID, err)
			}
			// Track newly-created cars as owned
			ownedIDs[c.ID] = true
			accessibleIDs[c.ID] = true
		}

		// Services: owners and shared users can write
		for i := range req.Changes.Services {
			svc := &req.Changes.Services[i]
			if !accessibleIDs[svc.CarID] {
				continue // skip services for cars user can't access
			}
			// Non-owners can't delete
			if svc.Deleted && !ownedIDs[svc.CarID] {
				svc.Deleted = false
			}
			if err := h.store.UpsertService(ctx, tx, svc); err != nil {
				log.Printf("sync: upsert service %s: %v", svc.ID, err)
			}
		}

		// Scheduled services: same rules as services
		for i := range req.Changes.ScheduledServices {
			ss := &req.Changes.ScheduledServices[i]
			if !accessibleIDs[ss.CarID] {
				continue
			}
			if ss.Deleted && !ownedIDs[ss.CarID] {
				ss.Deleted = false
			}
			if err := h.store.UpsertScheduledService(ctx, tx, ss); err != nil {
				log.Printf("sync: upsert scheduled service %s: %v", ss.ID, err)
			}
		}

		// Car settings: per-user, only for accessible cars
		for i := range req.Changes.CarSettings {
			cs := &req.Changes.CarSettings[i]
			cs.UserID = userID // force user
			if !accessibleIDs[cs.CarID] {
				continue
			}
			if err := h.store.UpsertCarSettings(ctx, tx, cs); err != nil {
				log.Printf("sync: upsert car settings %s: %v", cs.CarID, err)
			}
		}

		if err := tx.Commit(); err != nil {
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		// ── Pull: gather all changes since cursor ──
		allCarIDs := make([]string, 0, len(accessibleIDs))
		for id := range accessibleIDs {
			allCarIDs = append(allCarIDs, id)
		}

		changedCars, err := h.store.GetCarsForUser(ctx, userID, since)
		if err != nil {
			log.Printf("sync: get cars: %v", err)
		}
		changedServices, err := h.store.GetServicesForCars(ctx, allCarIDs, since)
		if err != nil {
			log.Printf("sync: get services: %v", err)
		}
		changedScheduled, err := h.store.GetScheduledServicesForCars(ctx, allCarIDs, since)
		if err != nil {
			log.Printf("sync: get scheduled services: %v", err)
		}
		changedSettings, err := h.store.GetCarSettingsForUser(ctx, userID, allCarIDs, since)
		if err != nil {
			log.Printf("sync: get car settings: %v", err)
		}

		// ── Shares info ──
		ownedShares, _ := h.store.GetSharesOwnedByUser(ctx, userID)
		receivedShares, _ := h.store.GetSharesReceivedByUser(ctx, userID)

		ownedMap := make(map[string][]model.SharePerson)
		for _, s := range ownedShares {
			ownedMap[s.CarID] = append(ownedMap[s.CarID], model.SharePerson{
				Email:  s.InvitedEmail,
				Status: s.Status,
			})
		}
		var ownedShareList []model.OwnedShare
		for carID, people := range ownedMap {
			ownedShareList = append(ownedShareList, model.OwnedShare{CarID: carID, SharedWith: people})
		}

		var receivedShareList []model.ReceivedShare
		for _, s := range receivedShares {
			owner, _ := h.store.GetUserByID(ctx, s.SharedByID)
			email := ""
			if owner != nil {
				email = owner.Email
			}
			receivedShareList = append(receivedShareList, model.ReceivedShare{
				CarID:      s.CarID,
				OwnerEmail: email,
				Status:     s.Status,
			})
		}

		// Update sync cursor
		h.store.UpsertSyncCursor(ctx, userID, req.DeviceID, syncTime)

		// Ensure non-nil slices in response
		if changedCars == nil {
			changedCars = []model.Car{}
		}
		if changedServices == nil {
			changedServices = []model.Service{}
		}
		if changedScheduled == nil {
			changedScheduled = []model.ScheduledService{}
		}
		if changedSettings == nil {
			changedSettings = []model.CarSettings{}
		}
		if ownedShareList == nil {
			ownedShareList = []model.OwnedShare{}
		}
		if receivedShareList == nil {
			receivedShareList = []model.ReceivedShare{}
		}

		resp := model.SyncResponse{
			SyncedAt: syncTime,
			Changes: model.SyncChanges{
				Cars:              changedCars,
				Services:          changedServices,
				ScheduledServices: changedScheduled,
				CarSettings:       changedSettings,
			},
			Shares: model.SyncShares{
				Owned:    ownedShareList,
				Received: receivedShareList,
			},
		}

		writeJSON(w, http.StatusOK, resp)
	}
}
