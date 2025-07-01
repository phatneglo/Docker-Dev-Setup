"""
ONLYOFFICE FastAPI Server with MinIO Integration
Handles document callbacks, file storage, and provides APIs for document management
Now includes temporary file management for better editing control
"""

import os
import io
import uuid
import asyncio
import logging
import jwt
import json
import tempfile
import shutil
import base64
import hashlib
from datetime import datetime, timedelta
from typing import Optional, Dict, Any, List
from pathlib import Path

import uvicorn
import httpx
import aiofiles
from fastapi import FastAPI, HTTPException, UploadFile, File, Form, Request, BackgroundTasks
from fastapi.responses import JSONResponse, StreamingResponse, HTMLResponse, FileResponse
from fastapi.middleware.cors import CORSMiddleware
from fastapi.staticfiles import StaticFiles
from pydantic import BaseModel, Field
from pydantic_settings import BaseSettings
from minio import Minio
from minio.error import S3Error

# Configure logging
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

class Settings(BaseSettings):
    # ONLYOFFICE Configuration
    jwt_secret: str = "0yq5h3to9"
    jwt_enabled: bool = True
    onlyoffice_server_url: str = "http://localhost:8080"
    
    # MinIO/S3 Configuration (Digital Ocean Spaces)
    minio_endpoint: str = "sgp1.digitaloceanspaces.com"
    minio_access_key: str = "DO00WVEWRUJZDU2XGJRB"
    minio_secret_key: str = "7kAG4m6BfRE07mnnZygCwQSTqn+hpKmK0o9zVGT0D+4"
    minio_bucket: str = "pg-itbs-dev"
    minio_secure: bool = True
    
    # Server Configuration
    webhook_port: int = 3000
    webhook_host: str = "0.0.0.0"
    host_ip: str = "172.30.160.1"  # Host IP for Docker containers to access
    log_level: str = "INFO"
    environment: str = "development"
    
    # Temporary file management
    temp_dir: str = "temp_files"
    temp_file_ttl_hours: int = 24  # Files older than this will be cleaned up
    
    class Config:
        env_file = "../only_office.env"

settings = Settings()

# Initialize FastAPI app
app = FastAPI(
    title="ONLYOFFICE MinIO API Server",
    description="FastAPI server for ONLYOFFICE document management with MinIO storage and temporary file caching",
    version="1.0.0"
)

# Add CORS middleware
app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

# Initialize MinIO client
minio_client = Minio(
    settings.minio_endpoint,
    access_key=settings.minio_access_key,
    secret_key=settings.minio_secret_key,
    secure=settings.minio_secure
)

# Create temp directory
TEMP_DIR = Path(settings.temp_dir)
TEMP_DIR.mkdir(exist_ok=True)

# Pydantic models
class DocumentCallback(BaseModel):
    """ONLYOFFICE document callback model"""
    key: str
    status: int
    url: Optional[str] = None
    changesurl: Optional[str] = None
    history: Optional[Dict] = None
    users: Optional[List[str]] = None
    actions: Optional[List[Dict]] = None
    lastsave: Optional[str] = None
    notmodified: Optional[bool] = None

class DocumentInfo(BaseModel):
    """Document information model"""
    filename: str
    key: str
    url: str
    size: Optional[int] = None
    created_at: datetime
    modified_at: datetime

class UploadResponse(BaseModel):
    """File upload response model"""
    filename: str
    key: str
    url: str
    size: int
    bucket: str

# Utility functions
async def ensure_bucket_exists():
    """Ensure MinIO bucket exists"""
    try:
        if not minio_client.bucket_exists(settings.minio_bucket):
            minio_client.make_bucket(settings.minio_bucket)
            logger.info(f"Created bucket: {settings.minio_bucket}")
        return True
    except S3Error as e:
        logger.error(f"Error creating bucket: {e}")
        return False

