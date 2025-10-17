#!/bin/bash
# Script para ver status da aplicação

echo "Status do Go WhatsApp API:"
echo ""
pm2 status go-whatsapp-api

echo ""
echo "Informações detalhadas:"
pm2 info go-whatsapp-api
