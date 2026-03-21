@echo off
setlocal EnableDelayedExpansion
title SaaS FinOps Analytics Platform - Stop Services

echo ============================================================
echo   SaaS FinOps Analytics Platform - Stopping All Services
echo ============================================================
echo.

:: ─── Kill service windows by title ───────────────────────────
echo [STOP] Closing service windows...

for %%T in (
    "API Gateway"
    "Auth Service"
    "Billing Service"
    "FinOps Service"
    "AI Query Engine"
    "Frontend"
) do (
    taskkill /FI "WINDOWTITLE eq %%~T" /T /F >nul 2>&1
    if !errorlevel! equ 0 (
        echo   [OK] Stopped %%~T
    ) else (
        echo   [--] %%~T was not running
    )
)

echo.

:: ─── Kill processes by port ───────────────────────────────────
echo [STOP] Releasing ports 8080-8084 and 5173...

for %%P in (8080 8081 8082 8083 8084 5173) do (
    for /f "tokens=5" %%I in ('netstat -ano ^| findstr ":%%P " ^| findstr "LISTENING" 2^>nul') do (
        if not "%%I"=="" (
            taskkill /PID %%I /T /F >nul 2>&1
            echo   [OK] Freed port %%P ^(PID %%I^)
        )
    )
)

echo.

:: ─── Clean up temp launcher scripts ──────────────────────────
echo [CLEAN] Removing temporary launcher scripts...

set ROOT=%~dp0
if "%ROOT:~-1%"=="\" set ROOT=%ROOT:~0,-1%

for %%F in (
    "_launch_api_gateway.bat"
    "_launch_auth.bat"
    "_launch_billing.bat"
    "_launch_finops.bat"
    "_launch_ai.bat"
    "_launch_frontend.bat"
) do (
    if exist "%ROOT%\%%F" (
        del /f /q "%ROOT%\%%F" >nul 2>&1
        echo   [OK] Deleted %%F
    )
)

echo.
echo ============================================================
echo   All services stopped.
echo ============================================================
echo.
pause
