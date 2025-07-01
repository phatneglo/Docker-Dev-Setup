# ONLYOFFICE with MinIO Integration

Complete setup for ONLYOFFICE Document Server with MinIO storage and FastAPI webhook integration.

## 🚀 Quick Start

### 1. Start MinIO Archive
```bash
cd ../minio-archive
docker-compose --env-file ../minio-archive.env up -d
```

### 2. Start ONLYOFFICE Document Server
```bash
cd only_office
.\start-onlyoffice-8080.ps1
```

### 3. Start FastAPI API Server
```bash
cd api-server
.\start-api.bat
```

## 📋 Services Overview

| Service | Port | URL | Purpose |
|---------|------|-----|---------|
| **ONLYOFFICE** | 8080 | http://localhost:8080 | Document editing server |
| **FastAPI** | 3000 | http://localhost:3000 | Webhook & file management API |
| **MinIO** | 9010/9011 | http://localhost:9010 | S3-compatible storage |

## 🔧 Configuration

### Environment Variables (`only_office.env`)
```env
# ONLYOFFICE Configuration
JWT_ENABLED=true
JWT_SECRET=0yq5h3to9
ONLYOFFICE_SERVER_URL=http://localhost:8080

# FastAPI Webhook Server
WEBHOOK_PORT=3000
WEBHOOK_URL=http://localhost:3000

# MinIO/S3 Configuration (Digital Ocean Spaces)
MINIO_ENDPOINT=sgp1.digitaloceanspaces.com
MINIO_ACCESS_KEY=your_access_key
MINIO_SECRET_KEY=your_secret_key
MINIO_BUCKET=your_bucket_name

# Temporary File Management
TEMP_DIR=temp_files
TEMP_FILE_TTL_HOURS=24
```

## 🌐 API Endpoints

### FastAPI Server (port 3000)

#### Document Management
- `GET /` - Server info and status page
- `GET /health` - Health check for all services
- `GET /docs` - Interactive API documentation
- `GET /documents` - List all documents in MinIO

#### File Operations
- `POST /upload` - Upload file to MinIO
- `GET /download/{filename}` - Download file from MinIO
- `GET /editor/{filename}` - Get document editor configuration

#### ONLYOFFICE Integration
- `POST /webhook/callback` - Receive ONLYOFFICE document save callbacks

### Usage Examples

#### Upload a file
```bash
curl -X POST "http://localhost:3000/upload" \
  -H "Content-Type: multipart/form-data" \
  -F "file=@document.docx"
```

#### List documents in S3
```bash
curl http://localhost:3000/documents
```

#### List temporary files
```bash
curl http://localhost:3000/temp-files
```

#### Clean up old temp files
```bash
curl -X POST http://localhost:3000/cleanup-temp-files
```

#### Delete specific temp file
```bash
curl -X DELETE http://localhost:3000/temp-files/document.docx
```

#### Health check
```bash
curl http://localhost:3000/health
```

## 📱 Document Editing Workflow

1. **Upload Document** → FastAPI uploads to S3 (Digital Ocean Spaces)
2. **Access Document** → FastAPI downloads from S3 to temporary storage
3. **Open Editor** → ONLYOFFICE loads document from temporary storage
4. **Edit Document** → Real-time collaborative editing
5. **Save Document** → ONLYOFFICE sends callback to FastAPI
6. **Apply Revision** → FastAPI saves changes back to **original S3 location**

### ✨ Enhanced Workflow Benefits

- **Better Performance**: Files cached locally for faster access
- **Proper Revisions**: Changes saved back to original file location
- **Version Control**: Edits update the original file instead of creating copies
- **Offline Editing**: Documents available in temp storage during network issues
- **File Processing**: Ability to process files before/after editing
- **Cost Optimization**: Reduced direct S3 API calls

### 🔄 Revision Handling

The system now tracks the original S3 path of each file and ensures that:
- Edited documents are saved back to their **original location**
- Changes appear as revisions of the original file
- No duplicate copies are created in separate folders
- File history and versioning work correctly

## 🔒 Security Features

- **JWT Authentication** enabled for ONLYOFFICE
- **CORS middleware** configured for cross-origin requests
- **File validation** and unique naming
- **Error handling** and logging

## 🗂️ Storage Structure

### S3 Bucket Organization:
```
your-bucket/
├── uploads/          # User uploaded files
│   ├── uuid1.docx
│   └── uuid2.xlsx
└── documents/        # ONLYOFFICE saved documents
    ├── document1.docx
    └── document2.xlsx
```

### Local Temporary Storage:
```
temp_files/           # Local temporary file cache
├── document1.docx    # Downloaded from S3 for editing
├── document2.xlsx    # Cached for faster access
└── document3.pptx    # Recently edited files
```

### Temporary File Management:
- Files are automatically downloaded from S3 when accessed
- Local copies are cached for improved performance
- Old files are cleaned up after TTL expires (default: 24 hours)
- Manual cleanup and management via API endpoints

## 🛠️ Development

### Project Structure
```
only_office/
├── api-server/          # FastAPI application
│   ├── main.py         # Main application
│   ├── requirements.txt # Python dependencies
│   └── start-api.bat   # Startup script
├── data/               # ONLYOFFICE data persistence
├── start-onlyoffice-8080.ps1  # ONLYOFFICE startup
└── README.md
```

### Running in Development Mode
1. Start services in order: MinIO → ONLYOFFICE → FastAPI
2. Access http://localhost:3000 for API server
3. Access http://localhost:8080 for ONLYOFFICE
4. Use http://localhost:3000/docs for API testing

## 🚨 Troubleshooting

### Common Issues

1. **Port conflicts**
   - ONLYOFFICE: Use port 8080 instead of 80
   - Check what's using ports: `netstat -ano | findstr :PORT`

2. **MinIO connection issues**
   - Ensure MinIO is running: `docker ps | findstr minio`
   - Check MinIO console: http://localhost:9011

3. **ONLYOFFICE not starting**
   - Check Docker logs: `docker logs onlyoffice-documentserver`
   - Verify JWT secret consistency

4. **FastAPI errors**
   - Check Python version (3.8+ required)
   - Install dependencies: `pip install -r requirements.txt`
   - Check logs in terminal

### Health Checks

- **ONLYOFFICE**: http://localhost:8080/healthcheck
- **FastAPI**: http://localhost:3000/health
- **MinIO**: http://localhost:9010/minio/health/live

## 📊 Monitoring

The FastAPI server provides comprehensive health monitoring:
- MinIO connection status
- ONLYOFFICE server status
- Bucket existence verification
- Service response times

## 🔄 Updates

To update ONLYOFFICE:
```bash
docker pull onlyoffice/documentserver:latest
docker stop onlyoffice-documentserver
docker rm onlyoffice-documentserver
.\start-onlyoffice-8080.ps1
```

## 📞 Support

- ONLYOFFICE Documentation: https://helpcenter.onlyoffice.com/
- MinIO Documentation: https://docs.min.io/
- FastAPI Documentation: https://fastapi.tiangolo.com/ 