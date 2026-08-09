@echo off
REM Check if Docker is running

docker ps >null 2>&1
if %errorlevel% neq 0 (
	echo Docker is not running. Please start Docker Desktop.
	exit /b 1
)

REM Run Docker commands
echo Docker is ready, starting...

docker compose pull
docker compose up -d
