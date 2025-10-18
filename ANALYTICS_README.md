# Sistema de Rastreamento e Métricas de Mensagens

Este sistema permite rastrear mensagens enviadas via API (especialmente anúncios de comunidades) e coletar métricas detalhadas sobre visualizações e reações.

## Funcionalidades

### 1. Rastreamento Automático de Mensagens
- Todas as mensagens enviadas via `/instances/{name}/communities/{communityId}/announce` são automaticamente rastreadas
- Armazena informações como:
  - ID da mensagem
  - Instância que enviou
  - Comunidade de destino
  - Tipo de mensagem (texto, imagem, vídeo, etc.)
  - Conteúdo e mídia
  - Data e hora de envio

### 2. Captura de Visualizações
- Sistema captura automaticamente quando alguém visualiza uma mensagem rastreada
- Registra:
  - Quem visualizou (JID do usuário)
  - Nome do visualizador
  - Data e hora da visualização
  - Evita duplicatas (atualiza timestamp se usuário visualizar novamente)

### 3. Captura de Reações
- Sistema captura automaticamente reações enviadas às mensagens rastreadas
- Registra:
  - Quem reagiu (JID do usuário)
  - Nome do usuário
  - Emoji da reação
  - Data e hora da reação
  - Suporta atualização de reação (trocar emoji)
  - Suporta remoção de reação (emoji vazio)

## Configuração

### Passo 1: Criar as Tabelas no Banco de Dados

Execute o script SQL localizado em:
```
internal/platform/database/migrations/001_create_analytics_tables.sql
```

Ou execute manualmente:

```sql
-- Tabela de rastreamento de mensagens
CREATE TABLE IF NOT EXISTS message_tracking (
    id VARCHAR(255) PRIMARY KEY,
    instance_id VARCHAR(255) NOT NULL,
    message_id VARCHAR(255) NOT NULL,
    remote_jid VARCHAR(255) NOT NULL,
    community_jid VARCHAR(255),
    message_type VARCHAR(50) NOT NULL,
    content TEXT,
    media_url TEXT,
    caption TEXT,
    sent_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_message_tracking_instance ON message_tracking(instance_id);
CREATE INDEX idx_message_tracking_message_id ON message_tracking(message_id);
CREATE INDEX idx_message_tracking_sent_at ON message_tracking(sent_at DESC);

-- Tabela de visualizações
CREATE TABLE IF NOT EXISTS message_views (
    id VARCHAR(255) PRIMARY KEY,
    message_track_id VARCHAR(255) NOT NULL,
    viewer_jid VARCHAR(255) NOT NULL,
    viewer_name VARCHAR(255),
    viewed_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    FOREIGN KEY (message_track_id) REFERENCES message_tracking(id) ON DELETE CASCADE,
    UNIQUE (message_track_id, viewer_jid)
);

CREATE INDEX idx_message_views_track_id ON message_views(message_track_id);

-- Tabela de reações
CREATE TABLE IF NOT EXISTS message_reactions (
    id VARCHAR(255) PRIMARY KEY,
    message_track_id VARCHAR(255) NOT NULL,
    reactor_jid VARCHAR(255) NOT NULL,
    reactor_name VARCHAR(255),
    reaction VARCHAR(50) NOT NULL,
    reacted_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    FOREIGN KEY (message_track_id) REFERENCES message_tracking(id) ON DELETE CASCADE,
    UNIQUE (message_track_id, reactor_jid)
);

CREATE INDEX idx_message_reactions_track_id ON message_reactions(message_track_id);
```

### Passo 2: Configurar Postgres no .env

Certifique-se que seu `.env` está configurado para usar Postgres:

```env
DB_DRIVER=postgres
DATABASE_DSN=postgresql://user:password@localhost:5432/whatsapp_db?sslmode=disable
```

### Passo 3: Reiniciar a Aplicação

```bash
go run ./cmd/server/main.go
```

## Endpoints da API

### 1. Consultar Métricas de uma Mensagem Específica

```http
GET /analytics/messages/{trackId}/metrics
```

**Resposta:**
```json
{
  "message": {
    "id": "uuid-123",
    "instanceId": "my-instance",
    "messageId": "3EB0XXX",
    "remoteJid": "120363XXX@g.us",
    "communityJid": "120363YYY@g.us",
    "messageType": "conversation",
    "content": "Olá pessoal!",
    "sentAt": "2025-10-18T10:30:00Z"
  },
  "viewCount": 45,
  "reactionCount": 12,
  "views": [
    {
      "id": "view-1",
      "viewerJid": "5511999999999@s.whatsapp.net",
      "viewerName": "João Silva",
      "viewedAt": "2025-10-18T10:31:00Z"
    }
  ],
  "reactions": [
    {
      "id": "reaction-1",
      "reactorJid": "5511999999999@s.whatsapp.net",
      "reactorName": "João Silva",
      "reaction": "👍",
      "reactedAt": "2025-10-18T10:32:00Z"
    },
    {
      "id": "reaction-2",
      "reactorJid": "5511888888888@s.whatsapp.net",
      "reactorName": "Maria Santos",
      "reaction": "❤️",
      "reactedAt": "2025-10-18T10:35:00Z"
    }
  ]
}
```

