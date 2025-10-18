# Guia de Execução de Migrations

## 📋 Pré-requisitos

1. **PostgreSQL Client instalado**
   - Windows: Instale o PostgreSQL (vem com `psql`)
   - Linux: `sudo apt-get install postgresql-client`
   - Mac: `brew install postgresql`

2. **Banco de dados PostgreSQL criado**
   ```sql
   CREATE DATABASE whatsapp_api;
   ```

3. **Arquivo .env configurado**
   ```env
   DB_DRIVER=postgres
   DATABASE_DSN=postgresql://user:password@localhost:5432/whatsapp_api?sslmode=disable
   ```

## 🚀 Métodos de Execução

### Método 1: Script Automatizado (Recomendado)

#### Windows:
```cmd
cd d:\work\go-whatsapp-api
scripts\windows\migrate.bat
```

#### Linux/Mac:
```bash
cd /path/to/go-whatsapp-api
chmod +x scripts/linux/migrate.sh
./scripts/linux/migrate.sh
```

O script irá:
1. Ler configurações do `.env`
2. Listar todos os arquivos SQL em `internal/platform/database/migrations/`
3. Solicitar confirmação
4. Executar cada migration em ordem
5. Reportar sucesso ou erro

---

### Método 2: psql Direto

Se você preferir executar manualmente:

```bash
# Substitua pelos seus dados de conexão
export DATABASE_DSN="postgresql://user:password@localhost:5432/whatsapp_api?sslmode=disable"

# Execute a migration
psql "$DATABASE_DSN" -f internal/platform/database/migrations/001_create_analytics_tables.sql
```

Ou no Windows (PowerShell):
```powershell
$env:DATABASE_DSN = "postgresql://user:password@localhost:5432/whatsapp_api?sslmode=disable"
psql $env:DATABASE_DSN -f internal\platform\database\migrations\001_create_analytics_tables.sql
```

---

### Método 3: Via DBeaver/pgAdmin (GUI)

1. Conecte-se ao banco de dados PostgreSQL
2. Abra o arquivo SQL: `internal/platform/database/migrations/001_create_analytics_tables.sql`
3. Execute o script SQL

---

### Método 4: Docker (se estiver usando)

```bash
# Copiar arquivo SQL para o container
docker cp internal/platform/database/migrations/001_create_analytics_tables.sql postgres_container:/tmp/

# Executar no container
docker exec -i postgres_container psql -U username -d whatsapp_api -f /tmp/001_create_analytics_tables.sql
```

---

## ✅ Verificar se Migrations foram Aplicadas

Após executar, verifique se as tabelas foram criadas:

```sql
-- Liste todas as tabelas
\dt

-- Ou via SQL
SELECT table_name 
FROM information_schema.tables 
WHERE table_schema = 'public'
  AND table_name LIKE 'message_%';
```

Você deve ver:
- `message_tracking`
- `message_views`
- `message_reactions`
- `message_metrics_summary` (view)

### Verificar estrutura de uma tabela:
```sql
\d message_tracking
```

### Verificar índices:
```sql
SELECT indexname, indexdef 
FROM pg_indexes 
WHERE tablename = 'message_tracking';
```

---

## 🔄 Rollback (Reverter Migrations)

Se precisar desfazer as migrations, crie um arquivo de rollback:

```sql
-- internal/platform/database/migrations/001_rollback_analytics_tables.sql

DROP VIEW IF EXISTS message_metrics_summary;
DROP TABLE IF EXISTS message_reactions;
DROP TABLE IF EXISTS message_views;
DROP TABLE IF EXISTS message_tracking;
```

Execute:
```bash
psql "$DATABASE_DSN" -f internal/platform/database/migrations/001_rollback_analytics_tables.sql
```

---

## 🐛 Troubleshooting

### Erro: "psql: command not found"

**Solução (Windows):**
1. Adicione o PostgreSQL ao PATH:
   ```
   C:\Program Files\PostgreSQL\15\bin
   ```
2. Reinicie o terminal

**Solução (Linux):**
```bash
sudo apt-get install postgresql-client
```

---

### Erro: "connection refused"

Verifique se:
1. PostgreSQL está rodando: `pg_ctl status` (Windows) ou `sudo systemctl status postgresql` (Linux)
2. Porta 5432 está aberta
3. Credenciais no `.env` estão corretas

---

### Erro: "permission denied"

**No Linux, dê permissão ao script:**
```bash
chmod +x scripts/linux/migrate.sh
```

---

### Erro: "database does not exist"

Crie o banco primeiro:
```sql
psql -U postgres
CREATE DATABASE whatsapp_api;
\q
```

---

### Erro: "relation already exists"

As tabelas já existem. Opções:

1. **Ignorar** (migration usa `CREATE TABLE IF NOT EXISTS`)
2. **Dropar e recriar:**
   ```sql
   DROP TABLE IF EXISTS message_reactions CASCADE;
   DROP TABLE IF EXISTS message_views CASCADE;
   DROP TABLE IF EXISTS message_tracking CASCADE;
   ```
   Depois execute a migration novamente.

---

## 📊 Verificar Dados Após Migrations

```sql
-- Ver quantas mensagens rastreadas
SELECT COUNT(*) FROM message_tracking;

-- Ver mensagens com mais visualizações
SELECT message_id, COUNT(*) as views
FROM message_views
GROUP BY message_id
ORDER BY views DESC
LIMIT 10;

-- Ver reações mais usadas
SELECT reaction, COUNT(*) as count
FROM message_reactions
GROUP BY reaction
ORDER BY count DESC;

-- Usar a view agregada
SELECT * FROM message_metrics_summary
ORDER BY sent_at DESC
LIMIT 10;
```

---

## 🎯 Exemplo Completo (Fluxo Rápido)

```bash
# 1. Clone/navegue para o projeto
cd d:\work\go-whatsapp-api

# 2. Configure o .env
echo DATABASE_DSN=postgresql://postgres:senha@localhost:5432/whatsapp_api >> .env
echo DB_DRIVER=postgres >> .env

# 3. Crie o banco (se necessário)
psql -U postgres -c "CREATE DATABASE whatsapp_api;"

# 4. Execute as migrations
scripts\windows\migrate.bat

# 5. Verifique
psql postgresql://postgres:senha@localhost:5432/whatsapp_api -c "\dt"
```

---

## 📝 Notas Importantes

1. **Backup**: Sempre faça backup antes de executar migrations em produção
   ```bash
   pg_dump -U postgres whatsapp_api > backup_$(date +%Y%m%d).sql
   ```

2. **Idempotência**: As migrations usam `IF NOT EXISTS`, então são seguras para executar múltiplas vezes

3. **Ordem**: Os arquivos SQL devem ter prefixo numérico (001_, 002_, etc.) para manter ordem de execução

4. **Testing**: Teste sempre em ambiente de desenvolvimento primeiro

---

## 🔧 Próximos Passos

Após executar as migrations:

1. **Reinicie a aplicação** para que ela use as novas tabelas
2. **Envie um anúncio teste** via API
3. **Verifique se está sendo rastreado:**
   ```sql
   SELECT * FROM message_tracking ORDER BY created_at DESC LIMIT 1;
   ```
4. **Consulte as métricas** via API:
   ```bash
   curl http://localhost:8080/analytics/instances/sua-instancia/metrics
   ```

---

## 🆘 Suporte

Se encontrar problemas:
1. Verifique os logs da aplicação
2. Verifique os logs do PostgreSQL
3. Consulte a documentação completa em `ANALYTICS_README.md`
