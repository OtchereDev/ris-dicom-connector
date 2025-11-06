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
	"github.com/rs/zerolog/log"
)

// DIMSE timeout constants (in seconds) - industry standards
const (
	TimeoutCEcho  = 10  // 10 seconds for C-ECHO
	TimeoutCFind  = 120 // 120 seconds for C-FIND (can return many results)
	TimeoutCMove  = 300 // 300 seconds for C-MOVE (5 minutes - transfers take time)
	TimeoutCStore = 60  // 60 seconds for C-STORE
)

// Standard AE Title for this connector
const CallingAETitle = "RIS_CONNECTOR"

// DIMSEAdapter implements PACSAdapter for DIMSE protocol using the SDK
type DIMSEAdapter struct {
	BaseAdapter
	config      models.PACSConfig
	destination *network.Destination
}

// NewDIMSEAdapter creates a new DIMSE adapter
func NewDIMSEAdapter(config models.PACSConfig) (*DIMSEAdapter, error) {
	// Validate required fields
	if config.AETitle == "" {
		return nil, fmt.Errorf("AE Title (Called AE) is required for DIMSE connection")
	}
	if config.Endpoint == "" {
		return nil, fmt.Errorf("endpoint (hostname) is required for DIMSE connection")
	}
	if config.Port == 0 {
		return nil, fmt.Errorf("port is required for DIMSE connection")
	}

	destination := &network.Destination{
		HostName:  config.Endpoint,
		Port:      config.Port,
		CalledAE:  config.AETitle, // PACS AE Title
		CallingAE: CallingAETitle, // Our AE Title
		IsCFind:   true,           // We support C-FIND
		IsCMove:   false,          // Not yet implemented
		IsCStore:  false,          // Not yet implemented
	}

	log.Info().
		Str("endpoint", config.Endpoint).
		Int("port", config.Port).
		Str("called_ae", config.AETitle).
		Str("calling_ae", CallingAETitle).
		Str("tenant_id", config.TenantID.String()).
		Msg("Created DIMSE adapter")

	return &DIMSEAdapter{
		BaseAdapter: BaseAdapter{config: config},
		config:      config,
		destination: destination,
	}, nil
}

func (d *DIMSEAdapter) Type() models.PACSType {
	return models.PACSTypeDIMSE
}

func (d *DIMSEAdapter) Capabilities() []string {
	return []string{"C-FIND", "C-ECHO"}
}

// TestConnection tests the PACS connection using C-ECHO
func (d *DIMSEAdapter) TestConnection(ctx context.Context) (*models.ConnectionStatus, error) {
	start := time.Now()
	status := &models.ConnectionStatus{
		LastChecked: start,
		IsConnected: false,
	}

	log.Debug().
		Str("endpoint", d.config.Endpoint).
		Int("port", d.config.Port).
		Str("ae_title", d.config.AETitle).
		Msg("Testing DIMSE connection with C-ECHO")

	// Create SCU
	scu := services.NewSCU(d.destination)

	// Perform C-ECHO
	err := scu.EchoSCU(TimeoutCEcho)

	status.ResponseTime = time.Since(start).Milliseconds()

	if err != nil {
		status.IsConnected = false
		status.ErrorMessage = fmt.Sprintf("C-ECHO failed: %v", err)
		log.Warn().
			Err(err).
			Str("endpoint", d.config.Endpoint).
			Int64("response_time_ms", status.ResponseTime).
			Msg("DIMSE C-ECHO failed")
		return status, err
	}

	status.IsConnected = true
	status.Capabilities = d.Capabilities()

	log.Info().
		Str("endpoint", d.config.Endpoint).
		Int64("response_time_ms", status.ResponseTime).
		Msg("DIMSE C-ECHO successful")

	return status, nil
}

