@echo off
setlocal EnableDelayedExpansion
title FinOps Platform - One-Click Setup ^& Run

echo ============================================================
echo   SaaS FinOps Analytics Platform - Windows One-Click Setup
echo ============================================================
echo.

:: ─── Root directory (no trailing backslash) ───────────────────
set "ROOT=%~dp0"
if "%ROOT:~-1%"=="\" set "ROOT=%ROOT:~0,-1%"

:: ─── 1. Prerequisites ─────────────────────────────────────────
echo [1/6] Checking prerequisites...

where go >nul 2>&1
if %errorlevel% neq 0 (
    echo [ERROR] Go not found. Install from https://go.dev/dl/ then re-run.
    pause & exit /b 1
)
for /f "tokens=3" %%V in ('go version') do echo        Go %%V

where python >nul 2>&1
if %errorlevel% neq 0 (
    where python3 >nul 2>&1
    if %errorlevel% neq 0 (
        echo [ERROR] Python not found. Install from https://www.python.org/downloads/
        pause & exit /b 1
    )
    set "PYTHON=python3"
) else (
    set "PYTHON=python"
)
for /f "tokens=*" %%V in ('!PYTHON! --version 2^>^&1') do echo        %%V

where mysql >nul 2>&1
if %errorlevel% neq 0 (
    echo [WARN] mysql CLI not in PATH. Ensure MySQL 8.0 is running on port 3306.
) else (
    echo        MySQL CLI found.
)

set "SKIP_FRONTEND=1"
where node >nul 2>&1
if %errorlevel% equ 0 (
    for /f "tokens=*" %%V in ('node --version') do echo        Node %%V
    set "SKIP_FRONTEND=0"
) else (
    echo [WARN] Node.js not found - frontend will be skipped.
)
echo [OK] Prerequisites checked.
echo.

:: ─── 2. Database + migrations ─────────────────────────────────
echo [2/6] Setting up database and running migrations...
cd /d "%ROOT%"
!PYTHON! "%ROOT%\_dbsetup.py"
if %errorlevel% neq 0 (
    echo [ERROR] Database setup failed. Check config.ini credentials.
    pause & exit /b 1
)
echo [OK] Database ready.
echo.

:: ─── 3. Go workspace + deps ───────────────────────────────────
echo [3/6] Syncing Go workspace and downloading dependencies...
cd /d "%ROOT%"
go work sync
if %errorlevel% neq 0 echo [WARN] go work sync had issues - continuing.

for %%S in (api-gateway auth-service billing-service finops-service) do (
    if exist "%ROOT%\services\%%S\go.mod" (
        echo        [%%S] go mod download...
        cd /d "%ROOT%\services\%%S"
        go mod download >nul 2>&1
    )
)
for %%S in (config database redis) do (
    if exist "%ROOT%\shared\%%S\go.mod" (
        cd /d "%ROOT%\shared\%%S"
        go mod download >nul 2>&1
    )
)
cd /d "%ROOT%"
echo [OK] Go dependencies ready.
echo.

:: ─── 4. Python venv ───────────────────────────────────────────
echo [4/6] Setting up Python virtual environment...
cd /d "%ROOT%\services\ai-query-engine"

if not exist "venv\Scripts\activate.bat" (
    echo        Creating venv...
    !PYTHON! -m venv venv
    if !errorlevel! neq 0 (
        echo [ERROR] Failed to create Python venv.
        cd /d "%ROOT%" & pause & exit /b 1
    )
)

echo        Upgrading pip...
call venv\Scripts\activate.bat
python -m pip install --upgrade pip -q

echo        Installing packages (first run may take a few minutes)...
pip install -r requirements.txt -q
if !errorlevel! neq 0 (
    echo [ERROR] pip install failed. See error above.
    call venv\Scripts\deactivate.bat
    cd /d "%ROOT%" & pause & exit /b 1
)
call venv\Scripts\deactivate.bat

if not exist ".env" (
    if exist ".env.example" (
        copy ".env.example" ".env" >nul
        echo [INFO] Created .env from .env.example
    )
    echo [WARN] Edit services\ai-query-engine\.env and set GEMINI_API_KEY.
)
cd /d "%ROOT%"
echo [OK] Python environment ready.
echo.

:: ─── 5. Frontend ──────────────────────────────────────────────
echo [5/6] Installing frontend dependencies...
if "%SKIP_FRONTEND%"=="0" (
    cd /d "%ROOT%\services\frontend"
    if not exist "node_modules" (
        call npm install --silent
        if !errorlevel! neq 0 echo [WARN] npm install had issues.
    ) else (
        echo        node_modules already present, skipping.
    )
    cd /d "%ROOT%"
    echo [OK] Frontend ready.
) else (
    echo [SKIP] Node.js not found - skipping frontend.
)
echo.

:: ─── 6. Launch services ───────────────────────────────────────
echo [6/6] Launching services...
echo.
cd /d "%ROOT%"

> "%ROOT%\_launch_api_gateway.bat" (
    echo @echo off
    echo title API Gateway :8080
    echo cd /d "%ROOT%\services\api-gateway"
    echo go run main.go
    echo pause
)
> "%ROOT%\_launch_auth.bat" (
    echo @echo off
    echo title Auth Service :8081
    echo cd /d "%ROOT%\services\auth-service"
    echo go run main.go
    echo pause
)
> "%ROOT%\_launch_billing.bat" (
    echo @echo off
    echo title Billing Service :8082
    echo cd /d "%ROOT%\services\billing-service"
    echo go run main.go
    echo pause
)
> "%ROOT%\_launch_finops.bat" (
    echo @echo off
    echo title FinOps Service :8083
    echo cd /d "%ROOT%\services\finops-service"
    echo go run main.go
    echo pause
)
> "%ROOT%\_launch_ai.bat" (
    echo @echo off
    echo title AI Query Engine :8084
    echo cd /d "%ROOT%\services\ai-query-engine"
    echo call venv\Scripts\activate.bat
    echo python main.py
    echo pause
)

start "API Gateway"     cmd /k "%ROOT%\_launch_api_gateway.bat"
timeout /t 2 /nobreak >nul
start "Auth Service"    cmd /k "%ROOT%\_launch_auth.bat"
timeout /t 2 /nobreak >nul
start "Billing Service" cmd /k "%ROOT%\_launch_billing.bat"
timeout /t 2 /nobreak >nul
start "FinOps Service"  cmd /k "%ROOT%\_launch_finops.bat"
timeout /t 2 /nobreak >nul
start "AI Query Engine" cmd /k "%ROOT%\_launch_ai.bat"

if "%SKIP_FRONTEND%"=="0" (
    > "%ROOT%\_launch_frontend.bat" (
        echo @echo off
        echo title Frontend :5173
        echo cd /d "%ROOT%\services\frontend"
        echo npm run dev
        echo pause
    )
    timeout /t 2 /nobreak >nul
    start "Frontend" cmd /k "%ROOT%\_launch_frontend.bat"
)

echo ============================================================
echo   All services launched in separate windows.
echo.
echo   API Gateway  : http://localhost:8080/health
echo   Auth Service : http://localhost:8081/health
echo   Billing      : http://localhost:8082/health
echo   FinOps       : http://localhost:8083/health
echo   AI Engine    : http://localhost:8084/health
if "%SKIP_FRONTEND%"=="0" (
    echo   Frontend     : http://localhost:5173
)
echo.
echo   Run stop.bat to shut everything down.
echo ============================================================
echo.
pause
