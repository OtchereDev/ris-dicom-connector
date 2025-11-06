Excellent! This clears up a lot. Let me revise the implementation strategy based on the SDK code and industry standards.

## Revised Implementation Strategy

### 1. **Remove Custom pkg/dimse Package**

**Rationale:** The SDK's `services.SCU` already provides all DIMSE operations we need:

- `EchoSCU()` - C-ECHO
- `FindSCU()` - C-FIND
- `MoveSCU()` - C-MOVE
- `StoreSCU()` - C-STORE

We should use this directly rather than reimplementing.

### 2. **Industry Standards for DIMSE**

#### **Connection & Timeout Standards**

- **Association timeout:** 30 seconds (configurable, but 30s is standard)
- **Operation timeout:**
  - C-ECHO: 10 seconds
  - C-FIND: 60-120 seconds (can be long for large result sets)
  - C-MOVE: 300 seconds (5 minutes - transfers take time)
  - C-STORE: 60 seconds per instance

#### **AE Title Standards**

- **Format:** 1-16 characters, uppercase alphanumeric + underscore
- **Our calling AET:** `RIS_CONNECTOR` or `SPECTRARIS` (standard, descriptive)
- **Storage SCP AET:** `RIS_STORE_SCP` (for receiving C-MOVE results)
- **PACS called AET:** Retrieved from PACS config (varies by vendor)

#### **Port Standards**

- **DIMSE Port:** 104 (standard) or 11112 (common alternative)
- **Our Storage SCP:** 11113 or dynamic port (avoid conflicts)

#### **Query/Retrieve Levels**

Standard DICOM QR levels:

1. `PATIENT` - Query by patient
2. `STUDY` - Query by study (most common)
3. `SERIES` - Query by series
4. `IMAGE` (or `INSTANCE`) - Query individual instances

#### **SOP Classes to Support**

```
Study Root Query/Retrieve:
- 1.2.840.10008.5.1.4.1.2.2.1 (Study Root QR - FIND)
- 1.2.840.10008.5.1.4.1.2.2.2 (Study Root QR - MOVE)
- 1.2.840.10008.5.1.4.1.2.2.3 (Study Root QR - GET) - optional

Patient Root Query/Retrieve:
- 1.2.840.10008.5.1.4.1.2.1.1 (Patient Root QR - FIND)
- 1.2.840.10008.5.1.4.1.2.1.2 (Patient Root QR - MOVE)
- 1.2.840.10008.5.1.4.1.2.1.3 (Patient Root QR - GET) - optional

Verification:
- 1.2.840.10008.1.1 (C-ECHO)
```

**Industry Practice:** Study Root is more common and should be default. The SDK already uses `sopclass.StudyRootQueryRetrieveInformationModelFind`.

### 3. **Standard DICOM Workflow**

#### **Query Workflow (C-FIND)**

```
1. Build query dataset with:
   - QueryRetrieveLevel (STUDY/SERIES/IMAGE)
   - Matching keys (PatientID, StudyDate, etc.)
   - Return keys (what attributes to get back)
   - Universal matching: empty string matches all

2. Send C-FIND-RQ
3. Receive multiple C-FIND-RSP with status:
   - 0xFF00 (Pending) - more results coming
   - 0xFF01 (Pending with warnings)
   - 0x0000 (Success) - no more results
   - 0xXXXX (Failure) - various error codes

4. Process each result dataset
```

#### **C-MOVE vs C-GET Decision**

**Industry Standard: C-MOVE is the gold standard**

**Why C-MOVE is preferred:**

- Universally supported by all PACS vendors
- More mature and tested
- Better for firewalled environments (PACS initiates connection to us)
- Standard in hospital environments

**C-GET limitations:**

- Not supported by many legacy PACS systems
- Requires bidirectional network access
- Less common in production

**Decision:** Implement C-MOVE as primary method, with automatic fallback to DICOMweb if DIMSE fails entirely.

### 4. **C-MOVE Implementation Architecture**

#### **Components Needed:**

**A. Storage SCP (Receiver)**

```
Purpose: Receive images pushed by PACS via C-STORE
Port: 11113 (configurable)
AE Title: RIS_STORE_SCP
Lifecycle: Long-running service, starts with main application
```

**B. Move Coordinator**

```
Purpose: Track C-MOVE operations and correlate with incoming C-STORE
Pattern:
  1. Send C-MOVE-RQ to PACS
  2. Register operation ID with expected Study/Series/Instance UIDs
  3. Storage SCP receives C-STORE requests
  4. Coordinator matches received instances to pending operations
  5. Stream completed instances to requester
  6. Cleanup when all sub-operations complete
```

**C. Temporary Storage**

```
Purpose: Buffer received DICOM files before streaming to client
Location: Filesystem (not Redis - files too large)
Structure: /tmp/dicom-connector/{operationID}/{sopInstanceUID}.dcm
Cleanup: Delete after streaming or after 1 hour timeout
```

### 5. **Standard DICOM Tags for Query/Retrieve**

#### **Study Level Query (Most Common)**

```
Required:
- (0008,0052) QueryRetrieveLevel = "STUDY"
- (0020,000D) StudyInstanceUID (for retrieve)

Common Matching Keys:
- (0010,0020) PatientID
- (0010,0010) PatientName (with wildcard: "DOE*")
- (0008,0020) StudyDate (range: "20240101-20241231")
- (0008,0050) AccessionNumber
- (0008,0061) ModalitiesInStudy

Return Keys (what to retrieve):
- (0010,0030) PatientBirthDate
- (0010,0040) PatientSex
- (0008,0030) StudyTime
- (0008,1030) StudyDescription
- (0008,0090) ReferringPhysicianName
- (0020,1206) NumberOfStudyRelatedSeries
- (0020,1208) NumberOfStudyRelatedInstances
```

