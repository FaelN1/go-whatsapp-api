#!/bin/bash
# Script para ver logs da aplicação

# Verificar se foi passado argumento para número de linhas
LINES=${1:-50}

echo "Mostrando últimas $LINES linhas de log..."
echo "Use Ctrl+C para sair do modo live"
echo ""

pm2 logs go-whatsapp-api --lines $LINES
