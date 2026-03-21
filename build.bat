@echo off
setlocal EnableDelayedExpansion
title FinOps Platform - Build All Services

echo ============================================================
echo   SaaS FinOps Analytics Platform - Build All Binaries
echo ============================================================
echo.

set "ROOT=%~dp0"
if "%ROOT:~-1%"=="\" set "ROOT=%ROOT:~0,-1%"
set "DIST=%ROOT%\dist"
set ERRORS=0

:: Create output directory
if not exist "%DIST%" mkdir "%DIST%"

echo [INFO] Output directory: %DIST%
echo.

:: ─── Check Go ────────────────────────────────────────────────
where go >nul 2>&1
if %errorlevel% neq 0 (
    echo [ERROR] Go not found. Install from https://go.dev/dl/
    pause & exit /b 1
)
echo [INFO] Using: & go version
echo.

:: ─── Sync workspace ──────────────────────────────────────────
echo [BUILD] Syncing Go workspace...
cd /d "%ROOT%"
go work sync
if %errorlevel% neq 0 echo [WARN] go work sync had issues - continuing.
echo.

:: ─── Build migrations tool ───────────────────────────────────
echo [BUILD] migrations tool...
cd /d "%ROOT%\migrations"
go build -o "%DIST%\migrate.exe" .
if !errorlevel! neq 0 (
    echo [FAIL] migrations
    set /a ERRORS+=1
) else (
    echo [OK]   dist\migrate.exe
)

:: ─── Build Go services ───────────────────────────────────────
for %%S in (api-gateway auth-service billing-service finops-service) do (
    echo [BUILD] %%S...
    cd /d "%ROOT%\services\%%S"
    go build -ldflags="-s -w" -o "%DIST%\%%S.exe" .
    if !errorlevel! neq 0 (
        echo [FAIL] %%S
        set /a ERRORS+=1
    ) else (
        echo [OK]   dist\%%S.exe
    )
)
cd /d "%ROOT%"

:: ─── Build frontend ──────────────────────────────────────────
echo.
where node >nul 2>&1
if %errorlevel% equ 0 (
    echo [BUILD] frontend...
    cd /d "%ROOT%\services\frontend"
    if not exist "node_modules" call npm install --silent
    call npm run build
    if !errorlevel! neq 0 (
        echo [FAIL] frontend
        set /a ERRORS+=1
    ) else (
        if not exist "%DIST%\frontend" mkdir "%DIST%\frontend"
        xcopy /E /I /Q dist "%DIST%\frontend" >nul 2>&1
        echo [OK]   dist\frontend\ ^(static assets^)
    )
    cd /d "%ROOT%"
) else (
    echo [SKIP] Node.js not found - skipping frontend build.
)

:: ─── Copy config + migrations SQL ────────────────────────────
echo.
echo [COPY] Bundling config and migrations...
copy /Y "%ROOT%\config.ini" "%DIST%\config.ini" >nul
if not exist "%DIST%\migrations" mkdir "%DIST%\migrations"
copy /Y "%ROOT%\migrations\*.sql" "%DIST%\migrations\" >nul
echo [OK]   config.ini and migrations SQL copied.

:: ─── Write run-dist.bat (runs pre-built binaries) ────────────
echo.
echo [GEN]  Generating dist\run-dist.bat...
> "%DIST%\run-dist.bat" (
    echo @echo off
    echo setlocal EnableDelayedExpansion
    echo title FinOps Platform - Run Pre-Built Binaries
    echo set "ROOT=%%~dp0"
    echo if "%%ROOT:~-1%%"=="\" set "ROOT=%%ROOT:~0,-1%%"
    echo.
    echo :: Run migrations first
    echo echo [MIGRATE] Running database migrations...
    echo set /p DUMMY=Press Enter after MySQL is ready, then migrations will run...
    echo "%%ROOT%%\migrate.exe"
    echo if %%errorlevel%% neq 0 ^( echo [ERROR] Migrations failed. ^& pause ^& exit /b 1 ^)
    echo.
    echo :: Launch each service
    echo for %%S in ^(api-gateway auth-service billing-service finops-service^) do ^(
    echo     start "%%S" cmd /k "cd /d %%ROOT%% ^&^& set CONFIG_PATH=%%ROOT%%\config.ini ^&^& %%ROOT%%\%%S.exe"
    echo     timeout /t 2 /nobreak ^>nul
    echo ^)
    echo echo.
    echo echo All services launched. API Gateway: http://localhost:8080/health
    echo pause
)
echo [OK]   dist\run-dist.bat

:: ─── Summary ─────────────────────────────────────────────────
echo.
echo ============================================================
if %ERRORS% equ 0 (
    echo   Build SUCCESSFUL - all binaries in dist\
) else (
    echo   Build completed with %ERRORS% error^(s^) - check output above.
)
echo ============================================================
echo.
dir /b "%DIST%"
echo.
pause
