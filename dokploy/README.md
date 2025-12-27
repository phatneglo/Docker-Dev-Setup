# Dokploy - Self-Hosted Deployment Platform

Dokploy is a self-hosted deployment platform that helps you deploy and manage applications easily.

## Quick Start

1. **Load environment variables:**
   ```bash
   # Windows PowerShell
   Get-Content ..\dokploy.env | ForEach-Object { if ($_ -match '^([^=]+)=(.*)$') { [Environment]::SetEnvironmentVariable($matches[1], $matches[2], 'Process') } }

   # Linux/Mac
   export $(cat ../dokploy.env | xargs)
   ```

2. **Start the services:**
   ```bash
   cd dokploy
   docker-compose up -d
   ```

3. **Access Dokploy:**
   - Web UI: http://localhost:3000
   - Default credentials: Set up on first login

## Configuration

Edit `../dokploy.env` to configure:
- Database credentials
- Port mappings
- Security keys

## Services

- **dokploy**: Main application (port 3000)
- **dokploy-db**: PostgreSQL database (port 5433)

## Data Persistence

All data is stored in `./data/`:
- Database: `./data/db/`
- Application data: `./data/`

## Security Notes

⚠️ **Important**: Change the default `SECRET_KEY` and `ENCRYPTION_KEY` in `dokploy.env` before deploying to production!

## Network

Services run on the `dokploy_network` network.

