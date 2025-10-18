#!/bin/bash
# Script para executar migrations usando Docker - Linux/Mac
# Uso: ./migrate.sh

set -e  # Sair em caso de erro

echo "========================================"
echo "  WhatsApp API - Database Migration"
echo "  Usando Docker Container"
echo "========================================"
echo ""

# Verificar se Docker está rodando
if ! docker info > /dev/null 2>&1; then
    echo "ERRO: Docker não está rodando ou não está instalado!"
    echo "Por favor, inicie o Docker e tente novamente."
    exit 1
fi

# Verificar se o container do PostgreSQL está rodando
if ! docker ps | grep -q whatsapp-postgres; then
    echo "AVISO: Container PostgreSQL não está rodando!"
    echo "Iniciando containers..."
    docker-compose up -d postgres
    echo "Aguardando PostgreSQL inicializar..."
    sleep 10
fi

echo ""
echo "Verificando conexão com o banco de dados..."
if ! docker exec whatsapp-postgres pg_isready -U whatsapp > /dev/null 2>&1; then
    echo "ERRO: Não foi possível conectar ao PostgreSQL!"
    echo "Verifique se o container está rodando: docker ps"
    exit 1
fi

echo "✓ Conexão com PostgreSQL OK"
echo ""

# Procurar arquivos de migration
MIGRATION_DIR="internal/platform/database/migrations"
if [ ! -d "$MIGRATION_DIR" ]; then
    echo "ERRO: Diretório de migrations não encontrado: $MIGRATION_DIR"
    exit 1
fi

echo "Arquivos de migration encontrados:"
ls -1 "$MIGRATION_DIR"/*.sql

echo ""
echo "========================================"
echo "ATENÇÃO: Este script irá executar as migrations no banco de dados."
echo "Container: whatsapp-postgres"
echo "Banco: whatsapp_db"
echo "Usuário: whatsapp"
echo ""
read -p "Deseja continuar? (s/N): " CONFIRM
if [ "$CONFIRM" != "s" ] && [ "$CONFIRM" != "S" ]; then
    echo "Operação cancelada."
    exit 0
fi

echo ""
echo "Executando migrations via Docker..."
echo ""

# Executar cada arquivo SQL no container
for sql_file in "$MIGRATION_DIR"/*.sql; do
    echo ""
    echo "--------------------------------------------------"
    echo "Executando: $(basename "$sql_file")"
    echo "--------------------------------------------------"
    
    # Copiar arquivo SQL para o container e executar
    docker cp "$sql_file" whatsapp-postgres:/tmp/migration.sql
    docker exec whatsapp-postgres psql -U whatsapp -d whatsapp_db -f /tmp/migration.sql
    
    if [ $? -eq 0 ]; then
        echo "✓ $(basename "$sql_file") executado com sucesso!"
    else
        echo "ERRO ao executar $(basename "$sql_file")"
        exit 1
    fi
    
    # Limpar arquivo temporário
    docker exec whatsapp-postgres rm /tmp/migration.sql
done

echo ""
echo "========================================"
echo "✓ Todas as migrations foram executadas com sucesso!"
echo "========================================"
echo ""
echo "Verificando tabelas criadas..."
docker exec whatsapp-postgres psql -U whatsapp -d whatsapp_db -c "\dt message_*"

echo ""
echo "Pronto! As tabelas de analytics estão disponíveis."
