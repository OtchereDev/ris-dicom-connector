package adapters

import (
	"fmt"
	"sync"

	"github.com/google/uuid"
	"github.com/otcheredev/ris-dicom-connector/internal/models"
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
		adapter, err = NewDICOMWebAdapter(config)
	case models.PACSTypeDIMSE:
		adapter, err = NewDIMSEAdapter(config)
	case models.PACSTypeOrthanc:
		// Orthanc supports both DICOMweb and DIMSE
		adapter, err = NewDICOMWebAdapter(config)
	default:
		return nil, fmt.Errorf("unsupported PACS type: %s", config.Type)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create adapter: %w", err)
	}

	f.adapters[config.TenantID] = adapter
	return adapter, nil
}

// RemoveAdapter removes an adapter for a tenant
func (f *AdapterFactory) RemoveAdapter(tenantID uuid.UUID) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	adapter, exists := f.adapters[tenantID]
	if !exists {
		return nil
	}

	if err := adapter.Close(); err != nil {
		return fmt.Errorf("failed to close adapter: %w", err)
	}

	delete(f.adapters, tenantID)
	return nil
}

// CloseAll closes all adapters
func (f *AdapterFactory) CloseAll() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	var errors []error
	for tenantID, adapter := range f.adapters {
		if err := adapter.Close(); err != nil {
			errors = append(errors, fmt.Errorf("failed to close adapter for tenant %s: %w", tenantID, err))
		}
		delete(f.adapters, tenantID)
	}

	if len(errors) > 0 {
		return fmt.Errorf("encountered %d errors while closing adapters", len(errors))
	}

	return nil
}
