#!/bin/bash
# Script para instalar PM2

echo "===================================="
echo " Instalação do PM2"
echo "===================================="
echo ""

# Cores
GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m'

# Verificar se Node.js está instalado
if ! command -v node &> /dev/null; then
    echo -e "${RED}[ERROR] Node.js não está instalado!${NC}"
    echo ""
    echo "Instale o Node.js:"
    echo "  Ubuntu/Debian: curl -fsSL https://deb.nodesource.com/setup_lts.x | sudo -E bash - && sudo apt-get install -y nodejs"
    echo "  CentOS/RHEL:   curl -fsSL https://rpm.nodesource.com/setup_lts.x | sudo bash - && sudo yum install -y nodejs"
    echo "  macOS:         brew install node"
    exit 1
fi

echo "Node.js encontrado:"
node --version
echo ""

# Verificar se npm está disponível
if ! command -v npm &> /dev/null; then
    echo -e "${RED}[ERROR] npm não está disponível!${NC}"
    exit 1
fi

echo "npm encontrado:"
npm --version
echo ""

# Instalar PM2 globalmente
echo "Instalando PM2..."
sudo npm install -g pm2

if [ $? -ne 0 ]; then
    echo ""
    echo -e "${RED}[ERROR] Falha ao instalar PM2!${NC}"
    exit 1
fi

echo ""
echo "===================================="
echo -e "${GREEN} PM2 instalado com sucesso!${NC}"
echo "===================================="
echo ""

# Verificar instalação
pm2 --version
echo ""

echo "Comandos úteis:"
echo "  pm2 --help              - Ajuda"
echo "  pm2 list                - Listar processos"
echo "  pm2 startup             - Configurar inicialização automática"
echo ""

# Sugestão de configurar startup
read -p "Deseja configurar PM2 para iniciar no boot? (s/n) " -n 1 -r
echo ""
if [[ $REPLY =~ ^[SsYy]$ ]]; then
    pm2 startup
    echo ""
    echo -e "${GREEN}Execute o comando acima (com sudo) e depois 'pm2 save'${NC}"
fi
