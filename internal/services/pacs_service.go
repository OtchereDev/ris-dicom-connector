package services

import (
	"context"
	"fmt"
	"io"

	"github.com/google/uuid"
	"github.com/otcheredev/ris-dicom-connector/internal/adapters"
	"github.com/otcheredev/ris-dicom-connector/internal/cache"
	"github.com/otcheredev/ris-dicom-connector/internal/models"
	"github.com/otcheredev/ris-dicom-connector/internal/repository"
)

// PACSService handles business logic for PACS operations
type PACSService struct {
	pacsRepo       *repository.PACSRepository
	auditRepo      *repository.AuditRepository
	adapterFactory *adapters.AdapterFactory
	cache          cache.Cache
}

// NewPACSService creates a new PACS service
func NewPACSService(
	pacsRepo *repository.PACSRepository,
	auditRepo *repository.AuditRepository,
	adapterFactory *adapters.AdapterFactory,
	cache cache.Cache,
) *PACSService {
	return &PACSService{
		pacsRepo:       pacsRepo,
		auditRepo:      auditRepo,
		adapterFactory: adapterFactory,
		cache:          cache,
	}
}

// GetAdapter gets a PACS adapter for a tenant
func (s *PACSService) GetAdapter(ctx context.Context, tenantID uuid.UUID) (adapters.PACSAdapter, error) {
	// Get primary PACS config for tenant
	config, err := s.pacsRepo.GetPrimaryByTenantID(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to get PACS config: %w", err)
	}

	// Get or create adapter
	adapter, err := s.adapterFactory.GetAdapter(*config)
	if err != nil {
		return nil, fmt.Errorf("failed to get adapter: %w", err)
	}

	return adapter, nil
}

// CreatePACSConfig creates a new PACS configuration
func (s *PACSService) CreatePACSConfig(ctx context.Context, tenantID uuid.UUID, req *models.PACSConfigRequest) (*models.PACSConfig, error) {
	config := &models.PACSConfig{
		TenantID:  tenantID,
		Name:      req.Name,
		Type:      req.Type,
		Endpoint:  req.Endpoint,
		Port:      req.Port,
		AETitle:   req.AETitle,
		Username:  req.Username,
		IsPrimary: req.IsPrimary,
		IsActive:  true,
	}

	// TODO: Encrypt password and API key before storing
	if req.Password != "" {
		config.PasswordHash = req.Password // Should be encrypted
	}
	if req.APIKey != "" {
		config.APIKey = req.APIKey // Should be encrypted
	}

	// If this is set as primary, unset others
	if req.IsPrimary {
		if err := s.pacsRepo.SetPrimary(ctx, uuid.Nil, tenantID); err != nil {
			return nil, fmt.Errorf("failed to unset primary flags: %w", err)
		}
	}

	if err := s.pacsRepo.Create(ctx, config); err != nil {
		return nil, fmt.Errorf("failed to create PACS config: %w", err)
	}

	return config, nil
}

// TestConnection tests a PACS connection
func (s *PACSService) TestConnection(ctx context.Context, req *models.ConnectionTestRequest) (*models.ConnectionStatus, error) {
	// Create temporary config for testing
	config := models.PACSConfig{
		Type:         req.Type,
		Endpoint:     req.Endpoint,
		Port:         req.Port,
		AETitle:      req.AETitle,
		Username:     req.Username,
		PasswordHash: req.Password,
		APIKey:       req.APIKey,
	}

	// Create temporary adapter
	var adapter adapters.PACSAdapter
	var err error

	switch req.Type {
	case models.PACSTypeDICOMWeb, models.PACSTypeOrthanc:
		adapter, err = adapters.NewDICOMWebAdapter(config)
	default:
		return nil, fmt.Errorf("unsupported PACS type: %s", req.Type)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create adapter: %w", err)
	}
	defer adapter.Close()

	// Test connection
	status, err := adapter.TestConnection(ctx)
	if err != nil {
		return status, err
	}

	return status, nil
}

// FindStudies queries for studies
func (s *PACSService) FindStudies(ctx context.Context, tenantID uuid.UUID, params models.QueryParams) ([]models.Study, error) {
	adapter, err := s.GetAdapter(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	studies, err := adapter.FindStudies(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to find studies: %w", err)
	}

	return studies, nil
}

// FindSeries queries for series
func (s *PACSService) FindSeries(ctx context.Context, tenantID uuid.UUID, studyUID string) ([]models.Series, error) {
	adapter, err := s.GetAdapter(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	series, err := adapter.FindSeries(ctx, studyUID)
	if err != nil {
		return nil, fmt.Errorf("failed to find series: %w", err)
	}

	return series, nil
}

// FindInstances queries for instances
func (s *PACSService) FindInstances(ctx context.Context, tenantID uuid.UUID, studyUID, seriesUID string) ([]models.Instance, error) {
	adapter, err := s.GetAdapter(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	instances, err := adapter.FindInstances(ctx, studyUID, seriesUID)
	if err != nil {
		return nil, fmt.Errorf("failed to find instances: %w", err)
	}

	return instances, nil
}

// GetInstance retrieves an instance with caching
func (s *PACSService) GetInstance(ctx context.Context, tenantID uuid.UUID, studyUID, seriesUID, instanceUID string) (io.ReadCloser, string, error) {
	// Try cache first
	cacheKey := cache.CacheKey(tenantID.String(), studyUID, seriesUID, instanceUID, "instance")

	_, err := s.cache.Get(ctx, cacheKey)
	if err == nil {
		// Cache hit
		return io.NopCloser(io.Reader(nil)), "application/dicom", nil // TODO: Return proper reader
	}

	// Cache miss - fetch from PACS
	adapter, err := s.GetAdapter(ctx, tenantID)
	if err != nil {
		return nil, "", err
	}

	data, contentType, err := adapter.GetInstance(ctx, studyUID, seriesUID, instanceUID)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get instance: %w", err)
	}

	// TODO: Cache the data asynchronously

	return data, contentType, nil
}

// Add these methods to the PACSService

// GetPACSConfigs retrieves all PACS configurations for a tenant
func (s *PACSService) GetPACSConfigs(ctx context.Context, tenantID uuid.UUID) ([]models.PACSConfig, error) {
	configs, err := s.pacsRepo.GetByTenantID(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to get PACS configs: %w", err)
	}
	return configs, nil
}

// GetPACSConfig retrieves a specific PACS configuration
func (s *PACSService) GetPACSConfig(ctx context.Context, configID uuid.UUID) (*models.PACSConfig, error) {
	config, err := s.pacsRepo.GetByID(ctx, configID)
	if err != nil {
		return nil, fmt.Errorf("failed to get PACS config: %w", err)
	}
	return config, nil
}
