@echo off
REM Script para instalar PM2

echo ====================================
echo  Instalação do PM2
echo ====================================
echo.

REM Verificar se Node.js está instalado
where node >nul 2>nul
if %errorlevel% neq 0 (
    echo [ERROR] Node.js não está instalado!
    echo.
    echo Baixe e instale o Node.js em: https://nodejs.org/
    pause
    exit /b 1
)

echo Node.js encontrado:
node --version
echo.

REM Verificar se npm está disponível
where npm >nul 2>nul
if %errorlevel% neq 0 (
    echo [ERROR] npm não está disponível!
    pause
    exit /b 1
)

echo npm encontrado:
npm --version
echo.

REM Instalar PM2 globalmente
echo Instalando PM2...
npm install -g pm2

if %errorlevel% neq 0 (
    echo.
    echo [ERROR] Falha ao instalar PM2!
    echo Tente executar como Administrador
    pause
    exit /b 1
)

echo.
echo ====================================
echo  PM2 instalado com sucesso!
echo ====================================
echo.

REM Verificar instalação
pm2 --version
echo.

echo Comandos úteis:
echo   pm2 --help              - Ajuda
echo   pm2 list                - Listar processos
echo   pm2 startup             - Configurar inicialização automática
echo.

pause
