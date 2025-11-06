# DICOM Connector

Multi-tenant DICOM abstraction layer for SpectraRIS.

## Quick Start

1. **Install dependencies:**

```bash
   make install-deps
```

2. **Start infrastructure (PostgreSQL, Redis, Orthanc):**

```bash
   make docker-up
```

3. **Run the server:**

```bash
   make run
```

4. **Test the health endpoint:**

```bash
   curl http://localhost:8080/health
```

## Configuration

Copy `.env.example` to `.env` and adjust as needed:

```bash
cp .env.example .env
```

## API Endpoints

### Health

- `GET /health` - Health check
- `GET /ready` - Readiness check
- `GET /metrics` - Prometheus metrics

### DICOMweb (requires `X-Tenant-ID` header)

- `GET /dicom-web/studies` - Search studies (QIDO-RS)
- `GET /dicom-web/studies/{studyUID}/series` - Search series
- `GET /dicom-web/studies/{studyUID}/series/{seriesUID}/instances` - Search instances
- `GET /dicom-web/studies/{studyUID}/metadata` - Get study metadata
- `GET /dicom-web/studies/{studyUID}/series/{seriesUID}/instances/{instanceUID}` - Retrieve instance

### Management (requires `X-Tenant-ID` header)

- `POST /api/v1/pacs/config` - Create PACS configuration
- `GET /api/v1/pacs/config` - List PACS configurations
- `GET /api/v1/pacs/config/{id}` - Get PACS configuration
- `POST /api/v1/pacs/test` - Test PACS connection

## Testing with Orthanc

1. Access Orthanc web UI: http://localhost:8042
2. Upload some DICOM files
3. Create a PACS config pointing to Orthanc:

```bash
   curl -X POST http://localhost:8080/api/v1/pacs/config \
     -H "Content-Type: application/json" \
     -H "X-Tenant-ID: 00000000-0000-0000-0000-000000000001" \
     -d '{
       "name": "Orthanc Test",
       "type": "dicomweb",
       "endpoint": "localhost",
       "port": 8042,
       "username": "orthanc",
       "password": "orthanc",
       "is_primary": true
     }'
```

4. Query studies:

```bash
   curl http://localhost:8080/dicom-web/studies \
     -H "X-Tenant-ID: 00000000-0000-0000-0000-000000000001"
```

## Development

- `make build` - Build binary
- `make test` - Run tests
- `make lint` - Run linter
- `make clean` - Clean artifacts

## License

Proprietary - DeltaFlare Ltd

```

**`.gitignore`:**
```

# Binaries

bin/
_.exe
_.dll
_.so
_.dylib

# Test and coverage

_.test
_.out
coverage.out

# Go

vendor/
go.work

# Environment

.env
.env.local

# IDE

.vscode/
.idea/
_.swp
_.swo
\*~

# OS

.DS_Store
Thumbs.db

# Logs

\*.log
logs/

# Database

_.db
_.sqlite
