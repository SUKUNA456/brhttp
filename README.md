# brhttp — Servidor Web Estático Minimalista em Go

<p align="left">
  <img src="https://img.shields.io/badge/vers%C3%A3o-v1.3-blue.svg" alt="Versão" />
  <img src="https://img.shields.io/badge/licen%C3%A7a-GPL3-blue.svg" alt="Licença" />
  <img src="https://img.shields.io/badge/Go-1.18%2B-cyan.svg" alt="Go Version" />
  <img src="https://img.shields.io/badge/plataforma-Linux-blue.svg" alt="Plataforma" />
  <img src="https://img.shields.io/badge/feito_no-Brasil-blue.svg" alt="Feito no Brasil" />
</p>

---

## 🚀 O que é o brhttp?

**brhttp** é um servidor web minimalista, escrito em **Go**, focado em servir arquivos estáticos (HTML, CSS, JS, imagens, etc) com máxima performance e simplicidade. Ele é ideal para ambientes que buscam:

- Configuração zero
- Alta performance sem overhead
- Segurança básica integrada
- Deploy rápido com binário único

---

## ⚡ Principais Características

- **Live Reload nativo (v1.3):** Navegadores HTML conectados recarregam automaticamente ao editar arquivos no diretório servido.
- **Performance extrema:** Servidor leve, sem processamento dinâmico.
- **Zero configuração:** Execute e já está funcionando.
- **Segurança automática:** Impede listagem de diretórios.
- **Binário único:** Sem dependências externas para deploy.
- **Desligamento suave:** Finalização controlada via sinais do sistema.

---

## 🛠️ Requisitos

- **Go 1.18+ instalado**

### Instalando Go no Linux

```bash
sudo apt update && sudo apt install golang
```

Ou para instalar manualmente a versão mais recente:

```bash
wget https://go.dev/dl/go1.22.3.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.22.3.linux-amd64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc
```

Confira a versão instalada:

```bash
go version
```

---

## 📦 Instalação do brhttp

Clone o repositório e acesse a pasta do projeto:

```bash
git clone https://github.com/henriquetourinho/brhttp.git
cd brhttp
```

---

## ▶️ Como usar

Execute o servidor (servirá a pasta `www` na porta `5571`):

```bash
go run main.go
```

Abra no navegador:

