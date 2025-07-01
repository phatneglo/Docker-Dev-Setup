# ONLYOFFICE Document Server - Windows Docker Setup
# Following official documentation: https://helpcenter.onlyoffice.com/docs/installation/docs-community-install-docker.aspx

Write-Host "============================================" -ForegroundColor Green
Write-Host "Starting ONLYOFFICE Document Server..." -ForegroundColor Green
Write-Host "============================================" -ForegroundColor Green

# Set the current directory path
$currentPath = "D:\DOCKER_CONTAINER_FILES\only_office"

# Check if Docker is running
try {
    docker info | Out-Null
    Write-Host "Docker is running" -ForegroundColor Green
}
catch {
    Write-Host "Docker is not running or not accessible." -ForegroundColor Red
    Write-Host "Please start Docker Desktop and try again." -ForegroundColor Red
    Read-Host "Press Enter to exit"
    exit 1
}

# Stop and remove existing container if it exists
Write-Host "Cleaning up existing container..." -ForegroundColor Yellow
docker stop onlyoffice-documentserver 2>$null
docker rm onlyoffice-documentserver 2>$null

# Run ONLYOFFICE Document Server with data persistence
Write-Host "Starting ONLYOFFICE Document Server..." -ForegroundColor Yellow

$dockerCmd = @(
    "run"
    "-i"
    "-t"
    "-d"
    "-p"
    "80:80"
    "--restart=always"
    "--name"
    "onlyoffice-documentserver"
    "-v"
    "$currentPath\data\DocumentServer\logs:/var/log/onlyoffice"
    "-v"
    "$currentPath\data\DocumentServer\data:/var/www/onlyoffice/Data"
    "-v"
    "$currentPath\data\DocumentServer\lib:/var/lib/onlyoffice"
    "-v"
    "$currentPath\data\DocumentServer\db:/var/lib/postgresql"
    "-e"
    "JWT_SECRET=0yq5h3to9"
    "onlyoffice/documentserver"
)

# Execute Docker command
$containerId = & docker @dockerCmd

if ($LASTEXITCODE -eq 0) {
    Write-Host "ONLYOFFICE Document Server started successfully!" -ForegroundColor Green
    Write-Host "Container ID: $containerId" -ForegroundColor Green
    Write-Host ""
    Write-Host "============================================" -ForegroundColor Green
    Write-Host "ACCESS INFORMATION" -ForegroundColor Green
    Write-Host "============================================" -ForegroundColor Green
    Write-Host "Document Server: http://localhost" -ForegroundColor Cyan
    Write-Host "API Endpoint: http://localhost/web-apps/apps/api/documents/api.js" -ForegroundColor Cyan
    Write-Host "Health Check: http://localhost/healthcheck" -ForegroundColor Cyan
    Write-Host "JWT Secret: 0yq5h3to9" -ForegroundColor Cyan
    Write-Host ""
    Write-Host "============================================" -ForegroundColor Green
    Write-Host "USEFUL COMMANDS" -ForegroundColor Green
    Write-Host "============================================" -ForegroundColor Green
    Write-Host "View logs: docker logs -f onlyoffice-documentserver" -ForegroundColor Yellow
    Write-Host "Restart: docker restart onlyoffice-documentserver" -ForegroundColor Yellow
    Write-Host "Stop: docker stop onlyoffice-documentserver" -ForegroundColor Yellow
    Write-Host "Remove: docker rm onlyoffice-documentserver" -ForegroundColor Yellow
    Write-Host ""
    
    # Wait for service to be ready
    Write-Host "Waiting for service to be ready..." -ForegroundColor Yellow
    Start-Sleep -Seconds 10
    
    # Test health check
    try {
        $response = Invoke-WebRequest -Uri "http://localhost/healthcheck" -TimeoutSec 30
        if ($response.StatusCode -eq 200) {
            Write-Host "Health check passed! Service is ready." -ForegroundColor Green
        }
    }
    catch {
        Write-Host "Service is starting up. Health check will be available shortly." -ForegroundColor Yellow
    }
    
} else {
    Write-Host "Failed to start ONLYOFFICE Document Server" -ForegroundColor Red
    Write-Host "Check Docker logs for more information." -ForegroundColor Red
}

Write-Host ""
Write-Host "Press Enter to continue..." -ForegroundColor Gray
Read-Host 