// FindStudies queries for studies using C-FIND at STUDY level
func (d *DIMSEAdapter) FindStudies(ctx context.Context, params models.QueryParams) ([]models.Study, error) {
	log.Debug().
		Interface("params", params).
		Str("endpoint", d.config.Endpoint).
		Msg("Executing C-FIND for studies")

	// Create SCU
	scu := services.NewSCU(d.destination)

	// Build query dataset
	query := media.NewEmptyDCMObj()

	// Set query level
	query.WriteString(tags.QueryRetrieveLevel, "STUDY")

	// Add matching keys (empty string = match all, per DICOM standard)
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
	start := time.Now()
	numResults, status, err := scu.FindSCU(query, TimeoutCFind)
	duration := time.Since(start)

	if err != nil {
		log.Error().
			Err(err).
			Str("endpoint", d.config.Endpoint).
			Dur("duration", duration).
			Msg("C-FIND for studies failed")
		return nil, fmt.Errorf("C-FIND failed: %w", err)
	}

	// Status 0x0000 = Success
	if status != 0x0000 {
		log.Warn().
			Uint16("status", status).
			Str("endpoint", d.config.Endpoint).
			Msg("C-FIND completed with non-success status")
		return nil, fmt.Errorf("C-FIND completed with status: 0x%04X", status)
	}

	log.Info().
		Int("num_results", numResults).
		Int("num_studies", len(studies)).
		Dur("duration", duration).
		Str("endpoint", d.config.Endpoint).
		Msg("C-FIND for studies completed successfully")

	return studies, nil
}

// FindSeries queries for series using C-FIND at SERIES level
func (d *DIMSEAdapter) FindSeries(ctx context.Context, studyUID string) ([]models.Series, error) {
	log.Debug().
		Str("study_uid", studyUID).
		Str("endpoint", d.config.Endpoint).
		Msg("Executing C-FIND for series")

	// Create SCU
	scu := services.NewSCU(d.destination)

	// Build query dataset
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
	start := time.Now()
	numResults, status, err := scu.FindSCU(query, TimeoutCFind)
	duration := time.Since(start)

	if err != nil {
		log.Error().
			Err(err).
			Str("study_uid", studyUID).
			Str("endpoint", d.config.Endpoint).
			Dur("duration", duration).
			Msg("C-FIND for series failed")
		return nil, fmt.Errorf("C-FIND failed: %w", err)
	}

	if status != 0x0000 {
		log.Warn().
			Uint16("status", status).
			Str("study_uid", studyUID).
			Msg("C-FIND completed with non-success status")
		return nil, fmt.Errorf("C-FIND completed with status: 0x%04X", status)
	}

	log.Info().
		Int("num_results", numResults).
		Int("num_series", len(series)).
		Str("study_uid", studyUID).
		Dur("duration", duration).
		Msg("C-FIND for series completed successfully")

	return series, nil
}

// FindInstances queries for instances using C-FIND at IMAGE level
func (d *DIMSEAdapter) FindInstances(ctx context.Context, studyUID, seriesUID string) ([]models.Instance, error) {
	log.Debug().
		Str("study_uid", studyUID).
		Str("series_uid", seriesUID).
		Str("endpoint", d.config.Endpoint).
		Msg("Executing C-FIND for instances")

	// Create SCU
	scu := services.NewSCU(d.destination)

	// Build query dataset
	query := media.NewEmptyDCMObj()

	// Set query level (IMAGE is the DICOM standard, some PACS use INSTANCE)
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
	start := time.Now()
	numResults, status, err := scu.FindSCU(query, TimeoutCFind)
	duration := time.Since(start)

	if err != nil {
		log.Error().
			Err(err).
			Str("study_uid", studyUID).
			Str("series_uid", seriesUID).
			Dur("duration", duration).
			Msg("C-FIND for instances failed")
		return nil, fmt.Errorf("C-FIND failed: %w", err)
	}

	if status != 0x0000 {
		log.Warn().
			Uint16("status", status).
			Str("study_uid", studyUID).
			Str("series_uid", seriesUID).
			Msg("C-FIND completed with non-success status")
		return nil, fmt.Errorf("C-FIND completed with status: 0x%04X", status)
	}

	log.Info().
		Int("num_results", numResults).
		Int("num_instances", len(instances)).
		Str("study_uid", studyUID).
		Str("series_uid", seriesUID).
		Dur("duration", duration).
		Msg("C-FIND for instances completed successfully")

	return instances, nil
}

