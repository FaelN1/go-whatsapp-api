#!/bin/bash
# Script para reiniciar a aplicação

echo "Reiniciando Go WhatsApp API..."
pm2 restart go-whatsapp-api

echo ""
pm2 status
pm2 logs go-whatsapp-api --lines 20
