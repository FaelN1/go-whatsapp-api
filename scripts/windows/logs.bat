@echo off
REM Script para ver logs da aplicação

if "%1"=="" (
    set LINES=50
) else (
    set LINES=%1
)

echo Mostrando últimas %LINES% linhas de log...
echo Use Ctrl+C para sair do modo live
echo.

pm2 logs go-whatsapp-api --lines %LINES%