// GetInstance retrieves an instance (NOT IMPLEMENTED - Phase 2B)
func (d *DIMSEAdapter) GetInstance(ctx context.Context, studyUID, seriesUID, instanceUID string) (io.ReadCloser, string, error) {
	log.Warn().
		Str("study_uid", studyUID).
		Str("series_uid", seriesUID).
		Str("instance_uid", instanceUID).
		Msg("C-MOVE not implemented - use DICOMweb for image retrieval")

	return nil, "", fmt.Errorf("image retrieval via C-MOVE not yet implemented - use DICOMweb adapter for image retrieval")
}

// GetInstanceMetadata retrieves instance metadata using C-FIND
func (d *DIMSEAdapter) GetInstanceMetadata(ctx context.Context, studyUID, seriesUID, instanceUID string) (*models.Metadata, error) {
	log.Debug().
		Str("study_uid", studyUID).
		Str("series_uid", seriesUID).
		Str("instance_uid", instanceUID).
		Msg("Getting instance metadata via C-FIND")

	// Create SCU
	scu := services.NewSCU(d.destination)

	// Build query dataset
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
	_, status, err := scu.FindSCU(query, TimeoutCFind)
	if err != nil {
		return nil, fmt.Errorf("C-FIND failed: %w", err)
	}

	if status != 0x0000 {
		return nil, fmt.Errorf("C-FIND completed with status: 0x%04X", status)
	}

	if metadata == nil {
		return nil, fmt.Errorf("instance not found")
	}

	return metadata, nil
}

// GetStudyMetadata retrieves metadata for all instances in a study
func (d *DIMSEAdapter) GetStudyMetadata(ctx context.Context, studyUID string) ([]models.Metadata, error) {
	log.Debug().
		Str("study_uid", studyUID).
		Msg("Getting study metadata via C-FIND")

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
			log.Warn().
				Err(err).
				Str("study_uid", studyUID).
				Str("series_uid", s.SeriesInstanceUID).
				Msg("Failed to get instances for series, skipping")
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

	log.Info().
		Int("num_metadata", len(allMetadata)).
		Str("study_uid", studyUID).
		Msg("Retrieved study metadata")

	return allMetadata, nil
}

// GetThumbnail generates a thumbnail (not supported via DIMSE)
func (d *DIMSEAdapter) GetThumbnail(ctx context.Context, studyUID, seriesUID, instanceUID string, size int) ([]byte, error) {
	return nil, fmt.Errorf("thumbnail generation not supported via DIMSE protocol")
}

// Close closes the adapter (no persistent connections with this implementation)
func (d *DIMSEAdapter) Close() error {
	log.Debug().
		Str("endpoint", d.config.Endpoint).
		Msg("Closing DIMSE adapter (no persistent connections)")
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
		TransferSyntaxUID:         "", // Not available from C-FIND
	}
}

func (d *DIMSEAdapter) getIntValue(dcmObj media.DcmObj, tag *tags.Tag) int {
	str := dcmObj.GetString(tag)
	if str == "" {
		return 0
	}

	var val int
	_, err := fmt.Sscanf(str, "%d", &val)
	if err != nil {
		return 0
	}
	return val
}

func (d *DIMSEAdapter) getModalitiesInStudy(dcmObj media.DcmObj) []string {
	// ModalitiesInStudy can be multi-valued (separated by backslash)
	str := dcmObj.GetString(tags.ModalitiesInStudy)
	if str == "" {
		return nil
	}

	// Split by backslash (DICOM multi-value separator)
	var modalities []string
	current := ""
	for _, char := range str {
		if char == '\\' {
			if current != "" {
				modalities = append(modalities, current)
				current = ""
			}
		} else {
			current += string(char)
		}
	}
	if current != "" {
		modalities = append(modalities, current)
	}

	return modalities
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
	if val := dcmObj.GetString(tags.BitsStored); val != "" {
		attrs["BitsStored"] = val
	}
	if val := dcmObj.GetString(tags.PhotometricInterpretation); val != "" {
		attrs["PhotometricInterpretation"] = val
	}
	if val := dcmObj.GetString(tags.SamplesPerPixel); val != "" {
		attrs["SamplesPerPixel"] = val
	}
	if val := dcmObj.GetString(tags.NumberOfFrames); val != "" {
		attrs["NumberOfFrames"] = val
	}

	return attrs
}
