package handler

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/snowskeleton/igg-server/internal/apns"
	"github.com/snowskeleton/igg-server/internal/middleware"
	"github.com/snowskeleton/igg-server/internal/model"
	"github.com/snowskeleton/igg-server/internal/store/postgres"
)

type SyncHandler struct {
	store *postgres.Store
	apns  *apns.Client
}

func NewSyncHandler(store *postgres.Store, apnsClient *apns.Client) *SyncHandler {
	return &SyncHandler{store: store, apns: apnsClient}
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

		// Track which car IDs had shared-relevant changes pushed
		changedCarIDs := make(map[string]bool)

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
			changedCarIDs[c.ID] = true
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
			changedCarIDs[svc.CarID] = true
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
			changedCarIDs[ss.CarID] = true
		}

		// Car settings: per-user, only for accessible cars
		// NOTE: CarSettings are per-user, NOT shared — do not add to changedCarIDs
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

		// ── Send push notifications for shared car changes ──
		if h.apns != nil && len(changedCarIDs) > 0 {
			carIDs := make([]string, 0, len(changedCarIDs))
			for id := range changedCarIDs {
				carIDs = append(carIDs, id)
			}
			go h.notifySharedUsers(userID, req.DeviceID, carIDs)
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

// notifySharedUsers sends push notifications to all users with access to the
// changed cars, excluding the user and device that triggered the sync.
func (h *SyncHandler) notifySharedUsers(userID, deviceID string, carIDs []string) {
	ctx, cancel := contextWithTimeout()
	defer cancel()

	userIDs, err := h.store.GetUsersWithAccessToCars(ctx, carIDs, userID)
	if err != nil {
		log.Printf("sync: get users with access: %v", err)
		return
	}
	if len(userIDs) == 0 {
		return
	}

	tokens, err := h.store.GetDeviceTokensForUsers(ctx, userIDs, deviceID)
	if err != nil {
		log.Printf("sync: get device tokens: %v", err)
		return
	}

	for _, dt := range tokens {
		var remove bool
		switch dt.NotifyMode {
		case "visible":
			remove = h.apns.SendAlert(dt.Token, "I Got Gas", "A shared vehicle was updated")
		default:
			remove = h.apns.SendBackground(dt.Token)
		}
		if remove {
			if err := h.store.DeleteDeviceTokenByToken(ctx, dt.Token); err != nil {
				log.Printf("sync: cleanup invalid token: %v", err)
			}
		}
	}
}

func contextWithTimeout() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 30*time.Second)
}
