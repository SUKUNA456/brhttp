# brhttp — Servidor de Desenvolvimento Web de Alta Performance

<p align="left">
  <img src="https://img.shields.io/badge/versão-v1.8-blue.svg" alt="Versão" />
  <img src="https://img.shields.io/badge/licença-GPL--3.0-blue.svg" alt="Licença" />
  <img src="https://img.shields.io/badge/Go-1.18%2B-cyan.svg" alt="Go Version" />
  <img src="https://img.shields.io/badge/plataforma-Linux-blue.svg" alt="Plataforma" />
</p>

## 1. Introdução

**brhttp** é um servidor de desenvolvimento local de alta performance, escrito em Go. Projetado para eficiência e flexibilidade, ele opera como um binário único sem dependências externas, oferecendo uma suíte de ferramentas robusta para acelerar o fluxo de trabalho de desenvolvimento web moderno.

O sistema é "zero-config" por padrão, mas permite customização extensiva através de flags de linha de comando e um arquivo de configuração em formato JSON, suportando desde o serviço de arquivos estáticos simples até arquiteturas complexas com automação de build e proxy reverso.

## 2. Funcionalidades Principais (Versão 1.8)

-   **Live Reload com HMR (Hot Module Replacement):** Utiliza WebSockets para monitorar o sistema de arquivos e notificar o cliente. Realiza recarregamento total para alterações em HTML e atualizações parciais (injeção de CSS e recarregamento de scripts JS) sem um refresh completo da página.
-   **Servidor Estático Configurável:** Serve arquivos de um diretório especificado com controle sobre listagem de diretórios e páginas de erro 404 customizadas.
-   **Proxy Reverso (Reverse Proxy):** Redireciona requisições de um determinado path (ex: `/api`) para um servidor de backend. Essencial para contornar políticas de CORS e integrar aplicações front-end com APIs durante o desenvolvimento.
-   **Roteamento Avançado:** Suporta reescrita de URL (server-side) e redirecionamentos HTTP com códigos de status customizáveis (ex: 301, 302), permitindo a simulação de arquiteturas de produção.
-   **Automação via Webhooks:**
    -   **Webhooks de Comando:** Executa comandos de terminal em eventos do ciclo de vida do servidor (`server_start`, `server_stop`) ou em modificações de arquivos (`file_change`). Permite a orquestração de ferramentas de build como compiladores Sass/TypeScript, bundlers, etc.
    -   **Webhooks de Notificação:** Envia uma carga útil (payload) JSON via POST para um endpoint externo em cada modificação de arquivo.
-   **API de Gerenciamento Remoto:** Expõe uma API REST (`/api/*`) protegida por token para controle programático do servidor, permitindo disparar reloads, executar comandos e verificar o status da instância.
-   **Cadeia de Middlewares:** Inclui middlewares para compressão Gzip, tratamento de CORS, desabilitação de cache e logging de requisições.



## 3. Instalação e Execução


### 3.1. Pré-requisitos
-   Go versão 1.18 ou superior.

### 3.2. Instalação

Clone o repositório e instale as dependências do módulo:

```bash
git clone [https://github.com/henriquetourinho/brhttp.git](https://github.com/henriquetourinho/brhttp.git)
cd brhttp
go mod tidy
````

### 3.3. Execução

O servidor pode ser iniciado de três maneiras principais, com a seguinte ordem de precedência para configurações: **Flags \> Arquivo JSON \> Padrões**.

#### 3.3.1. Modo Padrão (Zero-Config)

Executa o servidor com as configurações padrão (servindo o diretório `./www` na porta `5571`).

```bash
go run main.go
```

#### 3.3.2. Via Flags de Linha de Comando

Permite a customização de parâmetros específicos.

```bash
# Exemplo: servir o diretório 'dist' na porta 8080, com fallback de SPA e Gzip
go run main.go --dir=dist --port=8080 --spa-fallback --enable-gzip
```

**Flags Disponíveis:**

| Flag | Descrição | Padrão |
| :--- | :--- | :--- |
| `--port` | Porta de escuta do servidor HTTP. | `5571` |
| `--dir` | Diretório raiz a ser servido. | `www` |
| `--config` | Caminho para o arquivo de configuração `config.json`. | `""` |
| `--spa-fallback` | Habilita o fallback para `index.html` em rotas não encontradas. | `false` |
| `--enable-gzip` | Habilita a compressão Gzip para as respostas. | `false` |
| `--enable-dir-listing` | Permite a listagem de conteúdo de diretórios. | `false` |
| `--inject-js` | Injeta um arquivo JavaScript em todas as páginas HTML. | `""` |
| `--inject-css` | Injeta um arquivo CSS em todas as páginas HTML. | `""` |
| `--404-page` | Caminho para uma página de erro 404 personalizada. | `""` |
| `--log-file` | Caminho para o arquivo de log. | `server.log` |
| `--api-token` | Token de autenticação "Bearer" para a API de gerenciamento. | `""` |
| `--notification-webhook-url` | URL para webhooks de notificação de mudança. | `""` |
| `--watch-debounce-ms` | Tempo de espera (ms) para o watcher após uma mudança. | `100` |
| `--watch-exclude-dirs` | Diretórios a excluir do watcher (separados por vírgula). | `""` |

#### 3.3.3. Via Arquivo de Configuração

Para configurações complexas, especialmente `proxy_rules` e `command_webhooks`, utilize um arquivo `config.json`.

```bash
go run main.go --config config.json
```

**Exemplo de `config.json`:**

```json
{
  "port": 5571,
  "serve_dir": "public",
  "spa_fallback_enabled": true,
  "gzip_enabled": true,
  "log_file_path": "brhttp.log",
  "api_token": "seu-token-secreto-aqui-jwt-ou-similar",
  "watch_debounce_ms": 150,
  "watch_exclude_dirs": ["node_modules", ".git", "dist"],
  "proxy_rules": [
    {
      "path": "/api/v1",
      "target": "http://localhost:3000"
    }
  ],
  "redirects": [
    {
      "from": "/documentacao-antiga",
      "to": "/docs/v2",
      "code": 301
    }
  ],
  "command_webhooks": [
    {
      "event": "server_start",
      "command": "npm",
      "args": ["run", "watch-css"]
    },
    {
      "event": "file_change",
      "path": "src/ts",
      "command": "npm",
      "args": ["run", "build-ts"]
    }
  ]
}
```

## 4\. API de Gerenciamento

O servidor expõe uma API REST para gerenciamento programático. Requer a configuração de um `api_token` e o uso do cabeçalho `Authorization: Bearer <token>`.

#### 4.1. `GET /api/status`

Retorna o estado atual do servidor.

```bash
curl http://localhost:5571/api/status \
  -H "Authorization: Bearer seu-token-secreto-aqui-jwt-ou-similar"