async def cleanup_old_temp_files():
    """Clean up temporary files older than TTL"""
    try:
        cutoff_time = datetime.now() - timedelta(hours=settings.temp_file_ttl_hours)
        for temp_file in TEMP_DIR.glob("*"):
            if temp_file.is_file():
                file_mtime = datetime.fromtimestamp(temp_file.stat().st_mtime)
                if file_mtime < cutoff_time:
                    temp_file.unlink()
                    logger.info(f"Cleaned up old temp file: {temp_file.name}")
    except Exception as e:
        logger.error(f"Error cleaning up temp files: {e}")

async def download_s3_file_to_temp(filename: str) -> Optional[Path]:
    """Download file from S3 to temporary storage and return the local path"""
    try:
        # Check if file already exists in temp storage
        temp_file_path = TEMP_DIR / filename
        if temp_file_path.exists():
            logger.info(f"File {filename} already exists in temp storage")
            return temp_file_path
        
        # Try different S3 paths where the file might be stored
        possible_paths = [f"uploads/{filename}", f"{filename}", f"documents/{filename}"]
        s3_object_path = None
        
        for path in possible_paths:
            try:
                # Check if object exists at this path
                minio_client.stat_object(settings.minio_bucket, path)
                s3_object_path = path
                logger.info(f"Found file at S3 path: {path}")
                break
            except S3Error as e:
                if e.code == "NoSuchKey":
                    continue
                else:
                    logger.error(f"S3 Error for path {path}: {e}")
                    raise e
        
        if not s3_object_path:
            logger.error(f"File {filename} not found in S3 bucket {settings.minio_bucket}")
            return None
        
        # Download file from S3 to temp storage
        logger.info(f"Downloading {filename} from S3 to temp storage...")
        response = minio_client.get_object(settings.minio_bucket, s3_object_path)
        
        # Write to temp file
        with open(temp_file_path, 'wb') as f:
            for chunk in response:
                f.write(chunk)
        
        response.close()
        response.release_conn()
        
        logger.info(f"Successfully downloaded {filename} to temp storage: {temp_file_path}")
        return temp_file_path
        
    except Exception as e:
        logger.error(f"Error downloading {filename} to temp storage: {e}")
        return None

async def save_temp_file_to_s3(temp_file_path: Path, s3_path: str) -> bool:
    """Save temporary file back to S3 at specified path"""
    try:
        if not temp_file_path.exists():
            logger.error(f"Temp file does not exist: {temp_file_path}")
            return False
        
        with open(temp_file_path, 'rb') as file_data:
            file_size = temp_file_path.stat().st_size
            minio_client.put_object(
                settings.minio_bucket,
                s3_path,
                file_data,
                file_size
            )
        
        logger.info(f"Successfully saved {temp_file_path.name} to S3 as {s3_path}")
        return True
        
    except Exception as e:
        logger.error(f"Error saving temp file to S3: {e}")
        return False

async def find_original_s3_path(filename: str) -> Optional[str]:
    """Find the original S3 path for a filename"""
    possible_paths = [f"uploads/{filename}", f"{filename}", f"documents/{filename}"]
    
    for path in possible_paths:
        try:
            minio_client.stat_object(settings.minio_bucket, path)
            logger.info(f"Found original file at S3 path: {path}")
            return path
        except S3Error as e:
            if e.code == "NoSuchKey":
                continue
            else:
                logger.error(f"S3 Error for path {path}: {e}")
                raise e
    
    logger.error(f"File {filename} not found in any S3 location")
    return None

async def download_file_from_url(url: str) -> bytes:
    """Download file from URL"""
    async with httpx.AsyncClient() as client:
        response = await client.get(url)
        response.raise_for_status()
        return response.content

def generate_document_key() -> str:
    """Generate unique document key"""
    return str(uuid.uuid4())

def get_file_extension(filename: str) -> str:
    """Get file extension"""
    return Path(filename).suffix.lower()

