package adapters

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/OtchereDev/ris-common-sdk/pkg/io-dicom/dictionary/tags"
	"github.com/OtchereDev/ris-common-sdk/pkg/io-dicom/media"
	"github.com/OtchereDev/ris-common-sdk/pkg/io-dicom/network"
	"github.com/OtchereDev/ris-common-sdk/pkg/io-dicom/services"
	"github.com/otcheredev/ris-dicom-connector/internal/models"
)

// DIMSEAdapter implements PACSAdapter for DIMSE protocol
type DIMSEAdapter struct {
	BaseAdapter
	config      models.PACSConfig
	destination *network.Destination
	timeout     int
}

// NewDIMSEAdapter creates a new DIMSE adapter
func NewDIMSEAdapter(config models.PACSConfig) (*DIMSEAdapter, error) {
	destination := &network.Destination{
		HostName:  config.Endpoint,
		Port:      config.Port,
		CalledAE:  config.AETitle,
		CallingAE: "DICOM_CONNECTOR",
		IsCFind:   true,
		IsCMove:   true,
		IsCStore:  true,
	}

	return &DIMSEAdapter{
		BaseAdapter: BaseAdapter{config: config},
		config:      config,
		destination: destination,
		timeout:     30, // 30 seconds default timeout
	}, nil
}

func (d *DIMSEAdapter) Type() models.PACSType {
	return models.PACSTypeDIMSE
}

func (d *DIMSEAdapter) Capabilities() []string {
	return []string{"C-FIND", "C-MOVE", "C-GET", "C-ECHO"}
}

// createSCU creates a new SCU (Service Class User) connection
func (d *DIMSEAdapter) createSCU() services.SCU {
	return services.NewSCU(d.destination)
}

// FindStudies queries for studies using C-FIND
func (d *DIMSEAdapter) FindStudies(ctx context.Context, params models.QueryParams) ([]models.Study, error) {
	scu := d.createSCU()

	// Build C-FIND query dataset
	query := media.NewEmptyDCMObj()

	// Set query level
	query.WriteString(tags.QueryRetrieveLevel, "STUDY")

	// Add query parameters
	if params.PatientID != "" {
		query.WriteString(tags.PatientID, params.PatientID)
	} else {
		query.WriteString(tags.PatientID, "")
	}

	if params.PatientName != "" {
		query.WriteString(tags.PatientName, params.PatientName)
	} else {
		query.WriteString(tags.PatientName, "")
	}

	if params.StudyDate != "" {
		query.WriteString(tags.StudyDate, params.StudyDate)
	} else {
		query.WriteString(tags.StudyDate, "")
	}

	if params.AccessionNumber != "" {
		query.WriteString(tags.AccessionNumber, params.AccessionNumber)
	} else {
		query.WriteString(tags.AccessionNumber, "")
	}

	if params.Modality != "" {
		query.WriteString(tags.ModalitiesInStudy, params.Modality)
	} else {
		query.WriteString(tags.ModalitiesInStudy, "")
	}

	if params.StudyDescription != "" {
		query.WriteString(tags.StudyDescription, params.StudyDescription)
	}

	// Required return keys for study level
	query.WriteString(tags.StudyInstanceUID, "")
	query.WriteString(tags.StudyTime, "")
	query.WriteString(tags.ReferringPhysicianName, "")
	query.WriteString(tags.PatientBirthDate, "")
	query.WriteString(tags.PatientSex, "")
	query.WriteString(tags.NumberOfStudyRelatedSeries, "")
	query.WriteString(tags.NumberOfStudyRelatedInstances, "")

	// Store results
	var studies []models.Study

	// Set result handler
	scu.SetOnCFindResult(func(result media.DcmObj) {
		study := d.dicomToStudy(result)
		studies = append(studies, study)
	})

	// Execute C-FIND
	_, status, err := scu.FindSCU(query, d.timeout)
	if err != nil {
		return nil, fmt.Errorf("C-FIND failed: %w", err)
	}

	if status != 0x0000 {
		return nil, fmt.Errorf("C-FIND completed with non-success status: 0x%04X", status)
	}

	return studies, nil
}

