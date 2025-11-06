package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// AuditLog represents an audit log entry
type AuditLog struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	TenantID     uuid.UUID `gorm:"type:uuid;not null;index" json:"tenant_id"`
	UserID       uuid.UUID `gorm:"type:uuid;index" json:"user_id"`
	Action       string    `gorm:"type:varchar(100);not null;index" json:"action"`
	ResourceType string    `gorm:"type:varchar(50);index" json:"resource_type"`
	ResourceUID  string    `gorm:"type:varchar(255);index" json:"resource_uid"`
	IPAddress    string    `gorm:"type:varchar(45)" json:"ip_address"`
	UserAgent    string    `gorm:"type:text" json:"user_agent"`
	Status       string    `gorm:"type:varchar(20);index" json:"status"` // success, failure
	ErrorMessage string    `gorm:"type:text" json:"error_message,omitempty"`
	Duration     int64     `json:"duration_ms"` // milliseconds
	CreatedAt    time.Time `gorm:"index" json:"timestamp"`
}

// TableName overrides the table name
func (AuditLog) TableName() string {
	return "audit_logs"
}

// BeforeCreate hook
func (a *AuditLog) BeforeCreate(tx *gorm.DB) error {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	return nil
}

// CacheMetrics tracks cache performance metrics
type CacheMetrics struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	TenantID  uuid.UUID `gorm:"type:uuid;not null;index" json:"tenant_id"`
	CacheKey  string    `gorm:"type:varchar(500);not null" json:"cache_key"`
	CacheHit  bool      `gorm:"not null;index" json:"cache_hit"`
	CacheTier string    `gorm:"type:varchar(20)" json:"cache_tier"` // redis, s3, pacs
	Size      int64     `json:"size_bytes"`
	Duration  int64     `json:"duration_ms"`
	CreatedAt time.Time `gorm:"index" json:"timestamp"`
}

// TableName overrides the table name
func (CacheMetrics) TableName() string {
	return "cache_metrics"
}

// BeforeCreate hook
func (c *CacheMetrics) BeforeCreate(tx *gorm.DB) error {
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	return nil
}
