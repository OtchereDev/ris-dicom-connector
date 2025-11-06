package adapters

import (
	"context"
	"io"

	"github.com/otcheredev/ris-dicom-connector/internal/models"
)

// PACSAdapter defines the interface that all PACS adapters must implement
type PACSAdapter interface {
	// Query operations
	FindStudies(ctx context.Context, params models.QueryParams) ([]models.Study, error)
	FindSeries(ctx context.Context, studyUID string) ([]models.Series, error)
	FindInstances(ctx context.Context, studyUID, seriesUID string) ([]models.Instance, error)

	// Retrieve operations
	GetInstance(ctx context.Context, studyUID, seriesUID, instanceUID string) (io.ReadCloser, string, error)
	GetInstanceMetadata(ctx context.Context, studyUID, seriesUID, instanceUID string) (*models.Metadata, error)
	GetStudyMetadata(ctx context.Context, studyUID string) ([]models.Metadata, error)

	// Thumbnail operations
	GetThumbnail(ctx context.Context, studyUID, seriesUID, instanceUID string, size int) ([]byte, error)

	// Connection management
	TestConnection(ctx context.Context) (*models.ConnectionStatus, error)
	Close() error

	// Adapter info
	Type() models.PACSType
	Capabilities() []string
}

// BaseAdapter provides common functionality for all adapters
type BaseAdapter struct {
	config models.PACSConfig
}

func (b *BaseAdapter) Type() models.PACSType {
	return b.config.Type
}

func (b *BaseAdapter) GetConfig() models.PACSConfig {
	return b.config
}
