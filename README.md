# brhttp — Servidor Web Estático Minimalista em Go

<p align="left">
  <img src="https://img.shields.io/badge/versão-v1.4-blue.svg" alt="Versão" />
  <img src="https://img.shields.io/badge/licença-GPL3-blue.svg" alt="Licença" />
  <img src="https://img.shields.io/badge/Go-1.18%2B-cyan.svg" alt="Go Version" />
  <img src="https://img.shields.io/badge/plataforma-Linux-blue.svg" alt="Plataforma" />
  <img src="https://img.shields.io/badge/feito_no-Brasil-blue.svg" alt="Feito no Brasil" />
</p>

---

## 🚀 O que é o brhttp?

**brhttp** é um servidor web minimalista, escrito em **Go**, focado em servir arquivos estáticos (HTML, CSS, JS, imagens, etc.) com máxima performance e simplicidade. Ele é ideal para ambientes que buscam:

-   Configuração zero
-   Alta performance sem overhead
-   Segurança básica integrada
-   Deploy rápido com binário único

---

## ⚡ Principais Características (v1.4)

-   **Live Reload robusto com WebSockets:** A comunicação para recarregamento automático utiliza **WebSockets**, garantindo uma conexão mais estável e resiliente entre o servidor e o navegador.
-   **Performance extrema:** Servidor leve, sem processamento dinâmico.
-   **Zero configuração:** Execute e já está funcionando.
-   **Segurança automática:** Impede listagem de diretórios.
-   **Binário único:** Sem dependências externas para deploy (após a compilação).
-   **Desligamento suave:** Finalização controlada via sinais do sistema.

---

## 🔄 Evolução do brhttp: v1.4 vs. v1.3 vs. v1.0

A tabela abaixo detalha a evolução do projeto, desde sua concepção como um servidor simples até a implementação de funcionalidades avançadas para desenvolvimento.

| Característica | v1.4 (Atual - WebSockets) | v1.3 (Intermediária - SSE) | v1.0 (Inicial) |
| :--- | :--- | :--- | :--- |
| **Live Reload** | ✅ **Sim, robusto** | ✅ Sim, funcional | ❌ **Não** |
| **Tecnologia** | **WebSockets** | Server-Sent Events (SSE) | Nenhuma (servidor puro) |
| **Robustez da Conexão**| Alta (bidirecional) | Média (unidirecional) | Não aplicável |
| **Dependências** | `fsnotify`, `gorilla/websocket` | `fsnotify` | Nenhuma |
| **Middlewares** | `logging`, `noCache`, `liveReloadInjector` | `logging`, `noCache`, `liveReloadInjector`| `logging`, `noDirListing` |
| **Foco Principal**| Dev local (robusto) e produção simples | Dev local (básico) | Servidor estático puro |

**Vantagem da v1.4:** O uso de WebSockets (v1.4) sobre SSE (v1.3) cria uma conexão mais estável, que sobrevive melhor a instabilidades de rede. Ambas as versões são um grande avanço em relação à v1.0, que não possuía recarregamento automático.

---

## 🛠️ Requisitos

-   **Go 1.18+ instalado**

### Instalando Go no Linux

```bash
sudo apt update && sudo apt install golang
````

-----

## 📦 Instalação do brhttp

Clone o repositório e acesse a pasta do projeto:

```bash
git clone [https://github.com/henriquetourinho/brhttp.git](https://github.com/henriquetourinho/brhttp.git)
cd brhttp
```

Instale as dependências (necessário para `fsnotify` e `websocket`):

```bash
go mod tidy
```

-----

## ▶️ Como usar

Execute o servidor (servirá a pasta `www` na porta `5571`):

```bash
go run main.go
```

Abra no navegador:

[http://localhost:5571](https://www.google.com/search?q=http://localhost:5571)

Coloque seus arquivos estáticos dentro da pasta `www/`.

> **Live Reload:**
> Ao editar qualquer arquivo no diretório servido, páginas HTML abertas no navegador recarregarão automaticamente.

-----

## 💡 Funcionamento Interno (v1.4)

| Componente | Pasta Servida | Live Reload (WebSocket) | Segurança | Logs e Middlewares | Desligamento Suave |
| :---: | :---: | :---: | :---: | :---: | :---: |
| `main.go` | `www/` | Rota `/ws` para conexão WebSocket, injetando script em HTML | Bloqueio da listagem | Logging detalhado | Aguarda 5 segundos após sinal do SO |

-----

## 🚫 Limitações (intencionais)

  - Sem suporte a scripts dinâmicos (PHP, Node.js, Python).
  - Sem HTTPS embutido (recomenda-se proxy reverso).
  - Sem rotas dinâmicas ou APIs.
  - Sem autenticação ou controle de acesso.
  - Sem configuração avançada (cache, compressão, hosts virtuais).
  - Logs em stdout sem rotação automática.

> 🎯 **O foco é ser uma alternativa simples, rápida e segura para servir arquivos estáticos, não substituir servidores completos como Nginx ou Apache.**
> A porta padrão 5571 homenageia o Brasil (55) e Salvador (71).

-----

## 🤝 Apoie o projeto

Se o **brhttp** foi útil, ajude a manter o desenvolvimento:

**Chave Pix:**

```
poupanca@henriquetourinho.com.br
```

-----

## 📄 Licença

Distribuído sob a licença **GPL-3.0** — consulte o arquivo `LICENSE` para detalhes.

-----

## 🙋‍♂️ Desenvolvido por

**Carlos Henrique Tourinho Santana** — Salvador, Bahia, Brasil
🔗 [Wiki Debian](https://wiki.debian.org/henriquetourinho)
🔗 [LinkedIn](https://br.linkedin.com/in/carloshenriquetourinhosantana)
🔗 [GitHub](https://github.com/henriquetourinho)