def generate_jwt_token(payload: dict) -> str:
    """Generate JWT token for ONLYOFFICE configuration"""
    if not settings.jwt_enabled:
        return ""
    
    try:
        token = jwt.encode(payload, settings.jwt_secret, algorithm="HS256")
        logger.info(f"Generated JWT token for payload keys: {list(payload.keys())}")
        return token
    except Exception as e:
        logger.error(f"Error generating JWT token: {e}")
        raise HTTPException(status_code=500, detail=f"Failed to generate JWT token: {str(e)}")

# API Routes
@app.on_event("startup")
async def startup_event():
    """Initialize services on startup"""
    logger.info("Starting ONLYOFFICE MinIO API Server with temp file management...")
    await ensure_bucket_exists()
    await cleanup_old_temp_files()
    logger.info(f"Temporary files directory: {TEMP_DIR.absolute()}")
    logger.info(f"Server running on {settings.webhook_host}:{settings.webhook_port}")

@app.get("/", response_class=HTMLResponse)
async def root():
    """Root endpoint with API information"""
    html_content = f"""
    <!DOCTYPE html>
    <html>
    <head>
        <title>ONLYOFFICE MinIO API Server</title>
        <style>
            body {{ font-family: Arial, sans-serif; margin: 40px; background-color: #f5f5f5; }}
            .container {{ background: white; padding: 20px; border-radius: 8px; box-shadow: 0 2px 10px rgba(0,0,0,0.1); }}
            h1 {{ color: #333; }}
            .status {{ padding: 10px; margin: 10px 0; border-radius: 4px; }}
            .success {{ background-color: #d4edda; color: #155724; border: 1px solid #c3e6cb; }}
            .info {{ background-color: #cce7ff; color: #004085; border: 1px solid #99d6ff; }}
            a {{ color: #007bff; text-decoration: none; }}
            a:hover {{ text-decoration: underline; }}
            .endpoint {{ background-color: #f8f9fa; padding: 10px; margin: 5px 0; border-radius: 4px; }}
        </style>
    </head>
    <body>
        <div class="container">
            <h1>🏢 ONLYOFFICE MinIO API Server</h1>
            
            <div class="status success">
                ✅ Server is running successfully!
            </div>
            
            <h2>📋 Configuration</h2>
            <div class="info">
                <strong>ONLYOFFICE Server:</strong> {settings.onlyoffice_server_url}<br>
                <strong>MinIO Endpoint:</strong> {settings.minio_endpoint}<br>
                <strong>Storage Bucket:</strong> {settings.minio_bucket}<br>
                <strong>Environment:</strong> {settings.environment}
            </div>
            
            <h2>🔗 API Endpoints</h2>
            <div class="endpoint"><strong>GET</strong> <a href="/docs">/docs</a> - Interactive API Documentation</div>
            <div class="endpoint"><strong>GET</strong> <a href="/health">/health</a> - Health Check</div>
            <div class="endpoint"><strong>POST</strong> /webhook/callback - ONLYOFFICE Document Callback</div>
            <div class="endpoint"><strong>POST</strong> /upload - Upload File to S3</div>
            <div class="endpoint"><strong>GET</strong> /download/{{filename}} - Download File from Temp Storage</div>
            <div class="endpoint"><strong>GET</strong> /documents - List Documents in S3</div>
            <div class="endpoint"><strong>GET</strong> /temp-files - List Temporary Files</div>
            <div class="endpoint"><strong>POST</strong> /cleanup-temp-files - Clean Up Old Temp Files</div>
            <div class="endpoint"><strong>DELETE</strong> /temp-files/{{filename}} - Delete Specific Temp File</div>
            <div class="endpoint"><strong>GET</strong> /editor/{{filename}} - Document Editor</div>
            
            <h2>🧪 Test Links</h2>
            <p><a href="/health">Health Check</a></p>
            <p><a href="/documents">List Documents in S3</a></p>
            <p><a href="/temp-files">List Temporary Files</a></p>
            <p><a href="/docs">API Documentation</a></p>
            
            <h2>👥 Real-time Collaboration Testing</h2>
            <p>To test collaboration, open the same document with different users:</p>
            <div class="endpoint">
                <strong>User 1:</strong> <code>/editor/document.docx?user_id=1&username=Alice</code><br>
                <strong>User 2:</strong> <code>/editor/document.docx?user_id=2&username=Bob</code>
            </div>
            <p><small>Replace "document.docx" with your actual filename. Both users will see each other's changes in real-time!</small></p>
            
            <h2>📱 ONLYOFFICE Integration</h2>
            <p>Webhook URL for ONLYOFFICE: <code>http://localhost:{settings.webhook_port}/webhook/callback</code></p>
            <p>Access ONLYOFFICE: <a href="{settings.onlyoffice_server_url}" target="_blank">{settings.onlyoffice_server_url}</a></p>
        </div>
    </body>
    </html>
    """
    return HTMLResponse(content=html_content)

