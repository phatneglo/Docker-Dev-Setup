@echo off
echo 🚀 Starting CoTURN STUN/TURN Server (Official Config)...
cd /d D:\DOCKER_CONTAINER_FILES\coturn

echo 📝 Configuration based on official CoTURN documentation:
echo ✅ External IP mapping: 127.0.0.1 (key for TURN functionality)
echo ✅ Static authentication: coturn123secret
echo ✅ Safe port range: 60000-60100
echo.

docker-compose --env-file ../coturn.env up -d

echo ⏳ Waiting for CoTURN to initialize...
timeout /t 8 /nobreak >nul

echo ✅ CoTURN Status:
docker ps --filter "name=coturn_server" --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}"

echo.
echo 📋 Connection Details (Official CoTURN Config):
echo STUN Server: stun://localhost:3478
echo TURN Server: turn://localhost:3478  
echo Username: any (e.g., "user")
echo Password: coturn123secret
echo External IP: 127.0.0.1 (reported to clients)
echo.

echo 🧪 Testing connectivity...
powershell -Command "try { Test-NetConnection -ComputerName localhost -Port 3478 -InformationLevel Quiet; if($?) { Write-Host '✅ Port 3478 is accessible' -ForegroundColor Green } else { Write-Host '❌ Port 3478 not accessible' -ForegroundColor Red } } catch { Write-Host '⚠️  Network test unavailable' -ForegroundColor Yellow }"

echo.
echo 📝 Quick Commands:
echo View logs: docker logs coturn_server
echo Stop server: docker-compose --env-file ../coturn.env down
echo Check config: type turnserver.conf
echo.

if exist "test.html" (
    echo 🌐 Opening test page...
    start "" "test.html"
    echo.
    echo 🎯 Expected Results (with official config):
    echo ✅ STUN: Should work (IP discovery)
    echo ✅ TURN: Should now work (relay candidates with external-ip)
) else (
    echo 💡 WebRTC ICE Configuration:
    echo {
    echo   "iceServers": [
    echo     { "urls": "stun:localhost:3478" },
    echo     {
    echo       "urls": "turn:localhost:3478",
    echo       "username": "user",
    echo       "credential": "coturn123secret"
    echo     }
    echo   ]
    echo }
)

echo.
echo 🎉 CoTURN is ready with official configuration!
echo 📝 Based on: CoTURN README.turnserver documentation
pause