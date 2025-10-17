@echo off
REM Script de atualização para Go WhatsApp API

echo ====================================
echo  Go WhatsApp API - Update Script
echo ====================================
echo.

REM Verificar se PM2 está instalado
where pm2 >nul 2>nul
if %errorlevel% neq 0 (
    echo [ERROR] PM2 não está instalado!
    exit /b 1
)

REM Parar a aplicação
echo [1/4] Parando a aplicação...
pm2 stop go-whatsapp-api
echo OK

REM Atualizar código (opcional - descomente se usar git)
REM echo [2/4] Atualizando código...
REM git pull
REM echo OK

REM Recompilar
echo [2/4] Recompilando...
go build -o .\bin\server.exe .\cmd\server\main.go
if %errorlevel% neq 0 (
    echo [ERROR] Falha ao compilar!
    echo Reiniciando versão anterior...
    pm2 restart go-whatsapp-api
    exit /b 1
)
echo OK

REM Reiniciar
echo [3/4] Reiniciando aplicação...
pm2 restart go-whatsapp-api
echo OK

REM Salvar configuração
echo [4/4] Salvando configuração...
pm2 save
echo OK

echo.
echo ====================================
echo  Atualização concluída!
echo ====================================
echo.

pm2 status

pause
