package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// PACSType represents the type of PACS system
type PACSType string

const (
	PACSTypeDICOMWeb PACSType = "dicomweb"
	PACSTypeDIMSE    PACSType = "dimse"
	PACSTypeOrthanc  PACSType = "orthanc"
)

// PACSConfig represents a tenant's PACS configuration
type PACSConfig struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	TenantID     uuid.UUID `gorm:"type:uuid;not null;index" json:"tenant_id"`
	Name         string    `gorm:"type:varchar(255);not null" json:"name"`
	Type         PACSType  `gorm:"type:varchar(50);not null" json:"type"`
	Endpoint     string    `gorm:"type:varchar(500);not null" json:"endpoint"`
	Port         int       `gorm:"not null" json:"port"`
	AETitle      string    `gorm:"type:varchar(50)" json:"ae_title"`
	Username     string    `gorm:"type:varchar(255)" json:"username,omitempty"`
	PasswordHash string    `gorm:"type:text" json:"-"` // Encrypted password
	APIKey       string    `gorm:"type:text" json:"-"` // Encrypted API key
	Capabilities []string  `gorm:"type:text[];default:'{}'" json:"capabilities"`
	IsActive     bool      `gorm:"default:true" json:"is_active"`
	IsPrimary    bool      `gorm:"default:false" json:"is_primary"`

	// Connection status tracking
	LastConnectionTest   time.Time `gorm:"index" json:"last_connection_test,omitempty"`
	LastConnectionStatus bool      `json:"last_connection_status,omitempty"`
	LastError            string    `gorm:"type:text" json:"last_error,omitempty"`

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName overrides the table name
func (PACSConfig) TableName() string {
	return "pacs_configs"
}

// BeforeCreate hook
func (p *PACSConfig) BeforeCreate(tx *gorm.DB) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	return nil
}

// ConnectionStatus represents the status of a PACS connection
type ConnectionStatus struct {
	IsConnected  bool      `json:"is_connected"`
	LastChecked  time.Time `json:"last_checked"`
	ResponseTime int64     `json:"response_time_ms"`
	ErrorMessage string    `json:"error_message,omitempty"`
	Capabilities []string  `json:"capabilities,omitempty"`
}

// ConnectionTestRequest represents a request to test PACS connection
type ConnectionTestRequest struct {
	Type     PACSType `json:"type" binding:"required"`
	Endpoint string   `json:"endpoint" binding:"required"`
	Port     int      `json:"port" binding:"required"`
	AETitle  string   `json:"ae_title,omitempty"`
	Username string   `json:"username,omitempty"`
	Password string   `json:"password,omitempty"`
	APIKey   string   `json:"api_key,omitempty"`
}

// PACSConfigRequest represents a request to create/update PACS config
type PACSConfigRequest struct {
	Name      string   `json:"name" binding:"required"`
	Type      PACSType `json:"type" binding:"required"`
	Endpoint  string   `json:"endpoint" binding:"required"`
	Port      int      `json:"port" binding:"required"`
	AETitle   string   `json:"ae_title,omitempty"`
	Username  string   `json:"username,omitempty"`
	Password  string   `json:"password,omitempty"`
	APIKey    string   `json:"api_key,omitempty"`
	IsPrimary bool     `json:"is_primary"`
}