[http://localhost:5571](http://localhost:5571)

Coloque seus arquivos estáticos dentro da pasta `www/`.

> **Live Reload:**  
> Ao editar arquivos `.html` no diretório servido, páginas abertas no navegador recarregam automaticamente.

---

## 💡 Funcionamento Interno (Resumo)

| Arquivo Principal | Pasta Servida | Live Reload           | Segurança                | Logs e Middlewares      | Desligamento Suave                    |
|:-----------------:|:-------------:|:---------------------:|:------------------------:|:-----------------------:|:------------------------------------:|
| `main.go`         | `www/`        | Injeção automática em HTML | Bloqueio da listagem     | Logging detalhado       | Aguarda 5 segundos após sinal do SO  |

---

## Código principal — `main.go`

```go
package main

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
)

// --- Hub e Vigia (Live Reload) ---
type Hub struct {
	clients    map[chan string]bool
	register   chan chan string
	unregister chan chan string
	broadcast  chan string
	mu         sync.Mutex
}
func newHub() *Hub {
	return &Hub{
		clients:    make(map[chan string]bool),
		register:   make(chan chan string),
		unregister: make(chan chan string),
		broadcast:  make(chan string),
	}
}
func (h *Hub) run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client)
			}
			h.mu.Unlock()
		case message := <-h.broadcast:
			h.mu.Lock()
			for client := range h.clients {
				select {
				case client <- message:
				default:
				}
			}
			h.mu.Unlock()
		}
	}
}
func watchFiles(hub *Hub, path string) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatalf("ERRO: Vigia: %v", err)
	}
	defer watcher.Close()
	err = filepath.Walk(path, func(walkPath string, fi os.FileInfo, err error) error {
		if fi.IsDir() {
			return watcher.Add(walkPath)
		}
		return nil
	})
	if err != nil {
		log.Fatalf("ERRO: Vigia path: %v", err)
	}
	log.Printf("--> [Vigia] Observando a pasta '%s'", path)
	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			if event.Op&fsnotify.Write == fsnotify.Write || event.Op&fsnotify.Create == fsnotify.Create || event.Op&fsnotify.Remove == fsnotify.Remove || event.Op&fsnotify.Rename == fsnotify.Rename {
				hub.broadcast <- "reload"
			}
		case _, ok := <-watcher.Errors:
			if !ok {
				return
			}
		}
	}
}

// --- Middlewares ---

func noCacheMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")
		next.ServeHTTP(w, r)
	})
}

const liveReloadScript = `<script>const evtSource=new EventSource("/livereload-events");evtSource.onmessage=function(event){if(event.data==='reload'){console.log("brhttp: Mudança detectada, recarregando...");window.location.reload();}};</script>`
type responseRecorder struct {
	http.ResponseWriter
	body *bytes.Buffer
}
func (r *responseRecorder) Write(b []byte) (int, error) { return r.body.Write(b) }

func liveReloadInjectorMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(strings.ToLower(r.URL.Path), ".html") {
			next.ServeHTTP(w, r)
			return
		}
		buffer := &bytes.Buffer{}
		recorder := &responseRecorder{ResponseWriter: w, body: buffer}
		next.ServeHTTP(recorder, r)
		for key, values := range recorder.Header() {
			if strings.ToLower(key) != "content-length" {
				for _, value := range values {
					w.Header().Add(key, value)
				}
			}
		}
		bodyBytes := bytes.Replace(buffer.Bytes(), []byte("</body>"), []byte(liveReloadScript+"</body>"), 1)
		w.Header().Set("Content-Length", fmt.Sprint(len(bodyBytes)))
		w.Write(bodyBytes)
	})
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		log.Printf("--> [%s] %s %s", r.RemoteAddr, r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
		log.Printf("<-- Finalizado %s em %v", r.URL.Path, time.Since(start))
	})
}

// --- Função Principal ---

func main() {
	webrootDir := "./www"
	if len(os.Args) > 1 {
		webrootDir = os.Args[1]
	}
	if _, err := os.Stat(webrootDir); os.IsNotExist(err) {
		log.Fatalf("ERRO FATAL: O diretório raiz '%s' não existe.", webrootDir)
	}

	hub := newHub()
	go hub.run()
	go watchFiles(hub, webrootDir)

	fileServer := http.FileServer(http.Dir(webrootDir))
	router := http.NewServeMux()

	router.HandleFunc("/livereload-events", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		messageChan := make(chan string)
		hub.register <- messageChan
		defer func() { hub.unregister <- messageChan }()
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "Streaming not supported", http.StatusInternalServerError)
			return
		}
		for {
			select {
			case message, open := <-messageChan:
				if !open {
					return
				}
				fmt.Fprintf(w, "data: %s\n\n", message)
				flusher.Flush()
			case <-r.Context().Done():
				return
			}
		}
	})

	router.Handle("/", liveReloadInjectorMiddleware(fileServer))
	handler := loggingMiddleware(noCacheMiddleware(router))

	server := &http.Server{
		Addr:    "[::]:5571",
		Handler: handler,
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Println("======================================================================")
		log.Printf("🚀 Servidor 'brhttp v1.3 (Estável)' iniciado. Escutando em http://localhost:5571")
		log.Printf("--> O diretório que será exibido é: %s", webrootDir)
		log.Println("======================================================================")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Erro ao iniciar o servidor: %v", err)
		}
	}()

	<-quit
	log.Println("... Desligando o servidor ...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Erro no desligamento do servidor: %v", err)
	}
	log.Println("✅ Servidor desligado com sucesso.")
}
```

---

## 🧠 O que o código faz?

- Serve arquivos estáticos do diretório `www` na porta `5571`
- Loga cada requisição com IP, método, rota e tempo de resposta
- Injeta script de Live Reload automaticamente em arquivos `.html`
- Observa alterações de arquivos e notifica clientes conectados para recarregar a página
- Bloqueia listagem de diretórios (URLs terminando com `/`)
- Realiza desligamento suave aguardando conexões finalizarem
- Executa de forma concorrente com goroutines nativas do Go
- Não requer configuração extra nem dependências externas

---

## 🚫 Limitações (intencionais)

- Sem suporte a scripts dinâmicos (PHP, Node.js, Python)
- Sem HTTPS embutido (recomenda-se proxy reverso)
- Sem rotas dinâmicas ou APIs
- Sem autenticação ou controle de acesso
- Sem configuração avançada (cache, compressão, hosts virtuais)
- Logs em stdout sem rotação automática

> 🎯 **O foco é ser uma alternativa simples, rápida e segura para servir arquivos estáticos, não substituir servidores completos como Nginx ou Apache.**  
> A porta padrão 5571 homenageia o Brasil (55) e Salvador (71).

---

## 🔐 Segurança e Boas Práticas

- Listagem de diretórios desativada para evitar exposição indesejada
- Binário único, sem comunicação externa
- Desligamento controlado para evitar perda de dados
- Live Reload somente em arquivos HTML, sem interferir em outros MIME types

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
🔗 [Wiki Debian](https://wiki.debian.org/henriquetourinho)  
🔗 [LinkedIn](https://br.linkedin.com/in/carloshenriquetourinhosantana)  
🔗 [GitHub](https://github.com/henriquetourinho)