@app.get("/health")
async def health_check():
    """Health check endpoint"""
    try:
        # Test MinIO connection
        bucket_exists = minio_client.bucket_exists(settings.minio_bucket)
        
        # Test ONLYOFFICE connection
        onlyoffice_status = "unknown"
        try:
            async with httpx.AsyncClient() as client:
                response = await client.get(f"{settings.onlyoffice_server_url}/healthcheck", timeout=5)
                onlyoffice_status = "healthy" if response.status_code == 200 else "unhealthy"
        except:
            onlyoffice_status = "unreachable"
        
        return {
            "status": "healthy",
            "timestamp": datetime.now().isoformat(),
            "services": {
                "minio": "healthy" if bucket_exists else "unhealthy",
                "onlyoffice": onlyoffice_status
            },
            "config": {
                "bucket": settings.minio_bucket,
                "onlyoffice_url": settings.onlyoffice_server_url
            }
        }
    except Exception as e:
        logger.error(f"Health check failed: {e}")
        raise HTTPException(status_code=503, detail=f"Service unhealthy: {str(e)}")

@app.post("/webhook/callback")
async def onlyoffice_callback(callback: DocumentCallback, background_tasks: BackgroundTasks):
    """Handle ONLYOFFICE document callbacks"""
    logger.info(f"Received callback for document {callback.key}, status: {callback.status}")
    
    try:
        # Status meanings:
        # 0 - document not found
        # 1 - document editing
        # 2 - document ready for saving
        # 3 - document save error
        # 4 - document closed without changes
        # 6 - document being edited, but current document state is saved
        # 7 - force save request error
        
        if callback.status == 2 or callback.status == 6:  # Document ready for saving or force save
            if callback.url:
                # Download and save document to MinIO
                background_tasks.add_task(save_document_to_minio, callback.key, callback.url)
                logger.info(f"Queued document {callback.key} for saving to MinIO")
        
        # Always return success to ONLYOFFICE
        return {"error": 0}
        
    except Exception as e:
        logger.error(f"Callback processing error: {e}")
        return {"error": 1, "message": str(e)}

