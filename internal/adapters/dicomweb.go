package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/otcheredev/ris-dicom-connector/internal/models"
)

// DICOMWebAdapter implements PACSAdapter for DICOMweb protocol
type DICOMWebAdapter struct {
	BaseAdapter
	client   *http.Client
	baseURL  string
	username string
	password string
	apiKey   string
}

// NewDICOMWebAdapter creates a new DICOMweb adapter
func NewDICOMWebAdapter(config models.PACSConfig) (*DICOMWebAdapter, error) {
	// Build base URL
	scheme := "http"
	if config.Port == 443 {
		scheme = "https"
	}
	baseURL := fmt.Sprintf("%s://%s:%d/dicom-web", scheme, config.Endpoint, config.Port)

	return &DICOMWebAdapter{
		BaseAdapter: BaseAdapter{config: config},
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		baseURL:  baseURL,
		username: config.Username,
		password: config.PasswordHash, // In production, decrypt this
		apiKey:   config.APIKey,
	}, nil
}

func (d *DICOMWebAdapter) Type() models.PACSType {
	return models.PACSTypeDICOMWeb
}

func (d *DICOMWebAdapter) Capabilities() []string {
	return []string{"QIDO-RS", "WADO-RS", "WADO-URI"}
}

// FindStudies queries for studies using QIDO-RS
func (d *DICOMWebAdapter) FindStudies(ctx context.Context, params models.QueryParams) ([]models.Study, error) {
	// Build QIDO-RS query URL
	queryURL := fmt.Sprintf("%s/studies", d.baseURL)

	// Add query parameters
	urlParams := url.Values{}
	if params.PatientID != "" {
		urlParams.Add("PatientID", params.PatientID)
	}
	if params.PatientName != "" {
		urlParams.Add("PatientName", params.PatientName)
	}
	if params.StudyDate != "" {
		urlParams.Add("StudyDate", params.StudyDate)
	}
	if params.AccessionNumber != "" {
		urlParams.Add("AccessionNumber", params.AccessionNumber)
	}
	if params.Modality != "" {
		urlParams.Add("ModalitiesInStudy", params.Modality)
	}
	if params.StudyDescription != "" {
		urlParams.Add("StudyDescription", params.StudyDescription)
	}
	if params.Limit > 0 {
		urlParams.Add("limit", fmt.Sprintf("%d", params.Limit))
	}
	if params.Offset > 0 {
		urlParams.Add("offset", fmt.Sprintf("%d", params.Offset))
	}

	if len(urlParams) > 0 {
		queryURL = queryURL + "?" + urlParams.Encode()
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, "GET", queryURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add authentication
	d.addAuth(req)

	// Set headers
	req.Header.Set("Accept", "application/dicom+json")

	// Execute request
	resp, err := d.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("PACS returned status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var studies []models.Study
	if err := json.NewDecoder(resp.Body).Decode(&studies); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return studies, nil
}

// FindSeries queries for series using QIDO-RS
func (d *DICOMWebAdapter) FindSeries(ctx context.Context, studyUID string) ([]models.Series, error) {
	queryURL := fmt.Sprintf("%s/studies/%s/series", d.baseURL, studyUID)

	req, err := http.NewRequestWithContext(ctx, "GET", queryURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	d.addAuth(req)
	req.Header.Set("Accept", "application/dicom+json")

	resp, err := d.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("PACS returned status %d: %s", resp.StatusCode, string(body))
	}

	var series []models.Series
	if err := json.NewDecoder(resp.Body).Decode(&series); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return series, nil
}

// FindInstances queries for instances using QIDO-RS
func (d *DICOMWebAdapter) FindInstances(ctx context.Context, studyUID, seriesUID string) ([]models.Instance, error) {
	queryURL := fmt.Sprintf("%s/studies/%s/series/%s/instances", d.baseURL, studyUID, seriesUID)

	req, err := http.NewRequestWithContext(ctx, "GET", queryURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	d.addAuth(req)
	req.Header.Set("Accept", "application/dicom+json")

	resp, err := d.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("PACS returned status %d: %s", resp.StatusCode, string(body))
	}

	var instances []models.Instance
	if err := json.NewDecoder(resp.Body).Decode(&instances); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return instances, nil
}

// GetInstance retrieves an instance using WADO-RS
func (d *DICOMWebAdapter) GetInstance(ctx context.Context, studyUID, seriesUID, instanceUID string) (io.ReadCloser, string, error) {
	retrieveURL := fmt.Sprintf("%s/studies/%s/series/%s/instances/%s",
		d.baseURL, studyUID, seriesUID, instanceUID)

	req, err := http.NewRequestWithContext(ctx, "GET", retrieveURL, nil)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create request: %w", err)
	}

	d.addAuth(req)
	req.Header.Set("Accept", "application/dicom, multipart/related; type=application/dicom")

	resp, err := d.client.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("failed to execute request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		return nil, "", fmt.Errorf("PACS returned status %d: %s", resp.StatusCode, string(body))
	}

	contentType := resp.Header.Get("Content-Type")
	return resp.Body, contentType, nil
}

// GetInstanceMetadata retrieves instance metadata
func (d *DICOMWebAdapter) GetInstanceMetadata(ctx context.Context, studyUID, seriesUID, instanceUID string) (*models.Metadata, error) {
	metadataURL := fmt.Sprintf("%s/studies/%s/series/%s/instances/%s/metadata",
		d.baseURL, studyUID, seriesUID, instanceUID)

	req, err := http.NewRequestWithContext(ctx, "GET", metadataURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	d.addAuth(req)
	req.Header.Set("Accept", "application/dicom+json")

	resp, err := d.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("PACS returned status %d: %s", resp.StatusCode, string(body))
	}

	var metadata models.Metadata
	if err := json.NewDecoder(resp.Body).Decode(&metadata); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &metadata, nil
}

// GetStudyMetadata retrieves metadata for all instances in a study
func (d *DICOMWebAdapter) GetStudyMetadata(ctx context.Context, studyUID string) ([]models.Metadata, error) {
	metadataURL := fmt.Sprintf("%s/studies/%s/metadata", d.baseURL, studyUID)

	req, err := http.NewRequestWithContext(ctx, "GET", metadataURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	d.addAuth(req)
	req.Header.Set("Accept", "application/dicom+json")

	resp, err := d.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("PACS returned status %d: %s", resp.StatusCode, string(body))
	}

	var metadata []models.Metadata
	if err := json.NewDecoder(resp.Body).Decode(&metadata); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return metadata, nil
}

// GetThumbnail generates a thumbnail (placeholder for now)
func (d *DICOMWebAdapter) GetThumbnail(ctx context.Context, studyUID, seriesUID, instanceUID string, size int) ([]byte, error) {
	// TODO: Implement thumbnail generation
	// For now, return error indicating not implemented
	return nil, fmt.Errorf("thumbnail generation not yet implemented")
}

// TestConnection tests the PACS connection
func (d *DICOMWebAdapter) TestConnection(ctx context.Context) (*models.ConnectionStatus, error) {
	start := time.Now()
	status := &models.ConnectionStatus{
		LastChecked: start,
	}

	// Try to query for studies (empty query to test connection)
	_, err := d.FindStudies(ctx, models.QueryParams{Limit: 1})

	status.ResponseTime = time.Since(start).Milliseconds()

	if err != nil {
		status.IsConnected = false
		status.ErrorMessage = err.Error()
		return status, err
	}

	status.IsConnected = true
	status.Capabilities = d.Capabilities()
	return status, nil
}

// Close closes the adapter
func (d *DICOMWebAdapter) Close() error {
	d.client.CloseIdleConnections()
	return nil
}

// addAuth adds authentication to the request
func (d *DICOMWebAdapter) addAuth(req *http.Request) {
	if d.apiKey != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", d.apiKey))
	} else if d.username != "" && d.password != "" {
		req.SetBasicAuth(d.username, d.password)
	}
}
