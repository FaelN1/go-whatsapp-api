@echo off
REM Script de deploy para Go WhatsApp API com PM2

echo ====================================
echo  Go WhatsApp API - Deploy Script
echo ====================================
echo.

REM Verificar se PM2 está instalado
where pm2 >nul 2>nul
if %errorlevel% neq 0 (
    echo [ERROR] PM2 não está instalado!
    echo Execute: npm install -g pm2
    exit /b 1
)

REM Verificar se Go está instalado
where go >nul 2>nul
if %errorlevel% neq 0 (
    echo [ERROR] Go não está instalado!
    exit /b 1
)

REM Criar diretórios necessários
echo [1/5] Criando diretórios...
if not exist "bin" mkdir bin
if not exist "logs" mkdir logs
if not exist "data" mkdir data
echo OK

REM Compilar a aplicação
echo [2/5] Compilando a aplicação...
go build -o .\bin\server.exe .\cmd\server\main.go
if %errorlevel% neq 0 (
    echo [ERROR] Falha ao compilar a aplicação!
    exit /b 1
)
echo OK

REM Verificar se .env existe
echo [3/5] Verificando configurações...
if not exist ".env" (
    echo [WARNING] Arquivo .env não encontrado!
    if exist ".env.example" (
        echo Copiando .env.example para .env...
        copy .env.example .env
        echo [IMPORTANTE] Edite o arquivo .env com suas configurações!
        pause
    ) else (
        echo [ERROR] Arquivo .env.example não encontrado!
        exit /b 1
    )
)
echo OK

REM Iniciar com PM2
echo [4/5] Iniciando com PM2...
pm2 start ecosystem.config.js
if %errorlevel% neq 0 (
    echo [ERROR] Falha ao iniciar com PM2!
    exit /b 1
)
echo OK

REM Salvar configuração do PM2
echo [5/5] Salvando configuração do PM2...
pm2 save
echo OK

echo.
echo ====================================
echo  Deploy concluído com sucesso!
echo ====================================
echo.
echo Comandos úteis:
echo   pm2 status              - Ver status
echo   pm2 logs go-whatsapp-api - Ver logs
echo   pm2 monit               - Monitorar
echo   pm2 restart go-whatsapp-api - Reiniciar
echo.

REM Mostrar status
pm2 status

pause
