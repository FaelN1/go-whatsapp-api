@echo off
REM Script para executar migrations no PostgreSQL - Windows
REM Uso: migrate.bat

echo ========================================
echo   WhatsApp API - Database Migration
echo ========================================
echo.

REM Carregar variáveis de ambiente do .env
if exist .env (
    echo Carregando configurações do .env...
    for /f "tokens=1,2 delims==" %%a in (.env) do (
        if "%%a"=="DATABASE_DSN" set DATABASE_DSN=%%b
        if "%%a"=="DB_DRIVER" set DB_DRIVER=%%b
    )
) else (
    echo ERRO: Arquivo .env não encontrado!
    echo Crie um arquivo .env com DATABASE_DSN configurado
    pause
    exit /b 1
)

if "%DB_DRIVER%"=="postgres" (
    echo Driver: PostgreSQL
) else (
    echo AVISO: DB_DRIVER não está configurado como 'postgres'
    echo Este script é para PostgreSQL. Pressione Ctrl+C para cancelar.
    pause
)

echo.
echo Procurando arquivos de migration...
set MIGRATION_DIR=internal\platform\database\migrations
if not exist "%MIGRATION_DIR%" (
    echo ERRO: Diretório de migrations não encontrado: %MIGRATION_DIR%
    pause
    exit /b 1
)

echo.
echo Arquivos de migration encontrados:
dir /b "%MIGRATION_DIR%\*.sql"

echo.
echo ========================================
echo ATENÇÃO: Este script irá executar as migrations no banco de dados.
echo DATABASE_DSN: %DATABASE_DSN%
echo.
set /p CONFIRM="Deseja continuar? (S/N): "
if /i not "%CONFIRM%"=="S" (
    echo Operação cancelada.
    exit /b 0
)

echo.
echo Executando migrations...
echo.

REM Extrair detalhes da connection string
REM Formato esperado: postgresql://user:password@host:port/database
for /f "tokens=1,2,3 delims=:/@" %%a in ("%DATABASE_DSN%") do (
    set DB_USER=%%b
    set DB_PASS=%%c
)

REM Executar cada arquivo SQL
for %%f in ("%MIGRATION_DIR%\*.sql") do (
    echo.
    echo --------------------------------------------------
    echo Executando: %%~nxf
    echo --------------------------------------------------
    
    REM Usar psql para executar o SQL
    psql "%DATABASE_DSN%" -f "%%f"
    
    if errorlevel 1 (
        echo ERRO ao executar %%~nxf
        pause
        exit /b 1
    ) else (
        echo ✓ %%~nxf executado com sucesso!
    )
)

echo.
echo ========================================
echo ✓ Todas as migrations foram executadas com sucesso!
echo ========================================
pause
