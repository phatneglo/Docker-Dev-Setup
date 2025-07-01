@echo off
echo ============================================
echo Starting ONLYOFFICE MinIO API Server...
echo ============================================

REM Change to the script directory
cd /d "%~dp0"

REM Check if Python is available
python --version >nul 2>&1
if errorlevel 1 (
    echo ERROR: Python is not installed or not in PATH.
    echo Please install Python 3.8+ and try again.
    pause
    exit /b 1
)

REM Check if virtual environment exists, create if not
if not exist "venv" (
    echo Creating Python virtual environment...
    python -m venv venv
)

REM Activate virtual environment
echo Activating virtual environment...
call venv\Scripts\activate.bat

REM Install requirements
echo Installing dependencies...
pip install -r requirements.txt

REM Start the FastAPI server
echo.
echo ============================================
echo Starting FastAPI Server on port 3000...
echo ============================================
echo.
echo Access URLs:
echo - API Server: http://localhost:3000
echo - API Documentation: http://localhost:3000/docs
echo - Health Check: http://localhost:3000/health
echo.
echo Webhook URL for ONLYOFFICE: http://localhost:3000/webhook/callback
echo.

python main.py

pause 