```

#### 4.2. `POST /api/reload`

Dispara um evento de live-reload para todos os clientes conectados.

```bash
curl -X POST http://localhost:5571/api/reload \
  -H "Authorization: Bearer seu-token-secreto-aqui-jwt-ou-similar"
```

#### 4.3. `POST /api/command`

Executa um comando no sistema operacional do servidor.

```bash
curl -X POST http://localhost:5571/api/command \
  -H "Authorization: Bearer seu-token-secreto-aqui-jwt-ou-similar" \
  -H "Content-Type: application/json" \
  -d '{"command": "git", "args": ["pull"]}'
```

## 5\. Arquitetura Interna

O `brhttp` é construído sobre o pacote `net/http` padrão do Go. As requisições passam por uma cadeia de middlewares configurável cuja ordem de execução é: logging, gzip, cache-control, CORS, reescrita/redirecionamento, proxy reverso, injeção de código, fallback de SPA e, finalmente, o handler de arquivos estáticos. O monitoramento de arquivos é realizado pela biblioteca `fsnotify`, e a comunicação em tempo real para o Live Reload é gerenciada por um pool de conexões WebSocket baseado em `gorilla/websocket`. A camada de automação intercepta eventos do watcher e do ciclo de vida do servidor para disparar os webhooks configurados.

## 6\. Limitações

Este projeto foi desenhado como uma ferramenta de desenvolvimento e não é recomendado para ambientes de produção sem um proxy reverso robusto (como Nginx ou Caddy) à sua frente. As principais limitações intencionais são:

  - **Ausência de HTTPS nativo:** Não implementa TLS.
  - **Monousuário:** Não possui um sistema de autenticação de usuários para o conteúdo servido.
  - **Logs Simples:** O logging em arquivo não inclui rotação automática.

---


## 🔄 Evolução do brhttp: v1.5 vs. Anteriores

A tabela abaixo detalha a evolução do projeto, desde um servidor puro até uma suíte de desenvolvimento local.

| Característica | v1.5 (Atual - Dev Server) | v1.4 (WebSockets) | v1.3 (SSE) | v1.0 (Inicial) |
| :--- | :--- | :--- | :--- | :--- |
| **Live Reload** | ✅ **Sim, avançado (HMR)** | ✅ Sim, robusto | ✅ Sim, funcional | ❌ Não |
| **Tecnologia** | WebSockets (com HMR) | WebSockets | Server-Sent Events (SSE) | Nenhuma |
| **Configuração** | **Flags e arquivo JSON** | Nenhuma | Nenhuma | Nenhuma |
| **Foco Principal** | **Dev local avançado** | Dev local (robusto) | Dev local (básico) | Servidor estático puro |
| **Middlewares** | `logging`, `noCache`, `cors`, `gzip`, `proxy`, `rewrite`, `spa`, `custom404`, `injector` | `logging`, `noCache`, `liveReloadInjector` | `logging`, `noCache`, `liveReloadInjector` | `logging`, `noDirListing` |
| **Funcionalidades** | **Reverse Proxy, SPA, Gzip, Rewrites, CORS, Injeção de código** | Servidor estático | Servidor estático | Servidor estático |
| **Dependências** | `fsnotify`, `gorilla/websocket` | `fsnotify`, `gorilla/websocket` | `fsnotify` | Nenhuma |

**Vantagem da v1.5:** A versão 1.5 transforma o `brhttp` em uma ferramenta de desenvolvimento completa, rivalizando com soluções como `live-server` do Node.js, mas com a performance e simplicidade de um binário Go. Ele resolve problemas comuns de desenvolvimento, como proxy de API e roteamento de SPA.



## 🤝 Apoie o projeto

Se o **brhttp** foi útil, ajude a manter o desenvolvimento:

**Chave Pix:**
```
poupanca@henriquetourinho.com.br
```

---

## 📄 Licença

Distribuído sob a licença **GPL-3.0** — consulte o arquivo `LICENSE` para detalhes.

---

## 🙋‍♂️ Desenvolvido por

**Carlos Henrique Tourinho Santana** — Salvador, Bahia, Brasil
<br>
🔗 [Wiki Debian](https://wiki.debian.org/henriquetourinho)
<br>
🔗 [LinkedIn](https://br.linkedin.com/in/carloshenriquetourinhosantana)
<br>
🔗 [GitHub](https://github.com/henriquetourinho)
