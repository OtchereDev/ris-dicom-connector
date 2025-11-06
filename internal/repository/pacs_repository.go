package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/otcheredev/ris-dicom-connector/internal/database"
	"github.com/otcheredev/ris-dicom-connector/internal/models"
)

// PACSRepository handles PACS configuration database operations
type PACSRepository struct{}

// NewPACSRepository creates a new PACS repository
func NewPACSRepository() *PACSRepository {
	return &PACSRepository{}
}

// Create creates a new PACS configuration
func (r *PACSRepository) Create(ctx context.Context, config *models.PACSConfig) error {
	if err := database.DB.WithContext(ctx).Create(config).Error; err != nil {
		return fmt.Errorf("failed to create PACS config: %w", err)
	}
	return nil
}

// GetByID retrieves a PACS configuration by ID
func (r *PACSRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.PACSConfig, error) {
	var config models.PACSConfig
	if err := database.DB.WithContext(ctx).Where("id = ?", id).First(&config).Error; err != nil {
		return nil, fmt.Errorf("failed to get PACS config: %w", err)
	}
	return &config, nil
}

// GetByTenantID retrieves all PACS configurations for a tenant
func (r *PACSRepository) GetByTenantID(ctx context.Context, tenantID uuid.UUID) ([]models.PACSConfig, error) {
	var configs []models.PACSConfig
	if err := database.DB.WithContext(ctx).
		Where("tenant_id = ? AND is_active = ?", tenantID, true).
		Order("is_primary DESC, created_at ASC").
		Find(&configs).Error; err != nil {
		return nil, fmt.Errorf("failed to get PACS configs: %w", err)
	}
	return configs, nil
}

// GetPrimaryByTenantID retrieves the primary PACS configuration for a tenant
func (r *PACSRepository) GetPrimaryByTenantID(ctx context.Context, tenantID uuid.UUID) (*models.PACSConfig, error) {
	var config models.PACSConfig
	if err := database.DB.WithContext(ctx).
		Where("tenant_id = ? AND is_primary = ? AND is_active = ?", tenantID, true, true).
		First(&config).Error; err != nil {
		return nil, fmt.Errorf("failed to get primary PACS config: %w", err)
	}
	return &config, nil
}

// Update updates a PACS configuration
func (r *PACSRepository) Update(ctx context.Context, config *models.PACSConfig) error {
	if err := database.DB.WithContext(ctx).Save(config).Error; err != nil {
		return fmt.Errorf("failed to update PACS config: %w", err)
	}
	return nil
}

// Delete soft deletes a PACS configuration
func (r *PACSRepository) Delete(ctx context.Context, id uuid.UUID) error {
	if err := database.DB.WithContext(ctx).Delete(&models.PACSConfig{}, id).Error; err != nil {
		return fmt.Errorf("failed to delete PACS config: %w", err)
	}
	return nil
}

// SetPrimary sets a PACS configuration as primary (and unsets others)
func (r *PACSRepository) SetPrimary(ctx context.Context, id uuid.UUID, tenantID uuid.UUID) error {
	// Start transaction
	tx := database.DB.WithContext(ctx).Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Unset all primary flags for this tenant
	if err := tx.Model(&models.PACSConfig{}).
		Where("tenant_id = ?", tenantID).
		Update("is_primary", false).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to unset primary flags: %w", err)
	}

	// Set new primary
	if err := tx.Model(&models.PACSConfig{}).
		Where("id = ?", id).
		Update("is_primary", true).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to set primary: %w", err)
	}

	return tx.Commit().Error
}

// UpdateConnectionStatus updates the connection status of a PACS configuration
func (r *PACSRepository) UpdateConnectionStatus(ctx context.Context, id uuid.UUID, status *models.ConnectionStatus) error {
	updates := map[string]interface{}{
		"last_connection_test":   status.LastChecked,
		"last_connection_status": status.IsConnected,
		"last_error":             status.ErrorMessage,
	}

	if err := database.DB.WithContext(ctx).
		Model(&models.PACSConfig{}).
		Where("id = ?", id).
		Updates(updates).Error; err != nil {
		return fmt.Errorf("failed to update connection status: %w", err)
	}

	return nil
}
