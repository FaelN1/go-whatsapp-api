#!/bin/bash
# Script de deploy para Go WhatsApp API com PM2

set -e  # Exit on error

echo "===================================="
echo " Go WhatsApp API - Deploy Script"
echo "===================================="
echo ""

# Cores para output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Verificar se PM2 está instalado
echo "[1/5] Verificando dependências..."
if ! command -v pm2 &> /dev/null; then
    echo -e "${RED}[ERROR] PM2 não está instalado!${NC}"
    echo "Execute: npm install -g pm2"
    exit 1
fi

# Verificar se Go está instalado
if ! command -v go &> /dev/null; then
    echo -e "${RED}[ERROR] Go não está instalado!${NC}"
    exit 1
fi
echo -e "${GREEN}OK${NC}"

# Criar diretórios necessários
echo "[2/5] Criando diretórios..."
mkdir -p bin
mkdir -p logs
mkdir -p data
echo -e "${GREEN}OK${NC}"

# Compilar a aplicação
echo "[3/5] Compilando a aplicação..."
go build -o ./bin/server ./cmd/server/main.go
if [ $? -ne 0 ]; then
    echo -e "${RED}[ERROR] Falha ao compilar a aplicação!${NC}"
    exit 1
fi
echo -e "${GREEN}OK${NC}"

# Verificar se .env existe
echo "[4/5] Verificando configurações..."
if [ ! -f ".env" ]; then
    echo -e "${YELLOW}[WARNING] Arquivo .env não encontrado!${NC}"
    if [ -f ".env.example" ]; then
        echo "Copiando .env.example para .env..."
        cp .env.example .env
        echo -e "${YELLOW}[IMPORTANTE] Edite o arquivo .env com suas configurações!${NC}"
        read -p "Pressione ENTER para continuar..."
    else
        echo -e "${RED}[ERROR] Arquivo .env.example não encontrado!${NC}"
        exit 1
    fi
fi
echo -e "${GREEN}OK${NC}"

# Iniciar com PM2
echo "[5/5] Iniciando com PM2..."
pm2 start ecosystem.config.js
if [ $? -ne 0 ]; then
    echo -e "${RED}[ERROR] Falha ao iniciar com PM2!${NC}"
    exit 1
fi
echo -e "${GREEN}OK${NC}"

# Salvar configuração do PM2
echo "[6/6] Salvando configuração do PM2..."
pm2 save
echo -e "${GREEN}OK${NC}"

echo ""
echo "===================================="
echo " Deploy concluído com sucesso!"
echo "===================================="
echo ""
echo "Comandos úteis:"
echo "  pm2 status                   - Ver status"
echo "  pm2 logs go-whatsapp-api     - Ver logs"
echo "  pm2 monit                    - Monitorar"
echo "  pm2 restart go-whatsapp-api  - Reiniciar"
echo ""

# Mostrar status
pm2 status
