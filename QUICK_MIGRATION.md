# Como Executar as Migrations - Guia Rápido

## Opção 1: Usando Script Automático (Mais Fácil) ⭐

### Windows
```cmd
cd d:\work\go-whatsapp-api
scripts\windows\migrate.bat
```

### Linux/Mac
```bash
cd /caminho/para/go-whatsapp-api
chmod +x scripts/linux/migrate.sh
./scripts/linux/migrate.sh
```

---

## Opção 2: Manualmente com psql

### Windows (PowerShell)
```powershell
cd d:\work\go-whatsapp-api

# Configure sua connection string
$env:DATABASE_DSN = "postgresql://postgres:suasenha@localhost:5432/whatsapp_api?sslmode=disable"

# Execute a migration
psql $env:DATABASE_DSN -f internal\platform\database\migrations\001_create_analytics_tables.sql
```

### Linux/Mac
```bash
cd /caminho/para/go-whatsapp-api

# Configure sua connection string
export DATABASE_DSN="postgresql://postgres:suasenha@localhost:5432/whatsapp_api?sslmode=disable"

# Execute a migration
psql "$DATABASE_DSN" -f internal/platform/database/migrations/001_create_analytics_tables.sql
```

---

## Opção 3: Copiar e Colar no pgAdmin/DBeaver

1. Abra o pgAdmin ou DBeaver
2. Conecte ao banco `whatsapp_api`
3. Abra o arquivo: `internal/platform/database/migrations/001_create_analytics_tables.sql`
4. Execute todo o conteúdo do arquivo (Ctrl+Enter ou botão "Run")

---

## Verificar se Funcionou

Conecte ao banco e execute:

```sql
-- Ver tabelas criadas
SELECT table_name 
FROM information_schema.tables 
WHERE table_schema = 'public' 
  AND table_name LIKE 'message_%';

-- Resultado esperado:
-- message_tracking
-- message_views  
-- message_reactions
-- message_metrics_summary
```

---

## Problemas Comuns

### "psql: command not found"
- **Windows**: Adicione `C:\Program Files\PostgreSQL\15\bin` ao PATH
- **Linux**: `sudo apt-get install postgresql-client`
- **Mac**: `brew install postgresql`

### "database does not exist"
Crie o banco primeiro:
```sql
CREATE DATABASE whatsapp_api;
```

### "permission denied" (Linux)
```bash
chmod +x scripts/linux/migrate.sh
```

---

## Pronto!

Após executar a migration com sucesso:
1. Reinicie a aplicação Go
2. As tabelas estarão prontas para rastrear mensagens automaticamente
3. Use os endpoints `/analytics/*` para consultar métricas

Para mais detalhes, veja: `MIGRATION_GUIDE.md`
