#!/bin/bash
# Script de atualização para Go WhatsApp API

set -e  # Exit on error

echo "===================================="
echo " Go WhatsApp API - Update Script"
echo "===================================="
echo ""

# Cores para output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Verificar se PM2 está instalado
if ! command -v pm2 &> /dev/null; then
    echo -e "${RED}[ERROR] PM2 não está instalado!${NC}"
    exit 1
fi

# Parar a aplicação
echo "[1/4] Parando a aplicação..."
pm2 stop go-whatsapp-api 2>/dev/null || echo -e "${YELLOW}Aplicação não estava rodando${NC}"
echo -e "${GREEN}OK${NC}"

# Atualizar código (opcional - descomente se usar git)
# echo "[2/4] Atualizando código..."
# git pull
# if [ $? -ne 0 ]; then
#     echo -e "${RED}[ERROR] Falha ao atualizar código!${NC}"
#     pm2 restart go-whatsapp-api
#     exit 1
# fi
# echo -e "${GREEN}OK${NC}"

# Recompilar
echo "[2/4] Recompilando..."
go build -o ./bin/server ./cmd/server/main.go
if [ $? -ne 0 ]; then
    echo -e "${RED}[ERROR] Falha ao compilar!${NC}"
    echo "Reiniciando versão anterior..."
    pm2 restart go-whatsapp-api
    exit 1
fi
echo -e "${GREEN}OK${NC}"

# Reiniciar
echo "[3/4] Reiniciando aplicação..."
pm2 restart go-whatsapp-api
echo -e "${GREEN}OK${NC}"

# Salvar configuração
echo "[4/4] Salvando configuração..."
pm2 save
echo -e "${GREEN}OK${NC}"

echo ""
echo "===================================="
echo " Atualização concluída!"
echo "===================================="
echo ""

pm2 status