async def save_document_to_minio(document_key: str, download_url: str):
    """
    Save document from ONLYOFFICE to temporary storage and then back to original S3 location.
    
    This ensures proper revision handling:
    - Files are saved back to their original S3 path (uploads/, documents/, or root)
    - Changes appear as updates to the original file, not as separate copies
    - Maintains proper file versioning and history
    
    Document key format: doc_{uuid}_{base64_s3_path}_{filename}
    """
    try:
        logger.info(f"Downloading document {document_key} from {download_url}")
        
        # Download document content
        file_content = await download_file_from_url(download_url)
        
        # Extract filename and original S3 path from document key 
        # Format: doc_<uuid>_<base64_s3_path>_<filename>
        if document_key.startswith("doc_") and document_key.count("_") >= 3:
            parts = document_key.split("_", 3)  # Split into max 4 parts: ['doc', uuid, s3_path, filename]
            if len(parts) >= 4:
                encoded_s3_path = parts[2]
                filename = parts[3]
                
                # Decode the S3 path from base64
                try:
                    original_s3_path = base64.b64decode(encoded_s3_path.encode()).decode()
                except:
                    # Fallback: find the original path
                    original_s3_path = await find_original_s3_path(filename)
            else:
                # Fallback for old format
                filename = document_key.split("_", 2)[-1] if "_" in document_key else f"document_{document_key}.docx"
                original_s3_path = await find_original_s3_path(filename)
        else:
            # Fallback for old format
            filename = document_key.split("_", 2)[-1] if "_" in document_key else f"document_{document_key}.docx"
            original_s3_path = await find_original_s3_path(filename)
        
        if not original_s3_path:
            logger.error(f"Could not determine original S3 path for {filename}")
            return
        
        # Save to temporary storage first
        temp_file_path = TEMP_DIR / filename
        with open(temp_file_path, 'wb') as f:
            f.write(file_content)
        
        logger.info(f"Document saved to temp storage: {temp_file_path}")
        
        # Save back to original S3 location (this creates a revision of the original file)
        success = await save_temp_file_to_s3(temp_file_path, original_s3_path)
        
        if success:
            logger.info(f"Document successfully saved back to original S3 location: {original_s3_path}")
        else:
            logger.error(f"Failed to save document {filename} back to S3")
        
    except Exception as e:
        logger.error(f"Error saving document {document_key}: {e}")

@app.post("/upload", response_model=UploadResponse)
async def upload_file(file: UploadFile = File(...)):
    """Upload file to MinIO storage"""
    try:
        await ensure_bucket_exists()
        
        # Generate unique filename
        file_extension = get_file_extension(file.filename)
        unique_filename = f"{uuid.uuid4()}{file_extension}"
        object_name = f"uploads/{unique_filename}"
        
        # Read file content
        file_content = await file.read()
        
        # Upload to MinIO
        minio_client.put_object(
            settings.minio_bucket,
            object_name,
            data=io.BytesIO(file_content),
            length=len(file_content),
            content_type=file.content_type or "application/octet-stream"
        )
        
        # Generate download URL
        download_url = f"http://localhost:{settings.webhook_port}/download/{unique_filename}"
        
        logger.info(f"Uploaded file {file.filename} as {unique_filename}")
        
        return UploadResponse(
            filename=file.filename,
            key=unique_filename,
            url=download_url,
            size=len(file_content),
            bucket=settings.minio_bucket
        )
        
    except Exception as e:
        logger.error(f"Upload error: {e}")
        raise HTTPException(status_code=500, detail=f"Upload failed: {str(e)}")

@app.get("/download/{filename}")
async def download_file(filename: str):
    """Download file from temporary storage (downloads from S3 if not cached)"""
    try:
        # First, try to download the file to temp storage if not already there
        temp_file_path = await download_s3_file_to_temp(filename)
        
        if not temp_file_path or not temp_file_path.exists():
            logger.error(f"Could not retrieve file {filename} from S3 or temp storage")
            raise HTTPException(status_code=404, detail=f"File not found: {filename}")
        
        # Get content type based on file extension
        file_extension = get_file_extension(filename).lower()
        content_type = "application/octet-stream"
        
        if file_extension in ['.docx', '.doc']:
            content_type = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
        elif file_extension in ['.xlsx', '.xls']:
            content_type = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
        elif file_extension in ['.pptx', '.ppt']:
            content_type = "application/vnd.openxmlformats-officedocument.presentationml.presentation"
        elif file_extension == '.pdf':
            content_type = "application/pdf"
        elif file_extension in ['.txt']:
            content_type = "text/plain"
        
        logger.info(f"Serving file {filename} from temp storage: {temp_file_path}")
        
        # Serve the file from temporary storage using FileResponse
        return FileResponse(
            path=str(temp_file_path),
            media_type=content_type,
            filename=filename,
            headers={
                "Access-Control-Allow-Origin": "*",
                "Access-Control-Allow-Methods": "GET, HEAD, OPTIONS",
                "Access-Control-Allow-Headers": "Content-Type, Range, Authorization",
                "Access-Control-Expose-Headers": "Content-Length, Accept-Ranges",
                "Cache-Control": "no-cache, no-store, must-revalidate",
                "Accept-Ranges": "bytes"
            }
        )
        
    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"Error downloading file {filename}: {e}")
        raise HTTPException(status_code=500, detail=f"Failed to download file: {str(e)}")

