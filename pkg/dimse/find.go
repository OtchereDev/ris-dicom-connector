package dimse

import (
	"context"
	"fmt"

	"github.com/otcheredev/ris-dicom-connector/internal/models"
)

// CFindRequest represents a C-FIND request
type CFindRequest struct {
	QueryLevel        string // PATIENT, STUDY, SERIES, IMAGE
	PatientID         string
	PatientName       string
	StudyDate         string
	StudyTime         string
	AccessionNumber   string
	Modality          string
	StudyInstanceUID  string
	SeriesInstanceUID string
}

// CFindResponse represents a C-FIND response
type CFindResponse struct {
	Status  uint16
	Results []map[string]interface{}
}

// CFind performs a C-FIND operation
func (a *Association) CFind(ctx context.Context, req CFindRequest) (*CFindResponse, error) {
	if !a.IsConnected() {
		if err := a.Connect(ctx); err != nil {
			return nil, err
		}
	}

	a.UpdateLastUsed()

	// Build C-FIND-RQ command
	command := a.buildCFindRequest(req)

	// Send C-FIND-RQ
	if err := a.sendCommand(command); err != nil {
		return nil, fmt.Errorf("failed to send C-FIND request: %w", err)
	}

	// Receive C-FIND-RSP (multiple responses)
	response := &CFindResponse{
		Results: make([]map[string]interface{}, 0),
	}

	for {
		rsp, err := a.receiveCommand(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to receive C-FIND response: %w", err)
		}

		status := a.getCommandStatus(rsp)
		response.Status = status

		// Status 0xFF00 = Pending (more results coming)
		// Status 0x0000 = Success (no more results)
		if status == 0xFF00 {
			// Parse dataset and add to results
			dataset := a.parseDICOMDataset(rsp)
			response.Results = append(response.Results, dataset)
			continue
		} else if status == 0x0000 {
			// Success - no more results
			break
		} else {
			// Error
			return nil, fmt.Errorf("C-FIND failed with status: 0x%04x", status)
		}
	}

	return response, nil
}

// CFindStudies performs a study-level C-FIND
func (a *Association) CFindStudies(ctx context.Context, params models.QueryParams) ([]models.Study, error) {
	req := CFindRequest{
		QueryLevel:      "STUDY",
		PatientID:       params.PatientID,
		PatientName:     params.PatientName,
		StudyDate:       params.StudyDate,
		AccessionNumber: params.AccessionNumber,
		Modality:        params.Modality,
	}

	response, err := a.CFind(ctx, req)
	if err != nil {
		return nil, err
	}

	// Convert DICOM datasets to Study objects
	studies := make([]models.Study, 0, len(response.Results))
	for _, dataset := range response.Results {
		study := a.datasetToStudy(dataset)
		studies = append(studies, study)
	}

	return studies, nil
}

// CFindSeries performs a series-level C-FIND
func (a *Association) CFindSeries(ctx context.Context, studyUID string) ([]models.Series, error) {
	req := CFindRequest{
		QueryLevel:       "SERIES",
		StudyInstanceUID: studyUID,
	}

	response, err := a.CFind(ctx, req)
	if err != nil {
		return nil, err
	}

	// Convert DICOM datasets to Series objects
	series := make([]models.Series, 0, len(response.Results))
	for _, dataset := range response.Results {
		s := a.datasetToSeries(dataset)
		series = append(series, s)
	}

	return series, nil
}

// CFindInstances performs an image-level C-FIND
func (a *Association) CFindInstances(ctx context.Context, studyUID, seriesUID string) ([]models.Instance, error) {
	req := CFindRequest{
		QueryLevel:        "IMAGE",
		StudyInstanceUID:  studyUID,
		SeriesInstanceUID: seriesUID,
	}

	response, err := a.CFind(ctx, req)
	if err != nil {
		return nil, err
	}

	// Convert DICOM datasets to Instance objects
	instances := make([]models.Instance, 0, len(response.Results))
	for _, dataset := range response.Results {
		instance := a.datasetToInstance(dataset)
		instances = append(instances, instance)
	}

	return instances, nil
}

// buildCFindRequest builds a C-FIND-RQ command dataset
func (a *Association) buildCFindRequest(req CFindRequest) []byte {
	// TODO: Build proper DICOM C-FIND-RQ command with:
	// - (0000,0002) Affected SOP Class UID (Study Root QR Find)
	// - (0000,0100) Command Field (C-FIND-RQ = 0x0020)
	// - (0000,0110) Message ID
	// - (0000,0700) Priority
	// - (0000,0800) Command Data Set Type (not null)
	// - Dataset with query attributes

	command := []byte{}
	// TODO: Implement
	return command
}

// parseDICOMDataset parses a DICOM dataset from response
func (a *Association) parseDICOMDataset(data []byte) map[string]interface{} {
	// TODO: Parse DICOM dataset properly
	// For now, return empty map
	return make(map[string]interface{})
}

// datasetToStudy converts DICOM dataset to Study model
func (a *Association) datasetToStudy(dataset map[string]interface{}) models.Study {
	// TODO: Map DICOM tags to Study fields
	return models.Study{}
}

// datasetToSeries converts DICOM dataset to Series model
func (a *Association) datasetToSeries(dataset map[string]interface{}) models.Series {
	// TODO: Map DICOM tags to Series fields
	return models.Series{}
}

// datasetToInstance converts DICOM dataset to Instance model
func (a *Association) datasetToInstance(dataset map[string]interface{}) models.Instance {
	// TODO: Map DICOM tags to Instance fields
	return models.Instance{}
}
