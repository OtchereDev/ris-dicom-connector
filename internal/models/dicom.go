package models

// QueryParams represents DICOM query parameters
type QueryParams struct {
	PatientID        string `json:"patient_id,omitempty"`
	PatientName      string `json:"patient_name,omitempty"`
	StudyDate        string `json:"study_date,omitempty"`
	StudyTime        string `json:"study_time,omitempty"`
	AccessionNumber  string `json:"accession_number,omitempty"`
	Modality         string `json:"modality,omitempty"`
	StudyDescription string `json:"study_description,omitempty"`
	Limit            int    `json:"limit,omitempty"`
	Offset           int    `json:"offset,omitempty"`
}

// Study represents a DICOM study
type Study struct {
	StudyInstanceUID   string   `json:"0020000D" dicom:"0020000D"`
	PatientID          string   `json:"00100020" dicom:"00100020"`
	PatientName        string   `json:"00100010" dicom:"00100010"`
	PatientBirthDate   string   `json:"00100030" dicom:"00100030"`
	PatientSex         string   `json:"00100040" dicom:"00100040"`
	StudyDate          string   `json:"00080020" dicom:"00080020"`
	StudyTime          string   `json:"00080030" dicom:"00080030"`
	StudyDescription   string   `json:"00081030" dicom:"00081030"`
	AccessionNumber    string   `json:"00080050" dicom:"00080050"`
	ReferringPhysician string   `json:"00080090" dicom:"00080090"`
	NumberOfSeries     int      `json:"00201206" dicom:"00201206"`
	NumberOfInstances  int      `json:"00201208" dicom:"00201208"`
	ModalitiesInStudy  []string `json:"00080061" dicom:"00080061"`
	RetrieveURL        string   `json:"00081190,omitempty"`
}

// Series represents a DICOM series
type Series struct {
	SeriesInstanceUID  string `json:"0020000E" dicom:"0020000E"`
	SeriesNumber       int    `json:"00200011" dicom:"00200011"`
	Modality           string `json:"00080060" dicom:"00080060"`
	SeriesDescription  string `json:"0008103E" dicom:"0008103E"`
	SeriesDate         string `json:"00080021" dicom:"00080021"`
	SeriesTime         string `json:"00080031" dicom:"00080031"`
	BodyPartExamined   string `json:"00180015" dicom:"00180015"`
	NumberOfInstances  int    `json:"00201209" dicom:"00201209"`
	ProtocolName       string `json:"00181030" dicom:"00181030"`
	PerformedProcedure string `json:"00400254" dicom:"00400254"`
	RetrieveURL        string `json:"00081190,omitempty"`
}

// Instance represents a DICOM instance
type Instance struct {
	SOPInstanceUID            string `json:"00080018" dicom:"00080018"`
	SOPClassUID               string `json:"00080016" dicom:"00080016"`
	InstanceNumber            int    `json:"00200013" dicom:"00200013"`
	TransferSyntaxUID         string `json:"00020010" dicom:"00020010"`
	Rows                      int    `json:"00280010" dicom:"00280010"`
	Columns                   int    `json:"00280011" dicom:"00280011"`
	BitsAllocated             int    `json:"00280100" dicom:"00280100"`
	BitsStored                int    `json:"00280101" dicom:"00280101"`
	HighBit                   int    `json:"00280102" dicom:"00280102"`
	PixelRepresentation       int    `json:"00280103" dicom:"00280103"`
	PhotometricInterpretation string `json:"00280004" dicom:"00280004"`
	SamplesPerPixel           int    `json:"00280002" dicom:"00280002"`
	NumberOfFrames            int    `json:"00280008" dicom:"00280008"`
	RetrieveURL               string `json:"00081190,omitempty"`
}

// Metadata represents instance metadata
type Metadata struct {
	SOPInstanceUID    string                 `json:"sop_instance_uid"`
	SOPClassUID       string                 `json:"sop_class_uid"`
	TransferSyntaxUID string                 `json:"transfer_syntax_uid"`
	Attributes        map[string]interface{} `json:"attributes"`
}