@app.options("/download/{filename}")
async def download_options(filename: str):
    """Handle CORS preflight requests for downloads"""
    return JSONResponse(
        content={},
        headers={
            "Access-Control-Allow-Origin": "*",
            "Access-Control-Allow-Methods": "GET, HEAD, OPTIONS",
            "Access-Control-Allow-Headers": "Content-Type, Range, Authorization",
            "Access-Control-Max-Age": "86400",
        }
    )

@app.get("/documents")
async def list_documents():
    """List all documents in MinIO bucket"""
    try:
        objects = minio_client.list_objects(settings.minio_bucket, recursive=True)
        documents = []
        
        for obj in objects:
            filename = obj.object_name.split('/')[-1]  # Get just the filename
            
            documents.append({
                "name": obj.object_name,
                "filename": filename,
                "size": obj.size,
                "last_modified": obj.last_modified.isoformat() if obj.last_modified else None,
                "download_url": f"http://{settings.host_ip}:{settings.webhook_port}/download/{filename}",
                "editor_url": f"http://{settings.host_ip}:{settings.webhook_port}/editor/{filename}"
            })
        
        return {"documents": documents, "count": len(documents)}
        
    except Exception as e:
        logger.error(f"Error listing documents: {e}")
        raise HTTPException(status_code=500, detail=f"Failed to list documents: {str(e)}")

@app.get("/temp-files")
async def list_temp_files():
    """List all files in temporary storage"""
    try:
        temp_files = []
        
        for temp_file in TEMP_DIR.glob("*"):
            if temp_file.is_file():
                stat = temp_file.stat()
                temp_files.append({
                    "filename": temp_file.name,
                    "size": stat.st_size,
                    "created": datetime.fromtimestamp(stat.st_ctime).isoformat(),
                    "modified": datetime.fromtimestamp(stat.st_mtime).isoformat(),
                    "age_hours": (datetime.now() - datetime.fromtimestamp(stat.st_mtime)).total_seconds() / 3600
                })
        
        return {
            "temp_files": temp_files, 
            "count": len(temp_files),
            "temp_directory": str(TEMP_DIR.absolute()),
            "ttl_hours": settings.temp_file_ttl_hours
        }
        
    except Exception as e:
        logger.error(f"Error listing temp files: {e}")
        raise HTTPException(status_code=500, detail=f"Failed to list temp files: {str(e)}")

@app.post("/cleanup-temp-files")
async def cleanup_temp_files_manual():
    """Manually trigger cleanup of old temporary files"""
    try:
        await cleanup_old_temp_files()
        return {"message": "Temp file cleanup completed", "timestamp": datetime.now().isoformat()}
    except Exception as e:
        logger.error(f"Error during manual temp cleanup: {e}")
        raise HTTPException(status_code=500, detail=f"Cleanup failed: {str(e)}")

@app.delete("/temp-files/{filename}")
async def delete_temp_file(filename: str):
    """Delete a specific temporary file"""
    try:
        temp_file_path = TEMP_DIR / filename
        
        if not temp_file_path.exists():
            raise HTTPException(status_code=404, detail=f"Temp file not found: {filename}")
        
        temp_file_path.unlink()
        logger.info(f"Deleted temp file: {filename}")
        
        return {"message": f"Temp file {filename} deleted successfully"}
        
    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"Error deleting temp file {filename}: {e}")
        raise HTTPException(status_code=500, detail=f"Failed to delete temp file: {str(e)}")

