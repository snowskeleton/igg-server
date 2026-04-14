package handler

import (
	"encoding/json"
	"net/http"

	"github.com/snowskeleton/igg-server/internal/model"
)

func Health() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, model.HealthResponse{Status: "ok"})
	}
}

func writeJSON(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, model.ErrorResponse{Error: msg})
}
