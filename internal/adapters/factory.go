package adapters

import (
	"fmt"
	"sync"

	"github.com/google/uuid"
	"github.com/otcheredev/ris-dicom-connector/internal/models"
	"github.com/rs/zerolog/log"
)

// AdapterFactory manages PACS adapter instances
type AdapterFactory struct {
	mu       sync.RWMutex
	adapters map[uuid.UUID]PACSAdapter // keyed by tenant ID
}

// NewAdapterFactory creates a new adapter factory
func NewAdapterFactory() *AdapterFactory {
	return &AdapterFactory{
		adapters: make(map[uuid.UUID]PACSAdapter),
	}
}

// GetAdapter gets or creates an adapter for a tenant
func (f *AdapterFactory) GetAdapter(config models.PACSConfig) (PACSAdapter, error) {
	f.mu.RLock()
	adapter, exists := f.adapters[config.TenantID]
	f.mu.RUnlock()

	if exists {
		log.Debug().
			Str("tenant_id", config.TenantID.String()).
			Str("type", string(config.Type)).
			Msg("Reusing existing adapter")
		return adapter, nil
	}

	// Create new adapter
	f.mu.Lock()
	defer f.mu.Unlock()

	// Double-check after acquiring write lock
	if adapter, exists := f.adapters[config.TenantID]; exists {
		return adapter, nil
	}

	var err error
	switch config.Type {
	case models.PACSTypeDICOMWeb:
		log.Info().
			Str("tenant_id", config.TenantID.String()).
			Str("endpoint", config.Endpoint).
			Msg("Creating DICOMweb adapter")
		adapter, err = NewDICOMWebAdapter(config)

	case models.PACSTypeDIMSE:
		log.Info().
			Str("tenant_id", config.TenantID.String()).
			Str("endpoint", config.Endpoint).
			Int("port", config.Port).
			Str("ae_title", config.AETitle).
			Msg("Creating DIMSE adapter")
		adapter, err = NewDIMSEAdapter(config)

	case models.PACSTypeOrthanc:
		// Orthanc supports both DICOMweb and DIMSE
		// For now, use DICOMweb as it's more feature-complete
		log.Info().
			Str("tenant_id", config.TenantID.String()).
			Str("endpoint", config.Endpoint).
			Msg("Creating Orthanc adapter (using DICOMweb)")
		adapter, err = NewDICOMWebAdapter(config)

	default:
		return nil, fmt.Errorf("unsupported PACS type: %s", config.Type)
	}

	if err != nil {
		log.Error().
			Err(err).
			Str("tenant_id", config.TenantID.String()).
			Str("type", string(config.Type)).
			Msg("Failed to create adapter")
		return nil, fmt.Errorf("failed to create adapter: %w", err)
	}

	f.adapters[config.TenantID] = adapter

	log.Info().
		Str("tenant_id", config.TenantID.String()).
		Str("type", string(config.Type)).
		Strs("capabilities", adapter.Capabilities()).
		Msg("Adapter created and cached")

	return adapter, nil
}

// RemoveAdapter removes an adapter for a tenant
func (f *AdapterFactory) RemoveAdapter(tenantID uuid.UUID) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	adapter, exists := f.adapters[tenantID]
	if !exists {
		log.Debug().
			Str("tenant_id", tenantID.String()).
			Msg("Adapter not found, nothing to remove")
		return nil
	}

	if err := adapter.Close(); err != nil {
		log.Error().
			Err(err).
			Str("tenant_id", tenantID.String()).
			Msg("Failed to close adapter")
		return fmt.Errorf("failed to close adapter: %w", err)
	}

	delete(f.adapters, tenantID)

	log.Info().
		Str("tenant_id", tenantID.String()).
		Msg("Adapter removed")

	return nil
}

// CloseAll closes all adapters
func (f *AdapterFactory) CloseAll() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	log.Info().
		Int("num_adapters", len(f.adapters)).
		Msg("Closing all adapters")

	var errors []error
	for tenantID, adapter := range f.adapters {
		if err := adapter.Close(); err != nil {
			log.Error().
				Err(err).
				Str("tenant_id", tenantID.String()).
				Msg("Failed to close adapter")
			errors = append(errors, fmt.Errorf("failed to close adapter for tenant %s: %w", tenantID, err))
		}
		delete(f.adapters, tenantID)
	}

	if len(errors) > 0 {
		log.Warn().
			Int("num_errors", len(errors)).
			Msg("Encountered errors while closing adapters")
		return fmt.Errorf("encountered %d errors while closing adapters", len(errors))
	}

	log.Info().Msg("All adapters closed successfully")
	return nil
}

// GetStats returns statistics about the adapter factory
func (f *AdapterFactory) GetStats() AdapterStats {
	f.mu.RLock()
	defer f.mu.RUnlock()

	stats := AdapterStats{
		TotalAdapters: len(f.adapters),
		AdapterTypes:  make(map[string]int),
	}

	for _, adapter := range f.adapters {
		adapterType := string(adapter.Type())
		stats.AdapterTypes[adapterType]++
	}

	return stats
}

// AdapterStats holds statistics about adapters
type AdapterStats struct {
	TotalAdapters int            `json:"total_adapters"`
	AdapterTypes  map[string]int `json:"adapter_types"` // e.g., {"dicomweb": 5, "dimse": 3}
}