### 2. Consultar Resumo de Métricas de uma Instância

```http
GET /analytics/instances/{instanceId}/metrics?limit=50&offset=0
```

**Parâmetros de Query:**
- `limit` (opcional): Número máximo de mensagens a retornar (padrão: 50)
- `offset` (opcional): Offset para paginação (padrão: 0)

**Resposta:**
```json
{
  "instanceId": "my-instance",
  "limit": 50,
  "offset": 0,
  "count": 10,
  "messages": [
    {
      "messageId": "3EB0XXX",
      "remoteJid": "120363XXX@g.us",
      "messageType": "imageMessage",
      "sentAt": "2025-10-18T10:30:00Z",
      "viewCount": 45,
      "reactionCount": 12,
      "topReactions": null
    },
    {
      "messageId": "3EB0YYY",
      "remoteJid": "120363XXX@g.us",
      "messageType": "conversation",
      "sentAt": "2025-10-17T15:20:00Z",
      "viewCount": 38,
      "reactionCount": 8,
      "topReactions": null
    }
  ]
}
```

## Exemplo de Uso

### 1. Enviar Anúncio para Comunidade

```bash
curl -X POST http://localhost:8080/instances/my-instance/communities/120363XXX@g.us/announce \
  -H "Content-Type: application/json" \
  -H "apikey: sua-chave-api" \
  -d '{
    "text": "Atenção membros! Importante anúncio.",
    "communities": []
  }'
```

**Resposta:**
```json
[
  {
    "communityJid": "120363XXX@g.us",
    "targetJid": "120363YYY-1234567890@g.us",
    "message": {
      "key": {
        "remoteJid": "120363YYY-1234567890@g.us",
        "fromMe": true,
        "id": "3EB0ABC123DEF456"
      },
      "messageType": "conversation",
      "messageTimestamp": 1729248600,
      "status": "PENDING"
    }
  }
]
```

### 2. Consultar Métricas do Anúncio

Primeiro, você precisa encontrar o `trackId` da mensagem. Você pode consultar as mensagens da instância:

```bash
curl http://localhost:8080/analytics/instances/my-instance/metrics?limit=10 \
  -H "apikey: sua-chave-api"
```

Depois, use o ID retornado para obter detalhes:

```bash
curl http://localhost:8080/analytics/messages/{trackId}/metrics \
  -H "apikey: sua-chave-api"
```

## Como Funciona Internamente

### Fluxo de Rastreamento

1. **Envio de Anúncio**
   - `CommunityController.SendAnnouncement()` recebe requisição
   - `CommunityService.SendAnnouncement()` envia mensagem via WhatsApp
   - Após envio bem-sucedido, chama `AnalyticsService.TrackSentMessage()`
   - Mensagem é salva na tabela `message_tracking`

2. **Captura de Visualizações**
   - WhatsApp envia evento `Receipt` quando alguém lê a mensagem
   - `SessionBootstrap` roteia evento para `MessageEventHandler.HandleReceipt()`
   - Handler chama `AnalyticsService.RecordMessageView()`
   - Visualização é salva em `message_views`

3. **Captura de Reações**
   - WhatsApp envia evento `Message` com tipo `reactionMessage`
   - `MessageEventHandler.HandleMessage()` detecta tipo de reação
   - Chama `ProcessReaction()` que invoca `AnalyticsService.RecordMessageReaction()`
   - Reação é salva em `message_reactions`

### Estrutura do Banco de Dados

```
message_tracking (mensagens rastreadas)
├─ message_views (visualizações)
└─ message_reactions (reações)
```

## Otimizações

- **Índices**: Criados em colunas frequentemente consultadas (instance_id, message_id, etc.)
- **Unique Constraints**: Previnem duplicatas de visualizações e reações do mesmo usuário
- **Foreign Keys com CASCADE**: Quando mensagem é deletada, views e reactions são removidas automaticamente
- **Paginação**: Endpoint de listagem suporta limit/offset para grandes volumes

## Limitações Conhecidas

1. **Apenas mensagens via /announce**: Atualmente só rastreia mensagens enviadas pelo endpoint de anúncios. Para rastrear outras mensagens, adicione chamadas similares em outros controllers.

2. **Visualizações no WhatsApp**: O WhatsApp só envia receipts se as configurações de privacidade permitirem. Usuários podem desabilitar confirmações de leitura.

3. **Performance**: Em grupos muito grandes (>1000 membros), pode haver delay na captura de todas as visualizações.

## Próximos Passos / Melhorias Futuras

- [ ] Dashboard web para visualização de métricas
- [ ] Exportação de relatórios em CSV/Excel
- [ ] Alertas quando mensagem atingir X visualizações
- [ ] Análise de engajamento (taxa de visualização, tempo médio até primeira reação, etc.)
- [ ] Suporte para rastreamento de mensagens diretas (não apenas anúncios)
- [ ] GraphQL API para consultas mais flexíveis