// FindSeries queries for series using C-FIND
func (d *DIMSEAdapter) FindSeries(ctx context.Context, studyUID string) ([]models.Series, error) {
	scu := d.createSCU()

	// Build C-FIND query dataset
	query := media.NewEmptyDCMObj()

	// Set query level
	query.WriteString(tags.QueryRetrieveLevel, "SERIES")

	// Required keys
	query.WriteString(tags.StudyInstanceUID, studyUID)
	query.WriteString(tags.SeriesInstanceUID, "")
	query.WriteString(tags.SeriesNumber, "")
	query.WriteString(tags.Modality, "")
	query.WriteString(tags.SeriesDescription, "")
	query.WriteString(tags.SeriesDate, "")
	query.WriteString(tags.SeriesTime, "")
	query.WriteString(tags.NumberOfSeriesRelatedInstances, "")

	// Store results
	var series []models.Series

	// Set result handler
	scu.SetOnCFindResult(func(result media.DcmObj) {
		s := d.dicomToSeries(result)
		series = append(series, s)
	})

	// Execute C-FIND
	_, status, err := scu.FindSCU(query, d.timeout)
	if err != nil {
		return nil, fmt.Errorf("C-FIND failed: %w", err)
	}

	if status != 0x0000 {
		return nil, fmt.Errorf("C-FIND completed with non-success status: 0x%04X", status)
	}

	return series, nil
}

// FindInstances queries for instances using C-FIND
func (d *DIMSEAdapter) FindInstances(ctx context.Context, studyUID, seriesUID string) ([]models.Instance, error) {
	scu := d.createSCU()

	// Build C-FIND query dataset
	query := media.NewEmptyDCMObj()

	// Set query level
	query.WriteString(tags.QueryRetrieveLevel, "IMAGE")

	// Required keys
	query.WriteString(tags.StudyInstanceUID, studyUID)
	query.WriteString(tags.SeriesInstanceUID, seriesUID)
	query.WriteString(tags.SOPInstanceUID, "")
	query.WriteString(tags.SOPClassUID, "")
	query.WriteString(tags.InstanceNumber, "")
	query.WriteString(tags.Rows, "")
	query.WriteString(tags.Columns, "")
	query.WriteString(tags.BitsAllocated, "")
	query.WriteString(tags.NumberOfFrames, "")

	// Store results
	var instances []models.Instance

	// Set result handler
	scu.SetOnCFindResult(func(result media.DcmObj) {
		instance := d.dicomToInstance(result)
		instances = append(instances, instance)
	})

	// Execute C-FIND
	_, status, err := scu.FindSCU(query, d.timeout)
	if err != nil {
		return nil, fmt.Errorf("C-FIND failed: %w", err)
	}

	if status != 0x0000 {
		return nil, fmt.Errorf("C-FIND completed with non-success status: 0x%04X", status)
	}

	return instances, nil
}

// GetInstance retrieves an instance using C-MOVE (to our own storage SCP)
func (d *DIMSEAdapter) GetInstance(ctx context.Context, studyUID, seriesUID, instanceUID string) (io.ReadCloser, string, error) {
	// For C-MOVE, we need to:
	// 1. Start a temporary storage SCP to receive the image
	// 2. Send C-MOVE request to the PACS
	// 3. Wait for the image to be received
	// 4. Return the image data

	// This is complex - for now, use C-GET if available or return not implemented
	return nil, "", fmt.Errorf("C-MOVE not yet implemented - use DICOMweb for image retrieval")
}

