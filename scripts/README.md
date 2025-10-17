# Scripts de Gerenciamento

Esta pasta contém scripts para facilitar o deploy e gerenciamento da aplicação Go WhatsApp API com PM2.

## 📁 Estrutura

```
scripts/
├── windows/          # Scripts para Windows
│   ├── deploy.bat    # Deploy inicial
│   ├── update.bat    # Atualizar aplicação
│   ├── restart.bat   # Reiniciar
│   ├── stop.bat      # Parar
│   ├── status.bat    # Ver status
│   └── logs.bat      # Ver logs
└── linux/            # Scripts para Linux
    ├── deploy.sh     # Deploy inicial
    ├── update.sh     # Atualizar aplicação
    ├── restart.sh    # Reiniciar
    ├── stop.sh       # Parar
    ├── status.sh     # Ver status
    └── logs.sh       # Ver logs
```

## 🪟 Windows

### Primeiro Deploy
```cmd
cd d:\work\go-whatsapp-api
scripts\windows\deploy.bat
```

### Atualizar Aplicação
```cmd
scripts\windows\update.bat
```

### Gerenciamento Diário
```cmd
scripts\windows\status.bat   # Ver status
scripts\windows\restart.bat  # Reiniciar
scripts\windows\stop.bat     # Parar
scripts\windows\logs.bat     # Ver logs (padrão: 50 linhas)
scripts\windows\logs.bat 100 # Ver últimas 100 linhas
```

## 🐧 Linux

### Tornar Scripts Executáveis (primeira vez)
```bash
chmod +x scripts/linux/*.sh
```

### Primeiro Deploy
```bash
cd /path/to/go-whatsapp-api
./scripts/linux/deploy.sh
```

### Atualizar Aplicação
```bash
./scripts/linux/update.sh
```

### Gerenciamento Diário
```bash
./scripts/linux/status.sh      # Ver status
./scripts/linux/restart.sh     # Reiniciar
./scripts/linux/stop.sh        # Parar
./scripts/linux/logs.sh        # Ver logs (padrão: 50 linhas)
./scripts/linux/logs.sh 100    # Ver últimas 100 linhas
```

## 📋 Descrição dos Scripts

### deploy (Windows/Linux)
- ✅ Verifica dependências (PM2 e Go)
- ✅ Cria diretórios necessários
- ✅ Compila a aplicação
- ✅ Configura arquivo .env (se não existir)
- ✅ Inicia aplicação com PM2
- ✅ Salva configuração do PM2

**Quando usar**: Primeira instalação ou após reset completo

### update (Windows/Linux)
- ✅ Para a aplicação
- ✅ Recompila o código atualizado
- ✅ Reinicia aplicação
- ✅ Rollback automático em caso de erro de compilação

**Quando usar**: Após atualizar o código (git pull, alterações, etc)

### restart (Windows/Linux)
- ✅ Reinicia a aplicação rapidamente
- ✅ Mostra status e logs recentes

**Quando usar**: Após alterar configurações (.env), reiniciar após erro, etc

### stop (Windows/Linux)
- ✅ Para a aplicação
- ✅ Mostra status atualizado

**Quando usar**: Manutenção, debug, ou antes de fazer alterações manuais

### status (Windows/Linux)
- ✅ Mostra status resumido
- ✅ Mostra informações detalhadas (uptime, memória, CPU, etc)

**Quando usar**: Verificar se a aplicação está rodando, ver uso de recursos

### logs (Windows/Linux)
- ✅ Mostra logs em tempo real
- ✅ Permite especificar número de linhas históricas
- ✅ Use Ctrl+C para sair

**Quando usar**: Debug, monitoramento, verificar erros

## 🔧 Configuração do PM2

O arquivo `ecosystem.config.js` na raiz do projeto controla:
- Nome da aplicação
- Número de instâncias
- Limites de memória
- Configuração de logs
- Auto-restart
- Variáveis de ambiente

## 📝 Notas Importantes

### Windows
- Os scripts `.bat` pausam ao final para você ver o resultado
- Use `Ctrl+C` para sair de logs em tempo real

### Linux
- Scripts `.sh` precisam de permissão de execução (`chmod +x`)
- Use `Ctrl+C` para sair de logs em tempo real
- Scripts usam cores para melhor visualização

## 🚀 Workflow Recomendado

### Deploy Inicial
1. Configure o `.env` com suas credenciais
2. Execute `deploy.bat` (Windows) ou `deploy.sh` (Linux)
3. Verifique status com `status.bat/sh`
4. Monitore logs com `logs.bat/sh`

### Atualização de Código
1. Atualize o código (git pull, ou manual)
2. Execute `update.bat/sh`
3. Verifique logs para garantir que iniciou corretamente

### Monitoramento Regular
```bash
# Ver status geral
pm2 status

# Monitorar em tempo real
pm2 monit

# Ver logs
./scripts/linux/logs.sh   # ou scripts\windows\logs.bat
```

## 🆘 Troubleshooting

### Script não executa
**Windows**: Verifique se está executando como administrador
**Linux**: Verifique permissões (`chmod +x script.sh`)

### PM2 não encontrado
Execute: `npm install -g pm2`

### Go não encontrado
Instale o Go e adicione ao PATH

### Erro de compilação
- Verifique se o `go.mod` está atualizado
- Execute `go mod tidy`
- Verifique erros de sintaxe no código

### Aplicação não inicia
- Verifique `.env` está configurado corretamente
- Verifique se a porta está disponível
- Verifique logs: `pm2 logs go-whatsapp-api`
- Verifique banco de dados está acessível

## 📚 Recursos Adicionais

- [Documentação PM2](https://pm2.keymetrics.io/)
- [Guia de Produção](../README.production.md)
- [Documentação do Projeto](../README.md)

## 💡 Dicas

1. **Backup antes de atualizar**: Sempre faça backup do diretório `data/` antes de atualizar
2. **Monitore recursos**: Use `pm2 monit` para ver uso de CPU/memória em tempo real
3. **Configure startup**: Use `pm2 startup` e `pm2 save` para iniciar automaticamente no boot
4. **Logs rotativos**: PM2 já rotaciona logs automaticamente
5. **Ambiente de teste**: Teste atualizações em ambiente de desenvolvimento primeiro
