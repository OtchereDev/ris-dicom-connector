package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/otcheredev/ris-dicom-connector/internal/database"
)

type HealthHandler struct{}

func NewHealthHandler() *HealthHandler {
	return &HealthHandler{}
}

type healthResponse struct {
	Status    string            `json:"status"`
	Timestamp time.Time         `json:"timestamp"`
	Services  map[string]string `json:"services"`
}

func (h *HealthHandler) Health(w http.ResponseWriter, r *http.Request) {
	response := healthResponse{
		Status:    "healthy",
		Timestamp: time.Now(),
		Services:  make(map[string]string),
	}

	// Check database
	sqlDB, err := database.DB.DB()
	if err != nil || sqlDB.Ping() != nil {
		response.Services["database"] = "unhealthy"
		response.Status = "degraded"
	} else {
		response.Services["database"] = "healthy"
	}

	w.Header().Set("Content-Type", "application/json")
	if response.Status != "healthy" {
		w.WriteHeader(http.StatusServiceUnavailable)
	}
	json.NewEncoder(w).Encode(response)
}

func (h *HealthHandler) Ready(w http.ResponseWriter, r *http.Request) {
	// Check if service is ready to accept requests
	sqlDB, err := database.DB.DB()
	if err != nil || sqlDB.Ping() != nil {
		http.Error(w, "Service not ready", http.StatusServiceUnavailable)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}
