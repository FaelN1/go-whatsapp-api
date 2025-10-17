# Go WhatsApp API

Estrutura inicial de uma API REST para gerenciar instâncias WhatsApp via whatsmeow.

## Rotas

- POST /instances
- GET /instances
- DELETE /instances/{name}
- POST /messages/text
- POST /instances/{name}/logout
- POST /instances/{name}/connect (gera/retorna primeiro evento QR)
	- Para obter o QR atual: chamar repetidamente (poll) enquanto status for "code" ou "pending".

## Próximos Passos

- Integrar whatsmeow para criar sessão real e gerar QR Code / pairing code
- Persistência (SQLite usando sqlstore do whatsmeow)
- Swagger (usar swaggo)
- Testes unitários e integração
- Middlewares: autenticação por token da instância nas rotas de envio

## Status Atual

Implementado:

- Estrutura em camadas (controllers, services, repositories, platform, domain)
- Manager de sessões WhatsApp (in-memory) + bootstrap com sqlstore (SQLite) por instância
- Endpoint de criação de instância e listagem
- Endpoint de conexão que retorna evento inicial (QR code se novo login)
- Autenticação Bearer baseada no token salvo na criação da instância (aplicada às rotas de mensagens)
- Logging básico via utilitário do whatsmeow
- Configuração via variáveis de ambiente

Em progresso / Pendentes:

1. Envio real de mensagens texto (usar client.SendMessage com montagem do JID)
2. Envio de mídia (imagem / documento) com upload e mimetype detection
3. Webhooks (fila assíncrona + retry exponencial + assinatura HMAC opcional)
4. Persistência das instâncias (mover de repositório in-memory para SQLite, tabela Instances)
5. Atualização de status da instância (pending_qr, connected, disconnected, logged_out)
6. Suporte a pairing code (além de QR) se necessário
7. Swagger/OpenAPI servido em /docs (usar swaggo ou redoc)
8. Métricas Prometheus + Health detalhado (versão, uptime, instâncias ativas)
9. Observabilidade: tracing OpenTelemetry (HTTP + eventos WA)
10. Rate limiting (por token / IP) e circuit breaker para webhooks
1. Testes:

 - Unit: services, manager, bootstrap (mocks)
 - Integração: rotas HTTP (httptest) simulando fluxo instância

1. Harden / Segurança:

 - Guardar tokens usando hash
 - Limitar tamanho de uploads
 - Sanitizar logs

1. Rotina de reconexão configurável por instância

1. Estrutura de erros padronizada (código interno, mensagem, trace id)

1. Geração de client SDK (opcional) a partir do OpenAPI

## Variáveis de Ambiente

| Variável | Descrição | Default |
|----------|-----------|---------|
| HTTP_PORT | Porta HTTP | 8080 |
| APP_ENV | Ambiente (development/production) | development |
| DATA_DIR | Diretório para arquivos .db de cada instância | data |
| SWAGGER_ENABLE | Habilita docs (futuro) | true |
| WA_SKIP_CONNECT | Se true, pula tentativa de conectar automaticamente | false |

## Executando o Projeto

Windows (cmd):

```cmd
set HTTP_PORT=8080
set DATA_DIR=data
rem IMPORTANTE: Para SQLite (go-sqlite3) é necessário CGO habilitado e toolchain com um compilador C.
rem No Windows com Mingw ou LLVM (clang) configurado. Exemplo (PowerShell equivalente ajusta $env:CGO_ENABLED):
set CGO_ENABLED=1
rem Caso tenha instalado gcc via mingw64, aponte (se necessário):
rem set CC=x86_64-w64-mingw32-gcc
go run ./cmd/server
```

Build binário:

```cmd
set CGO_ENABLED=1
go build -o bin\server.exe ./cmd/server
bin\server.exe
```

## Fluxo Básico (Exemplos curl)

1. Criar instância:

```cmd
curl -X POST http://localhost:8080/instances ^
	-H "Content-Type: application/json" ^
	-d "{\"instanceName\":\"demo\",\"webhookUrl\":\"http://localhost:9000/webhook\",\"token\":\"t123\"}"
```

1. Iniciar conexão (gera QR se novo login):

```cmd
curl -X POST http://localhost:8080/instances/demo/connect
```

Resposta possível (exemplo primeiro evento):

```json
{"status":"code","code":"2@XXXXXXXXXXXX:XXXXXXXXXXXX"}
```

Renderize o código (ex: usando qrencode) para escanear no WhatsApp.

Polling para atualizar QR (em casos de expiração) ou status:

```cmd
curl -X POST http://localhost:8080/instances/demo/connect
```

Possíveis respostas:

```json
{"status":"code","code":"2@XXXX:YYYY"}   // QR atual
{"status":"pending"}                        // ainda aguardando evento (tente novamente)
{"status":"already_logged"}                 // sessão já autenticada
```

Observação: QR muda periodicamente; refaça a chamada para obter o novo code antes de expirar.

1. Enviar mensagem de texto (após conectado):

```cmd
curl -X POST http://localhost:8080/messages/text ^
	-H "Authorization: Bearer t123" ^
	-H "Content-Type: application/json" ^
	-d "{\"instanceId\":\"demo\",\"to\":\"5511999999999\",\"text\":\"Ola mundo\"}"
```

Observação: o campo `to` deve ser o número completo em formato internacional (sem '+'). A camada de envio ainda é stub e será integrada ao whatsmeow.

### Erro comum: "Binary was compiled with 'CGO_ENABLED=0'"

Esse erro indica que a dependência `github.com/mattn/go-sqlite3` não foi compilada porque CGO estava desabilitado. Solução:

```cmd
set CGO_ENABLED=1
go clean -cache
go build ./cmd/server
```

Certifique-se de possuir um compilador C instalado (mingw64, clang ou similar). No WSL/Linux basta garantir `build-essential` instalado.

1. Logout da instância:

```cmd
curl -X POST http://localhost:8080/instances/demo/logout
```

1. Deletar instância:

```cmd
curl -X DELETE http://localhost:8080/instances/demo
```text

## Estrutura de Pastas (resumo)

```
cmd/server            # main
internal/config       # carregamento de variáveis
internal/domain       # entidades de domínio
internal/app          # controllers, services, repositories
internal/platform     # integrações (whatsapp, http, middleware)
docs                  # documentação (README, OpenAPI)
pkg/logger            # logger wrapper
```

## Próximas Melhorias Técnicas

- Adicionar camada de DTOs separados das entidades
- Padrão de versionamento de API (/v1)
- Suporte a SSE ou WebSocket para eventos (QR updates, status)
- Feature flags / toggles por instância
- Scripts de migração (golang-migrate) para schema Instances

## Licença

Projeto inicial – ajuste conforme estratégia (MIT sugerido).
