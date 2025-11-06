package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/otcheredev/ris-dicom-connector/internal/middleware"
	"github.com/otcheredev/ris-dicom-connector/internal/models"
	"github.com/otcheredev/ris-dicom-connector/internal/services"
	"github.com/rs/zerolog/log"
)

type DICOMWebHandler struct {
	pacsService *services.PACSService
}

func NewDICOMWebHandler(pacsService *services.PACSService) *DICOMWebHandler {
	return &DICOMWebHandler{
		pacsService: pacsService,
	}
}

// SearchStudies handles QIDO-RS study search
func (h *DICOMWebHandler) SearchStudies(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID, ok := middleware.GetTenantID(ctx)
	if !ok {
		http.Error(w, "Tenant ID not found", http.StatusBadRequest)
		return
	}

	// Parse query parameters
	params := models.QueryParams{
		PatientID:        r.URL.Query().Get("PatientID"),
		PatientName:      r.URL.Query().Get("PatientName"),
		StudyDate:        r.URL.Query().Get("StudyDate"),
		AccessionNumber:  r.URL.Query().Get("AccessionNumber"),
		Modality:         r.URL.Query().Get("ModalitiesInStudy"),
		StudyDescription: r.URL.Query().Get("StudyDescription"),
	}

	if limit := r.URL.Query().Get("limit"); limit != "" {
		params.Limit, _ = strconv.Atoi(limit)
	}
	if offset := r.URL.Query().Get("offset"); offset != "" {
		params.Offset, _ = strconv.Atoi(offset)
	}

	studies, err := h.pacsService.FindStudies(ctx, tenantID, params)
	if err != nil {
		log.Error().Err(err).Msg("Failed to search studies")
		http.Error(w, "Failed to search studies", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/dicom+json")
	json.NewEncoder(w).Encode(studies)
}

// GetStudyMetadata handles WADO-RS metadata retrieval
func (h *DICOMWebHandler) GetStudyMetadata(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID, ok := middleware.GetTenantID(ctx)
	if !ok {
		http.Error(w, "Tenant ID not found", http.StatusBadRequest)
		return
	}

	studyUID := chi.URLParam(r, "studyUID")
	if studyUID == "" {
		http.Error(w, "Study UID is required", http.StatusBadRequest)
		return
	}

	// For now, return series instead of full metadata
	series, err := h.pacsService.FindSeries(ctx, tenantID, studyUID)
	if err != nil {
		log.Error().Err(err).Str("study_uid", studyUID).Msg("Failed to get study metadata")
		http.Error(w, "Failed to get study metadata", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/dicom+json")
	json.NewEncoder(w).Encode(series)
}

// SearchSeries handles QIDO-RS series search
func (h *DICOMWebHandler) SearchSeries(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID, ok := middleware.GetTenantID(ctx)
	if !ok {
		http.Error(w, "Tenant ID not found", http.StatusBadRequest)
		return
	}

	studyUID := chi.URLParam(r, "studyUID")
	if studyUID == "" {
		http.Error(w, "Study UID is required", http.StatusBadRequest)
		return
	}

	series, err := h.pacsService.FindSeries(ctx, tenantID, studyUID)
	if err != nil {
		log.Error().Err(err).Str("study_uid", studyUID).Msg("Failed to search series")
		http.Error(w, "Failed to search series", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/dicom+json")
	json.NewEncoder(w).Encode(series)
}

// SearchInstances handles QIDO-RS instance search
func (h *DICOMWebHandler) SearchInstances(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID, ok := middleware.GetTenantID(ctx)
	if !ok {
		http.Error(w, "Tenant ID not found", http.StatusBadRequest)
		return
	}

	studyUID := chi.URLParam(r, "studyUID")
	seriesUID := chi.URLParam(r, "seriesUID")

	if studyUID == "" || seriesUID == "" {
		http.Error(w, "Study UID and Series UID are required", http.StatusBadRequest)
		return
	}

	instances, err := h.pacsService.FindInstances(ctx, tenantID, studyUID, seriesUID)
	if err != nil {
		log.Error().Err(err).
			Str("study_uid", studyUID).
			Str("series_uid", seriesUID).
			Msg("Failed to search instances")
		http.Error(w, "Failed to search instances", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/dicom+json")
	json.NewEncoder(w).Encode(instances)
}

// RetrieveInstance handles WADO-RS instance retrieval
func (h *DICOMWebHandler) RetrieveInstance(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID, ok := middleware.GetTenantID(ctx)
	if !ok {
		http.Error(w, "Tenant ID not found", http.StatusBadRequest)
		return
	}

	studyUID := chi.URLParam(r, "studyUID")
	seriesUID := chi.URLParam(r, "seriesUID")
	instanceUID := chi.URLParam(r, "instanceUID")

	if studyUID == "" || seriesUID == "" || instanceUID == "" {
		http.Error(w, "Study UID, Series UID, and Instance UID are required", http.StatusBadRequest)
		return
	}

	data, contentType, err := h.pacsService.GetInstance(ctx, tenantID, studyUID, seriesUID, instanceUID)
	if err != nil {
		log.Error().Err(err).
			Str("study_uid", studyUID).
			Str("series_uid", seriesUID).
			Str("instance_uid", instanceUID).
			Msg("Failed to retrieve instance")
		http.Error(w, "Failed to retrieve instance", http.StatusInternalServerError)
		return
	}
	defer data.Close()

	w.Header().Set("Content-Type", contentType)
	io.Copy(w, data)
}