#### **Universal Matching Standard**

- Empty string (`""`) = match all (wildcard)
- `*` = wildcard within string (`"SMITH*"`)
- Date ranges: `"20240101-20241231"`
- Time ranges: `"080000-170000"`

### 6. **Automatic Configuration (No Manual Setup)**

#### **Service Discovery Approach**

```
1. On startup:
   - Start Storage SCP on port 11113 (or find available port)
   - Register our AE Title: RIS_STORE_SCP

2. On PACS config creation:
   - Test connection with C-ECHO
   - Discover capabilities (which SOP classes supported)
   - Store supported operations in database

3. On retrieve request:
   - Check PACS capabilities
   - Use C-MOVE if supported (it will be)
   - Our Storage SCP receives the images
   - Stream to client as they arrive
```

#### **No Configuration Files Needed**

- AE Titles: Hardcoded to standards
- Ports: Auto-discover or use standard defaults
- Storage paths: Environment variable or default to `/tmp`
- Timeouts: Industry standard defaults with env override

### 7. **Refined Implementation Plan**

#### **Phase 2.1: Complete C-FIND (1 week)**

```
1. Simplify dimse_adapter.go to use SDK's SCU directly
2. Implement proper query dataset building using SDK's media.DcmObj
3. Set up callback handlers for results
4. Implement proper tag mapping (dicomToStudy, etc.)
5. Add comprehensive error handling
6. Test against Orthanc
```

#### **Phase 2.2: Implement Storage SCP (1 week)**

```
1. Create internal/services/storage_scp.go
2. Use SDK's SCP capabilities (if available) or minimal implementation
3. Handle C-STORE-RQ from PACS
4. Save to temporary filesystem storage
5. Notify operation coordinator
6. Add tests
```

#### **Phase 2.3: Implement C-MOVE (1 week)**

```
1. Create move coordinator in dimse_adapter.go
2. Implement GetInstance/GetSeries using C-MOVE
3. Coordinate between C-MOVE-RQ and Storage SCP
4. Stream retrieved files to client
5. Implement cleanup and timeout handling
6. End-to-end testing
```

#### **Phase 2.4: Production Hardening (1 week)**

```
1. Connection pooling refinement
2. Concurrent operation handling
3. Comprehensive error scenarios
4. Performance testing
5. Memory leak prevention
6. Graceful degradation to DICOMweb
```

### 8. **Standard Error Handling**

#### **DICOM Status Codes (Industry Standard)**

```
Success:
- 0x0000 - Success

Pending:
- 0xFF00 - Pending (more sub-operations)
- 0xFF01 - Pending with warnings

Cancel:
- 0xFE00 - Cancel

Failures (0xXXXX):
- 0xA700 - Out of resources
- 0xA900 - Data set does not match SOP class
- 0xC000 - Unable to process
- 0xC100 - More than one match (should be unique)
- 0xC200 - Unable to support requested template
```

#### **Retry Strategy**

```
Network errors: Retry 3 times with exponential backoff
Association rejected: Don't retry (configuration issue)
Timeout: Retry once with longer timeout
Status 0xC000: Don't retry (PACS processing error)
Status 0xA700: Retry once after 5 seconds (PACS busy)
```

### 9. **Monitoring & Metrics (Industry Best Practices)**

#### **Key Metrics to Track**

```
- dimse_associations_total (counter)
- dimse_associations_active (gauge)
- dimse_operations_duration_seconds (histogram by operation type)
- dimse_operations_total (counter by operation, status)
- dimse_cmove_suboperations_total (counter)
- dimse_storage_scp_receives_total (counter)
```

### 10. **Configuration Constants**

```go
const (
    // AE Titles
    DefaultCallingAET = "RIS_CONNECTOR"
    StorageSCPAET     = "RIS_STORE_SCP"

    // Ports
    DefaultStorageSCPPort = 11113

    // Timeouts (milliseconds)
    TimeoutCEcho  = 10000
    TimeoutCFind  = 120000
    TimeoutCMove  = 300000
    TimeoutCStore = 60000

    // Connection Pool
    MaxPoolSize     = 5
    MaxIdleTime     = 300000 // 5 minutes

    // Storage
    TempStorageDir = "/tmp/dicom-connector"
    CleanupAfter   = 3600000 // 1 hour

    // Query Retrieve Level
    QRLevelStudy    = "STUDY"
    QRLevelSeries   = "SERIES"
    QRLevelImage    = "IMAGE"
)
```

## Summary of Changes from Original Plan

1. ✅ **Remove pkg/dimse entirely** - Use SDK's services.SCU
2. ✅ **Standard AE Titles** - Hardcoded, no configuration needed
3. ✅ **C-MOVE as primary** - Industry standard, universally supported
4. ✅ **Automatic service discovery** - No manual configuration
5. ✅ **Industry standard timeouts** - Based on real-world PACS behavior
6. ✅ **Study Root QR** - Most common, SDK already uses it
7. ✅ **Storage SCP integrated** - Required for C-MOVE, auto-starts

This approach is cleaner, follows industry standards, and leverages the SDK properly. Should we proceed with implementing Phase 2.1 first?