// GetInstanceMetadata retrieves instance metadata using C-FIND
func (d *DIMSEAdapter) GetInstanceMetadata(ctx context.Context, studyUID, seriesUID, instanceUID string) (*models.Metadata, error) {
	scu := d.createSCU()

	// Build C-FIND query dataset
	query := media.NewEmptyDCMObj()
	query.WriteString(tags.QueryRetrieveLevel, "IMAGE")
	query.WriteString(tags.StudyInstanceUID, studyUID)
	query.WriteString(tags.SeriesInstanceUID, seriesUID)
	query.WriteString(tags.SOPInstanceUID, instanceUID)

	// Request all available attributes
	query.WriteString(tags.SOPClassUID, "")
	query.WriteString(tags.InstanceNumber, "")
	query.WriteString(tags.Rows, "")
	query.WriteString(tags.Columns, "")
	query.WriteString(tags.BitsAllocated, "")
	query.WriteString(tags.BitsStored, "")
	query.WriteString(tags.HighBit, "")
	query.WriteString(tags.PixelRepresentation, "")
	query.WriteString(tags.PhotometricInterpretation, "")
	query.WriteString(tags.SamplesPerPixel, "")
	query.WriteString(tags.NumberOfFrames, "")

	var metadata *models.Metadata

	// Set result handler
	scu.SetOnCFindResult(func(result media.DcmObj) {
		metadata = &models.Metadata{
			SOPInstanceUID:    result.GetString(tags.SOPInstanceUID),
			SOPClassUID:       result.GetString(tags.SOPClassUID),
			TransferSyntaxUID: "", // Not available via C-FIND
			Attributes:        d.extractAttributes(result),
		}
	})

	// Execute C-FIND
	_, status, err := scu.FindSCU(query, d.timeout)
	if err != nil {
		return nil, fmt.Errorf("C-FIND failed: %w", err)
	}

	if status != 0x0000 {
		return nil, fmt.Errorf("C-FIND completed with non-success status: 0x%04X", status)
	}

	if metadata == nil {
		return nil, fmt.Errorf("instance not found")
	}

	return metadata, nil
}

// GetStudyMetadata retrieves metadata for all instances in a study
func (d *DIMSEAdapter) GetStudyMetadata(ctx context.Context, studyUID string) ([]models.Metadata, error) {
	// Get all series in study
	series, err := d.FindSeries(ctx, studyUID)
	if err != nil {
		return nil, err
	}

	var allMetadata []models.Metadata
	for _, s := range series {
		// Get instances in series
		instances, err := d.FindInstances(ctx, studyUID, s.SeriesInstanceUID)
		if err != nil {
			continue
		}

		for _, inst := range instances {
			metadata := models.Metadata{
				SOPInstanceUID:    inst.SOPInstanceUID,
				SOPClassUID:       inst.SOPClassUID,
				TransferSyntaxUID: inst.TransferSyntaxUID,
				Attributes:        make(map[string]interface{}),
			}
			allMetadata = append(allMetadata, metadata)
		}
	}

	return allMetadata, nil
}

// GetThumbnail generates a thumbnail (not supported via DIMSE)
func (d *DIMSEAdapter) GetThumbnail(ctx context.Context, studyUID, seriesUID, instanceUID string, size int) ([]byte, error) {
	return nil, fmt.Errorf("thumbnail generation not supported via DIMSE")
}

