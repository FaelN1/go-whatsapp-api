#!/bin/bash
# Script para executar migrations no PostgreSQL - Linux/Mac
# Uso: ./migrate.sh

set -e  # Sair em caso de erro

echo "========================================"
echo "  WhatsApp API - Database Migration"
echo "========================================"
echo ""

# Carregar variáveis de ambiente do .env
if [ -f .env ]; then
    echo "Carregando configurações do .env..."
    export $(grep -v '^#' .env | xargs)
else
    echo "ERRO: Arquivo .env não encontrado!"
    echo "Crie um arquivo .env com DATABASE_DSN configurado"
    exit 1
fi

if [ "$DB_DRIVER" = "postgres" ]; then
    echo "Driver: PostgreSQL"
else
    echo "AVISO: DB_DRIVER não está configurado como 'postgres'"
    echo "Este script é para PostgreSQL. Pressione Ctrl+C para cancelar."
    read -p "Pressione Enter para continuar..."
fi

echo ""
echo "Procurando arquivos de migration..."
MIGRATION_DIR="internal/platform/database/migrations"
if [ ! -d "$MIGRATION_DIR" ]; then
    echo "ERRO: Diretório de migrations não encontrado: $MIGRATION_DIR"
    exit 1
fi

echo ""
echo "Arquivos de migration encontrados:"
ls -1 "$MIGRATION_DIR"/*.sql

echo ""
echo "========================================"
echo "ATENÇÃO: Este script irá executar as migrations no banco de dados."
echo "DATABASE_DSN: $DATABASE_DSN"
echo ""
read -p "Deseja continuar? (s/N): " CONFIRM
if [ "$CONFIRM" != "s" ] && [ "$CONFIRM" != "S" ]; then
    echo "Operação cancelada."
    exit 0
fi

echo ""
echo "Executando migrations..."
echo ""

# Verificar se psql está instalado
if ! command -v psql &> /dev/null; then
    echo "ERRO: psql não está instalado ou não está no PATH"
    echo "Instale o PostgreSQL client: sudo apt-get install postgresql-client"
    exit 1
fi

# Executar cada arquivo SQL
for sql_file in "$MIGRATION_DIR"/*.sql; do
    echo ""
    echo "--------------------------------------------------"
    echo "Executando: $(basename "$sql_file")"
    echo "--------------------------------------------------"
    
    if psql "$DATABASE_DSN" -f "$sql_file"; then
        echo "✓ $(basename "$sql_file") executado com sucesso!"
    else
        echo "ERRO ao executar $(basename "$sql_file")"
        exit 1
    fi
done

echo ""
echo "========================================"
echo "✓ Todas as migrations foram executadas com sucesso!"
echo "========================================"