@app.get("/editor/{filename}", response_class=HTMLResponse)
async def document_editor(filename: str, request: Request):
    """Serve ONLYOFFICE document editor for a file"""
    try:
        # Find the original S3 path for this file
        original_s3_path = await find_original_s3_path(filename)
        
        if not original_s3_path:
            raise HTTPException(status_code=404, detail=f"Document not found: {filename}")
        
        # Generate consistent document key based on filename and S3 path (not random)
        # This ensures all users editing the same document get the same key for collaboration
        file_hash = hashlib.md5(f"{original_s3_path}".encode()).hexdigest()[:8]
        
        # Encode the S3 path in base64 to include in document key
        encoded_s3_path = base64.b64encode(original_s3_path.encode()).decode()
        
        # Generate consistent document key for collaboration
        # Format: doc_{hash}_{base64_s3_path}_{filename}
        document_key = f"doc_{file_hash}_{encoded_s3_path}_{filename}"
        
        logger.info(f"Generated document key for {filename} from S3 path {original_s3_path}: {document_key}")
        
        # Get file extension to determine document type
        file_extension = get_file_extension(filename).lower()
        
        # Determine document type
        if file_extension in ['.doc', '.docx', '.odt', '.txt', '.rtf']:
            document_type = 'word'
        elif file_extension in ['.xls', '.xlsx', '.ods', '.csv']:
            document_type = 'cell'
        elif file_extension in ['.ppt', '.pptx', '.odp']:
            document_type = 'slide'
        else:
            document_type = 'word'  # Default
        
        # Document download URL (via FastAPI proxy - accessible to ONLYOFFICE container)
        # Use host.docker.internal to allow Docker containers to access host services
        document_url = f"http://host.docker.internal:{settings.webhook_port}/download/{filename}"
        
        # Callback URL for saving (use host.docker.internal for Docker container access)
        callback_url = f"http://host.docker.internal:{settings.webhook_port}/webhook/callback"
        
        # Get user info from query parameters (for collaboration)
        user_id = request.query_params.get("user_id", "-1")
        username = request.query_params.get("username", "administrator")
        
        # Create ONLYOFFICE configuration object
        config = {
            "document": {
                "fileType": file_extension.replace('.', ''),
                "key": document_key,
                "title": filename,
                "url": document_url,
                "permissions": {
                    "edit": True,
                    "download": True,
                    "review": True,
                    "fillForms": True,
                    "comment": True
                }
            },
            "documentType": document_type,
            "editorConfig": {
                "mode": "edit",
                "lang": "en",
                "callbackUrl": callback_url,
                "user": {
                    "id": user_id,
                    "name": username
                },
                "customization": {
                    "autosave": True,
                    "forcesave": True,
                    "chat": True,
                    "comments": True,
                    "help": True,
                    "hideRightMenu": False,
                    "review": True,
                    "toolbar": True,
                    "zoom": 100,
                    "compactToolbar": False,
                    "plugins": True,
                    "toolbarNoTabs": False,
                    "features": {
                        "spellcheck": True,
                        "grammarcheck": True
                    }
                }
            },
            "width": "100%",
            "height": "100%"
        }
        
        # Generate JWT token if enabled
        jwt_token = ""
        if settings.jwt_enabled:
            jwt_token = generate_jwt_token(config)
            config["token"] = jwt_token
        
        # Convert config to JSON string for JavaScript
        config_json = json.dumps(config, indent=2)
        
        # Create HTML page with ONLYOFFICE editor
        html_content = f"""
        <!DOCTYPE html>
        <html>
        <head>
            <meta charset="utf-8">
            <title>Edit {filename} - ONLYOFFICE</title>
            <style>
                html, body {{
                    margin: 0;
                    padding: 0;
                    height: 100%;
                    font-family: Arial, sans-serif;
                    background-color: #f5f5f5;
                }}
                .container {{
                    display: flex;
                    flex-direction: column;
                    height: 100vh;
                    padding: 10px;
                    box-sizing: border-box;
                }}
                .header {{
                    background: white;
                    padding: 15px;
                    border-radius: 8px;
                    box-shadow: 0 2px 4px rgba(0,0,0,0.1);
                    margin-bottom: 10px;
                    flex-shrink: 0;
                }}
                .editor-container {{
                    background: white;
                    border-radius: 8px;
                    box-shadow: 0 2px 4px rgba(0,0,0,0.1);
                    overflow: hidden;
                    flex: 1;
                    display: flex;
                    flex-direction: column;
                }}
                #editor {{
                    width: 100%;
                    height: 100%;
                    min-height: 500px;
                    border: none;
                }}
                .info {{
                    display: flex;
                    justify-content: space-between;
                    align-items: center;
                }}
                .back-link {{
                    background: #007bff;
                    color: white;
                    padding: 8px 16px;
                    text-decoration: none;
                    border-radius: 4px;
                }}
                .back-link:hover {{
                    background: #0056b3;
                }}
            </style>
        </head>
        <body>
            <div class="container">
                <div class="header">
                    <div class="info">
                        <div>
                            <h2>📄 {filename}</h2>
                            <p><strong>Document Type:</strong> {document_type.title()}</p>
                            <p><strong>Current User:</strong> {username} (ID: {user_id})</p>
                            <p><strong>Document Key:</strong> <code>{document_key}</code></p>
                            <p><strong>Original S3 Path:</strong> <code>{original_s3_path}</code></p>
                            <p><small>Changes will be saved back to the original location</small></p>
                            <p><small>💡 <strong>Real-time Collaboration:</strong> Open this same URL in another window with different users to test collaboration!</small></p>
                        </div>
                        <a href="http://{settings.host_ip}:{settings.webhook_port}/" class="back-link">← Back to API</a>
                    </div>
                </div>
                
                <div class="editor-container">
                    <div id="editor"></div>
                </div>
            </div>

            <script src="{settings.onlyoffice_server_url}/web-apps/apps/api/documents/api.js"></script>
            <script>
                window.onload = function() {{
                    console.log('Initializing ONLYOFFICE editor...');
                    console.log('Document URL:', '{document_url}');
                    console.log('Callback URL:', '{callback_url}');
                    console.log('JWT Enabled:', {str(settings.jwt_enabled).lower()});
                    
                    // ONLYOFFICE configuration with JWT token
                    var config = {config_json};
                    
                    // Add event handlers for collaboration
                    config.events = {{
                        "onReady": function() {{
                            console.log("Document editor ready for user: {username}");
                        }},
                        "onError": function(event) {{
                            console.error("Editor error:", event);
                            alert("Error loading document: " + JSON.stringify(event));
                        }},
                        "onDocumentStateChange": function(event) {{
                            console.log("Document state changed:", event);
                        }},
                        "onInfo": function(event) {{
                            console.log("Editor info:", event);
                        }},
                        "onWarning": function(event) {{
                            console.warn("Editor warning:", event);
                        }},
                        "onRequestUsers": function(event) {{
                            console.log("Users requested:", event);
                        }},
                        "onRequestSendNotify": function(event) {{
                            console.log("Send notify requested:", event);
                        }},
                        "onCollaborativeChanges": function() {{
                            console.log("Collaborative changes detected");
                        }}
                    }};
                    
                    console.log('ONLYOFFICE config:', config);
                    
                    var docEditor = new DocsAPI.DocEditor("editor", config);
                }};
            </script>
        </body>
        </html>
        """
        
        return HTMLResponse(content=html_content)
        
    except Exception as e:
        logger.error(f"Error creating editor for {filename}: {e}")
        return HTMLResponse(
            content=f"<h1>Error</h1><p>Could not load editor for {filename}: {str(e)}</p>",
            status_code=500
        )

if __name__ == "__main__":
    uvicorn.run(
        "main:app",
        host=settings.webhook_host,
        port=settings.webhook_port,
        reload=True if settings.environment == "development" else False,
        log_level=settings.log_level.lower()
    ) 