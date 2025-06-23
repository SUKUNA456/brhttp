# brhttp — Servidor Web de Desenvolvimento Rápido em Go

<p align="left">
  <img src="https://img.shields.io/badge/versão-v1.5-blue.svg" alt="Versão" />
  <img src="https://img.shields.io/badge/licença-GPL3-blue.svg" alt="Licença" />
  <img src="https://img.shields.io/badge/Go-1.18%2B-cyan.svg" alt="Go Version" />
  <img src="https://img.shields.io/badge/plataforma-Linux-blue.svg" alt="Plataforma" />
  <img src="https://img.shields.io/badge/feito_no-Brasil-blue.svg" alt="Feito no Brasil" />
</p>

---

## 🚀 O que é o brhttp?

**brhttp** é um servidor web de alta performance escrito em **Go**. Originalmente um servidor estático minimalista, ele evoluiu para uma poderosa ferramenta de desenvolvimento local. É "zero-config" por padrão, mas altamente configurável via flags ou arquivo JSON.

Ele é ideal para desenvolvedores que precisam de:

-   Live Reload avançado com HMR para CSS/JS.
-   Um servidor estático rápido e configurável.
-   Funcionalidades de Reverse Proxy para APIs de backend.
-   Suporte para Single-Page Applications (SPA).
-   Deploy rápido com um único binário.

---

## ⚡ Principais Características (v1.5)

-   **Live Reload Avançado:** Utiliza WebSockets para recarregar a página (HTML) ou injetar mudanças sem recarregar (HMR para CSS e JS).
-   **Altamente Configurável:** Controle portas, diretórios, e funcionalidades via flags de linha de comando ou um arquivo `config.json`.
-   **Reverse Proxy:** Redirecione chamadas de API (ex: `/api/*`) para seu servidor de backend, evitando problemas com CORS.
-   **Suporte a SPA:** Fallback automático para `index.html` para rotas de front-end.
-   **Reescrita e Redirecionamento de URL:** Defina regras customizadas de reescrita e redirecionamento.
-   **Compressão Gzip:** Habilite compressão para melhor performance de carregamento.
-   **Middleware CORS:** Habilita automaticamente cabeçalhos CORS para desenvolvimento de API.
-   **Segurança por Padrão:** Listagem de diretórios desabilitada por padrão.
-   **Binário Único:** Sem dependências externas para deploy.
-   **Desligamento Suave:** Finalização controlada via sinais do sistema.

---

## 🔄 Evolução do brhttp: v1.5 vs. Anteriores

A tabela abaixo detalha a evolução do projeto, desde um servidor puro até uma suíte de desenvolvimento local.

| Característica        | v1.5 (Atual - Dev Server)                                                  | v1.4 (WebSockets)                  | v1.3 (SSE)                         | v1.0 (Inicial)                 |
| :-------------------- | :------------------------------------------------------------------------- | :--------------------------------- | :--------------------------------- | :----------------------------- |
| **Live Reload** | ✅ **Sim, avançado (HMR)** | ✅ Sim, robusto                    | ✅ Sim, funcional                  | ❌ Não                         |
| **Tecnologia** | WebSockets (com HMR)                                                       | WebSockets                         | Server-Sent Events (SSE)           | Nenhuma                        |
| **Configuração** | **Flags e arquivo JSON** | Nenhuma                            | Nenhuma                            | Nenhuma                        |
| **Foco Principal** | **Dev local avançado** | Dev local (robusto)                | Dev local (básico)                 | Servidor estático puro         |
| **Middlewares** | `logging`, `noCache`, `cors`, `gzip`, `proxy`, `rewrite`, `spa`, `custom404`, `injector` | `logging`, `noCache`, `liveReloadInjector` | `logging`, `noCache`, `liveReloadInjector` | `logging`, `noDirListing`      |
| **Funcionalidades** | **Reverse Proxy, SPA, Gzip, Rewrites, CORS, Injeção de código** | Servidor estático                  | Servidor estático                  | Servidor estático              |
| **Dependências** | `fsnotify`, `gorilla/websocket`                                            | `fsnotify`, `gorilla/websocket`    | `fsnotify`                         | Nenhuma                        |

