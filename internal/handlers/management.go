package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/otcheredev/ris-dicom-connector/internal/middleware"
	"github.com/otcheredev/ris-dicom-connector/internal/models"
	"github.com/otcheredev/ris-dicom-connector/internal/services"
	"github.com/rs/zerolog/log"
)

type ManagementHandler struct {
	pacsService *services.PACSService
}

func NewManagementHandler(pacsService *services.PACSService) *ManagementHandler {
	return &ManagementHandler{
		pacsService: pacsService,
	}
}

// CreatePACSConfig creates a new PACS configuration
func (h *ManagementHandler) CreatePACSConfig(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID, ok := middleware.GetTenantID(ctx)
	if !ok {
		http.Error(w, "Tenant ID not found", http.StatusBadRequest)
		return
	}

	var req models.PACSConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	config, err := h.pacsService.CreatePACSConfig(ctx, tenantID, &req)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create PACS config")
		http.Error(w, "Failed to create PACS config", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(config)
}

// TestConnection tests a PACS connection
func (h *ManagementHandler) TestConnection(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req models.ConnectionTestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	status, err := h.pacsService.TestConnection(ctx, &req)
	if err != nil {
		log.Warn().Err(err).Msg("Connection test failed")
		// Still return the status with error info
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK) // Return 200 but with isConnected: false
		json.NewEncoder(w).Encode(status)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// GetPACSConfigs retrieves all PACS configurations for a tenant
func (h *ManagementHandler) GetPACSConfigs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID, ok := middleware.GetTenantID(ctx)
	if !ok {
		http.Error(w, "Tenant ID not found", http.StatusBadRequest)
		return
	}

	configs, err := h.pacsService.GetPACSConfigs(ctx, tenantID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get PACS configs")
		http.Error(w, "Failed to get PACS configs", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(configs)
}

// GetPACSConfig retrieves a specific PACS configuration
func (h *ManagementHandler) GetPACSConfig(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	configIDStr := chi.URLParam(r, "id")
	configID, err := uuid.Parse(configIDStr)
	if err != nil {
		http.Error(w, "Invalid config ID", http.StatusBadRequest)
		return
	}

	config, err := h.pacsService.GetPACSConfig(ctx, configID)
	if err != nil {
		log.Error().Err(err).Str("config_id", configIDStr).Msg("Failed to get PACS config")
		http.Error(w, "Failed to get PACS config", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(config)
}
