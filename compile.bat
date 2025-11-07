@echo off
REM compile.bat - compiles the go-mfire CLI into an executable in the go-mfire folder
echo Building go-mfire...
cd /d "%~dp0"
go mod tidy >nul 2>&1
go build -o mfire.exe ./cmd/mfire
if %ERRORLEVEL% equ 0 (
    echo Build succeeded: %~dp0mfire.exe
) else (
    echo Build failed.
    exit /b %ERRORLEVEL%
)