**Vantagem da v1.5:** A versão 1.5 transforma o `brhttp` em uma ferramenta de desenvolvimento completa, rivalizando com soluções como `live-server` do Node.js, mas com a performance e simplicidade de um binário Go. Ele resolve problemas comuns de desenvolvimento, como proxy de API e roteamento de SPA.

---

## 🛠️ Requisitos

-   **Go 1.18+ instalado**

### Instalando Go no Linux

```bash
sudo apt update && sudo apt install golang
```

---

## 📦 Instalação do brhttp

Clone o repositório e acesse a pasta do projeto:

```bash
git clone [https://github.com/henriquetourinho/brhttp.git](https://github.com/henriquetourinho/brhttp.git)
cd brhttp
```

Instale as dependências:

```bash
go mod tidy
```

---

## ▶️ Como usar

### 1. Uso Básico (Zero-Config)

Execute o servidor. Ele servirá a pasta `www` na porta `5571` por padrão.

```bash
go run main.go
```

Abra no navegador: `http://localhost:5571`

Coloque seus arquivos estáticos dentro da pasta `www/`. Qualquer alteração recarregará a página automaticamente.

### 2. Uso com Flags de Linha de Comando

Você pode customizar o comportamento com flags:

```bash
# Servir o diretório 'dist' na porta 8080 com suporte a SPA e Gzip
go run main.go --dir=dist --port=8080 --spa-fallback --enable-gzip
```

**Flags disponíveis:**

| Flag                   | Descrição                                 | Padrão  |
| :--------------------- | :---------------------------------------- | :------ |
| `--port`               | Porta do servidor                         | `5571`  |
| `--dir`                | Diretório a ser servido                   | `www`   |
| `--config`             | Caminho para o arquivo `config.json`      | `""`    |
| `--spa-fallback`       | Habilita fallback para `index.html`       | `false` |
| `--enable-gzip`        | Habilita compressão Gzip                  | `false` |
| `--enable-dir-listing` | Habilita listagem de diretórios           | `false` |
| `--inject-js`          | Injeta um arquivo JS em todas as páginas  | `""`    |
| `--inject-css`         | Injeta um arquivo CSS em todas as páginas | `""`    |
| `--404-page`           | Caminho para uma página 404 customizada   | `""`    |

### 3. Uso com Arquivo `config.json`

Para configurações complexas como reverse proxy e reescritas, crie um arquivo `config.json`:

```json
{
  "port": 5571,
  "serve_dir": "public",
  "spa_fallback_enabled": true,
  "gzip_enabled": true,
  "custom_404_page_path": "404.html",
  "proxy_rules": [
    {
      "path": "/api",
      "target": "http://localhost:3000"
    }
  ],
  "rewrites": [
    {
      "from": "/user-profile",
      "to": "/profile.html"
    }
  ],
  "redirects": [
    {
      "from": "/old-docs",
      "to": "/new-docs",
      "code": 301
    }
  ]
}
```

Execute apontando para o arquivo de configuração:

```bash
go run main.go --config config.json
```

---

## 💡 Funcionamento Interno (v1.5)

O `brhttp` v1.5 opera com uma cadeia de middlewares que processam cada requisição HTTP. A ordem de execução garante que funcionalidades como logging, proxy, reescrita e compressão sejam aplicadas de forma coesa antes de servir o arquivo final. O Live Reload é gerenciado por uma conexão WebSocket (`/ws`) que notifica o front-end sobre mudanças no sistema de arquivos, acionando recarregamentos de página ou substituições de CSS/JS em tempo real (HMR).

---

## 🚫 Limitações (intencionais)

-   Sem suporte a scripts dinâmicos no lado do servidor (PHP, Node.js). Use o reverse proxy para conectar a backends.
-   Sem HTTPS embutido (recomenda-se um proxy reverso como Nginx ou Caddy para produção).
-   Sem autenticação ou controle de acesso complexo.
-   Logs em stdout sem rotação automática.

> 🎯 **O foco é ser a melhor ferramenta de desenvolvimento local: rápida, poderosa e fácil de usar, não substituir servidores de produção completos.**
> A porta padrão 5571 homenageia o Brasil (55) e Salvador (71).

---

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
