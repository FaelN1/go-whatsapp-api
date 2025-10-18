@echo off
REM Script para executar migrations usando Docker - Windows
REM Uso: migrate.bat

echo ========================================
echo   WhatsApp API - Database Migration
echo   Usando Docker Container
echo ========================================
echo.

REM Verificar se Docker está rodando
docker info >nul 2>&1
if errorlevel 1 (
    echo ERRO: Docker nao esta rodando ou nao esta instalado!
    echo Por favor, inicie o Docker Desktop e tente novamente.
    pause
    exit /b 1
)

REM Verificar se o container do PostgreSQL está rodando
docker ps | findstr whatsapp-postgres >nul 2>&1
if errorlevel 1 (
    echo AVISO: Container PostgreSQL nao esta rodando!
    echo Iniciando containers...
    docker-compose up -d postgres
    echo Aguardando PostgreSQL inicializar...
    timeout /t 10 /nobreak >nul
)

echo.
echo Verificando conexao com o banco de dados...
docker exec whatsapp-postgres pg_isready -U whatsapp >nul 2>&1
if errorlevel 1 (
    echo ERRO: Nao foi possivel conectar ao PostgreSQL!
    echo Verifique se o container esta rodando: docker ps
    pause
    exit /b 1
)

echo ✓ Conexao com PostgreSQL OK
echo.

REM Procurar arquivos de migration
set MIGRATION_DIR=internal\platform\database\migrations
if not exist "%MIGRATION_DIR%" (
    echo ERRO: Diretorio de migrations nao encontrado: %MIGRATION_DIR%
    pause
    exit /b 1
)

echo Arquivos de migration encontrados:
dir /b "%MIGRATION_DIR%\*.sql"

echo.
echo ========================================
echo ATENCAO: Este script ira executar as migrations no banco de dados.
echo Container: whatsapp-postgres
echo Banco: whatsapp_db
echo Usuario: whatsapp
echo.
set /p CONFIRM="Deseja continuar? (S/N): "
if /i not "%CONFIRM%"=="S" (
    echo Operacao cancelada.
    exit /b 0
)

echo.
echo Executando migrations via Docker...
echo.

REM Executar cada arquivo SQL no container
for %%f in ("%MIGRATION_DIR%\*.sql") do (
    echo.
    echo --------------------------------------------------
    echo Executando: %%~nxf
    echo --------------------------------------------------
    
    REM Copiar arquivo SQL para o container e executar
    docker cp "%%f" whatsapp-postgres:/tmp/migration.sql
    docker exec whatsapp-postgres psql -U whatsapp -d whatsapp_db -f /tmp/migration.sql
    
    if errorlevel 1 (
        echo ERRO ao executar %%~nxf
        pause
        exit /b 1
    ) else (
        echo ✓ %%~nxf executado com sucesso!
    )
    
    REM Limpar arquivo temporario
    docker exec whatsapp-postgres rm /tmp/migration.sql
)

echo.
echo ========================================
echo ✓ Todas as migrations foram executadas com sucesso!
echo ========================================
echo.
echo Verificando tabelas criadas...
docker exec whatsapp-postgres psql -U whatsapp -d whatsapp_db -c "\dt message_*"

echo.
echo Pronto! As tabelas de analytics estao disponiveis.
pause
