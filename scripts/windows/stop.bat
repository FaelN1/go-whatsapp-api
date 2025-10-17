@echo off
REM Script para parar a aplicação

echo Parando Go WhatsApp API...
pm2 stop go-whatsapp-api

echo.
pm2 status

pause