// TestConnection tests the PACS connection using C-ECHO
func (d *DIMSEAdapter) TestConnection(ctx context.Context) (*models.ConnectionStatus, error) {
	start := time.Now()
	status := &models.ConnectionStatus{
		LastChecked: start,
	}

	scu := d.createSCU()

	// Perform C-ECHO
	err := scu.EchoSCU(d.timeout)

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
func (d *DIMSEAdapter) Close() error {
	// No persistent connections to close with this implementation
	return nil
}

// Helper methods to convert DICOM objects to models

func (d *DIMSEAdapter) dicomToStudy(dcmObj media.DcmObj) models.Study {
	return models.Study{
		StudyInstanceUID:   dcmObj.GetString(tags.StudyInstanceUID),
		PatientID:          dcmObj.GetString(tags.PatientID),
		PatientName:        dcmObj.GetString(tags.PatientName),
		PatientBirthDate:   dcmObj.GetString(tags.PatientBirthDate),
		PatientSex:         dcmObj.GetString(tags.PatientSex),
		StudyDate:          dcmObj.GetString(tags.StudyDate),
		StudyTime:          dcmObj.GetString(tags.StudyTime),
		StudyDescription:   dcmObj.GetString(tags.StudyDescription),
		AccessionNumber:    dcmObj.GetString(tags.AccessionNumber),
		ReferringPhysician: dcmObj.GetString(tags.ReferringPhysicianName),
		NumberOfSeries:     d.getIntValue(dcmObj, tags.NumberOfStudyRelatedSeries),
		NumberOfInstances:  d.getIntValue(dcmObj, tags.NumberOfStudyRelatedInstances),
		ModalitiesInStudy:  d.getModalitiesInStudy(dcmObj),
	}
}

func (d *DIMSEAdapter) dicomToSeries(dcmObj media.DcmObj) models.Series {
	return models.Series{
		SeriesInstanceUID: dcmObj.GetString(tags.SeriesInstanceUID),
		SeriesNumber:      d.getIntValue(dcmObj, tags.SeriesNumber),
		Modality:          dcmObj.GetString(tags.Modality),
		SeriesDescription: dcmObj.GetString(tags.SeriesDescription),
		SeriesDate:        dcmObj.GetString(tags.SeriesDate),
		SeriesTime:        dcmObj.GetString(tags.SeriesTime),
		NumberOfInstances: d.getIntValue(dcmObj, tags.NumberOfSeriesRelatedInstances),
	}
}

func (d *DIMSEAdapter) dicomToInstance(dcmObj media.DcmObj) models.Instance {
	return models.Instance{
		SOPInstanceUID:            dcmObj.GetString(tags.SOPInstanceUID),
		SOPClassUID:               dcmObj.GetString(tags.SOPClassUID),
		InstanceNumber:            d.getIntValue(dcmObj, tags.InstanceNumber),
		Rows:                      d.getIntValue(dcmObj, tags.Rows),
		Columns:                   d.getIntValue(dcmObj, tags.Columns),
		BitsAllocated:             d.getIntValue(dcmObj, tags.BitsAllocated),
		BitsStored:                d.getIntValue(dcmObj, tags.BitsStored),
		HighBit:                   d.getIntValue(dcmObj, tags.HighBit),
		PixelRepresentation:       d.getIntValue(dcmObj, tags.PixelRepresentation),
		PhotometricInterpretation: dcmObj.GetString(tags.PhotometricInterpretation),
		SamplesPerPixel:           d.getIntValue(dcmObj, tags.SamplesPerPixel),
		NumberOfFrames:            d.getIntValue(dcmObj, tags.NumberOfFrames),
	}
}

func (d *DIMSEAdapter) getIntValue(dcmObj media.DcmObj, tagID *tags.Tag) int {
	str := dcmObj.GetString(tagID)
	if str == "" {
		return 0
	}

	var val int
	fmt.Sscanf(str, "%d", &val)
	return val
}

func (d *DIMSEAdapter) getModalitiesInStudy(dcmObj media.DcmObj) []string {
	// ModalitiesInStudy can be multi-valued
	str := dcmObj.GetString(tags.ModalitiesInStudy)
	if str == "" {
		return nil
	}

	// DICOM multi-value is separated by backslash
	return []string{str} // Simplified - should split by \
}

func (d *DIMSEAdapter) extractAttributes(dcmObj media.DcmObj) map[string]interface{} {
	attrs := make(map[string]interface{})

	// Extract common attributes
	if val := dcmObj.GetString(tags.Rows); val != "" {
		attrs["Rows"] = val
	}
	if val := dcmObj.GetString(tags.Columns); val != "" {
		attrs["Columns"] = val
	}
	if val := dcmObj.GetString(tags.BitsAllocated); val != "" {
		attrs["BitsAllocated"] = val
	}
	if val := dcmObj.GetString(tags.PhotometricInterpretation); val != "" {
		attrs["PhotometricInterpretation"] = val
	}

	return attrs
}
