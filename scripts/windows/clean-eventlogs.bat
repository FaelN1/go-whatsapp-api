@echo off
REM Wrapper to run the PowerShell clean-eventlogs script from cmd.exe
SETLOCAL
SET SCRIPT_DIR=%~dp0
SET PS_SCRIPT=%SCRIPT_DIR%clean-eventlogs.ps1
IF "%1"=="" (
  ECHO Usage: %~nx0 [path] [days] [keepLatest] [--dry-run]
  ECHO Example: %~nx0 .\event_logs 30 0 --dry-run
  EXIT /B 1
)

REM Build arguments
SET ARG_PATH=%1
SET ARG_DAYS=%2
IF "%ARG_DAYS%"=="" SET ARG_DAYS=30
SET ARG_KEEP=%3
IF "%ARG_KEEP%"=="" SET ARG_KEEP=0
SET ARG_DRY=%4

powershell -NoProfile -ExecutionPolicy Bypass -File "%PS_SCRIPT%" -Path "%ARG_PATH%" -Days %ARG_DAYS% -KeepLatest %ARG_KEEP% %ARG_DRY%
ENDLOCAL