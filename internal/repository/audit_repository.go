package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/otcheredev/ris-dicom-connector/internal/database"
	"github.com/otcheredev/ris-dicom-connector/internal/models"
)

// AuditRepository handles audit log database operations
type AuditRepository struct{}

// NewAuditRepository creates a new audit repository
func NewAuditRepository() *AuditRepository {
	return &AuditRepository{}
}

// Create creates a new audit log entry
func (r *AuditRepository) Create(ctx context.Context, log *models.AuditLog) error {
	if err := database.DB.WithContext(ctx).Create(log).Error; err != nil {
		return fmt.Errorf("failed to create audit log: %w", err)
	}
	return nil
}

// GetByTenantID retrieves audit logs for a tenant
func (r *AuditRepository) GetByTenantID(ctx context.Context, tenantID uuid.UUID, limit, offset int) ([]models.AuditLog, error) {
	var logs []models.AuditLog
	query := database.DB.WithContext(ctx).
		Where("tenant_id = ?", tenantID).
		Order("created_at DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}

	if err := query.Find(&logs).Error; err != nil {
		return nil, fmt.Errorf("failed to get audit logs: %w", err)
	}

	return logs, nil
}

// GetByResourceUID retrieves audit logs for a specific resource
func (r *AuditRepository) GetByResourceUID(ctx context.Context, tenantID uuid.UUID, resourceUID string) ([]models.AuditLog, error) {
	var logs []models.AuditLog
	if err := database.DB.WithContext(ctx).
		Where("tenant_id = ? AND resource_uid = ?", tenantID, resourceUID).
		Order("created_at DESC").
		Find(&logs).Error; err != nil {
		return nil, fmt.Errorf("failed to get audit logs: %w", err)
	}
	return logs, nil
}
