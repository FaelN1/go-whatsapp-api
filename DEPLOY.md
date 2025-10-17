# 🚀 Guia Rápido de Deploy

Este guia rápido mostra como colocar a aplicação em produção usando PM2.

## 📋 Pré-requisitos

- **Go 1.24+** instalado
- **Node.js e npm** instalados
- **PM2** instalado: `npm install -g pm2`
- **PostgreSQL** rodando (ou use docker-compose)
- **MinIO** rodando (opcional)

## ⚡ Deploy Rápido

### Windows

```cmd
# 1. Clone o repositório
git clone <repository-url>
cd go-whatsapp-api

# 2. Configure o ambiente
copy .env.example .env
# Edite o .env com suas configurações

# 3. Execute o deploy
scripts\windows\deploy.bat
```

### Linux

```bash
# 1. Clone o repositório
git clone <repository-url>
cd go-whatsapp-api

# 2. Torne os scripts executáveis
chmod +x scripts/linux/*.sh

# 3. Configure o ambiente
cp .env.example .env
# Edite o .env com suas configurações

# 4. Execute o deploy
./scripts/linux/deploy.sh
```

## 🎯 Gerenciamento Rápido

### Windows
```cmd
scripts\windows\status.bat    # Ver status
scripts\windows\restart.bat   # Reiniciar
scripts\windows\logs.bat      # Ver logs
scripts\windows\update.bat    # Atualizar código
```

### Linux
```bash
./scripts/linux/status.sh     # Ver status
./scripts/linux/restart.sh    # Reiniciar
./scripts/linux/logs.sh       # Ver logs
./scripts/linux/update.sh     # Atualizar código
```

## 📦 Usando Docker Compose

```bash
# Iniciar serviços (PostgreSQL, MinIO, Redis)
docker-compose up -d

# Verificar status
docker-compose ps

# Ver logs
docker-compose logs -f
```

## 🔧 Comandos PM2 Essenciais

```bash
pm2 status                    # Ver todas as aplicações
pm2 logs go-whatsapp-api      # Ver logs em tempo real
pm2 monit                     # Dashboard de monitoramento
pm2 restart go-whatsapp-api   # Reiniciar aplicação
pm2 stop go-whatsapp-api      # Parar aplicação
pm2 delete go-whatsapp-api    # Remover do PM2
```

## 📚 Documentação Completa

- [README de Produção](README.production.md) - Guia completo de produção
- [Scripts README](scripts/README.md) - Documentação dos scripts
- [OpenAPI Documentation](docs/openapi.yaml) - Documentação da API

## 🆘 Problemas Comuns

### Aplicação não inicia
```bash
# Verifique os logs
pm2 logs go-whatsapp-api

# Verifique se o binário existe
ls -la bin/  # Linux
dir bin\     # Windows

# Verifique conexão com banco
# Certifique-se que PostgreSQL está rodando e acessível
```

### Porta em uso
```bash
# Altere a porta no arquivo .env
HTTP_PORT=8081
```

### Erro de compilação
```bash
# Atualize dependências
go mod tidy

# Compile manualmente para ver erros
go build -o ./bin/server ./cmd/server/main.go
```

## 🔐 Segurança em Produção

1. **Use HTTPS** - Configure um reverse proxy (Nginx/Caddy)
2. **Token forte** - Altere `MASTER_TOKEN` no .env
3. **Firewall** - Configure adequadamente
4. **Backups** - Faça backup do diretório `data/`
5. **Atualize** - Mantenha dependências atualizadas

## 🌐 Exemplo de Nginx

```nginx
server {
    listen 80;
    server_name your-domain.com;

    location / {
        proxy_pass http://localhost:8080;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection 'upgrade';
        proxy_set_header Host $host;
        proxy_cache_bypass $http_upgrade;
    }
}
```

## 📊 Monitoramento

### PM2 Monit
```bash
pm2 monit  # Dashboard interativo
```

### Logs Estruturados
```bash
# Ver erros específicos
pm2 logs go-whatsapp-api --err

# Ver apenas output
pm2 logs go-whatsapp-api --out

# Últimas 100 linhas
pm2 logs go-whatsapp-api --lines 100
```

## 🔄 Workflow de Atualização

```bash
# 1. Atualizar código
git pull

# 2. Executar script de update
# Windows:
scripts\windows\update.bat

# Linux:
./scripts/linux/update.sh

# 3. Verificar logs
pm2 logs go-whatsapp-api --lines 50
```

## 💾 Backup e Restore

### Backup
```bash
# Backup das sessões do WhatsApp
tar -czf backup-data-$(date +%Y%m%d).tar.gz data/

# Backup do banco de dados (PostgreSQL)
pg_dump -U whatsapp whatsapp > backup-db-$(date +%Y%m%d).sql
```

### Restore
```bash
# Restore das sessões
tar -xzf backup-data-YYYYMMDD.tar.gz

# Restore do banco
psql -U whatsapp whatsapp < backup-db-YYYYMMDD.sql
```

## 🎓 Próximos Passos

1. ✅ Configure domínio e SSL
2. ✅ Configure backups automáticos
3. ✅ Configure monitoramento (PM2 Plus, ou outro)
4. ✅ Configure alertas
5. ✅ Documente seu setup específico

---

Para mais detalhes, consulte o [README de Produção](README.production.md) completo.
