/*
brhttp — Servidor Web Estático Minimalista com Live Reload
---------------------------------------------------------
Versão:      v1.3
Licença:     GPL-3.0
Go:          1.18+
Plataforma:  Linux/Unix
Autor:       Carlos Henrique Tourinho Santana
GitHub:      https://github.com/henriquetourinho/brhttp

Descrição:
-----------
O brhttp é um servidor HTTP minimalista escrito em Go, voltado para servir arquivos estáticos (HTML, CSS, JS, imagens, etc) com máxima simplicidade, segurança básica e desempenho.  
A partir da versão v1.3, inclui recarregamento automático de páginas HTML (Live Reload) ao editar arquivos no diretório servido, tornando-o ideal para desenvolvimento web local.

Principais recursos:
--------------------
- **Live Reload automático:** Injeta um script apenas em páginas HTML para recarregar o navegador ao detectar alterações nos arquivos.
- **Zero configuração:** Basta rodar e usar.
- **Logs detalhados:** Cada requisição é registrada no console.
- **Desligamento suave:** Aguarda até 5s para encerrar conexões abertas ao receber sinais SIGINT/SIGTERM.
- **Segurança:** Bloqueia listagem de diretórios e não executa código dinâmico.
- **Binário único:** Fácil distribuição e deploy.

Como usar:
----------
- Coloque os arquivos em uma pasta `www/`.
- Execute: `go run main.go` ou `go run main.go <diretório>`
- Acesse: http://localhost:5571

Limitações:
-----------
- Não suporta HTTPS ou scripts dinâmicos.
- Live reload é exclusivo para arquivos `.html`.
- Sem autenticação embutida.

Apoie: poupança@henriquetourinho.com.br (Pix)
*/

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

// # --- Hub e Vigia (Live Reload) --- #
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
			if !ok { return }
			if event.Op&fsnotify.Write == fsnotify.Write || event.Op&fsnotify.Create == fsnotify.Create || event.Op&fsnotify.Remove == fsnotify.Remove || event.Op&fsnotify.Rename == fsnotify.Rename {
				hub.broadcast <- "reload"
			}
		case _, ok := <-watcher.Errors:
			if !ok { return }
		}
	}
}

// # --- Middlewares (Postos de Controle) --- #

// noCacheMiddleware força o navegador a não usar cache.
func noCacheMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")
		next.ServeHTTP(w, r)
	})
}

// liveReloadInjectorMiddleware injeta o script de live-reload APENAS em páginas HTML.
const liveReloadScript = `<script>const evtSource=new EventSource("/livereload-events");evtSource.onmessage=function(event){if(event.data==='reload'){console.log("brhttp: Mudança detectada, recarregando...");window.location.reload();}};</script>`
type responseRecorder struct {
	http.ResponseWriter
	body *bytes.Buffer
}
func (r *responseRecorder) Write(b []byte) (int, error) { return r.body.Write(b) }

func liveReloadInjectorMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Apenas injeta em requisições para arquivos que terminam com .html
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

// loggingMiddleware registra cada requisição no console.
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		log.Printf("--> [%s] %s %s", r.RemoteAddr, r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
		log.Printf("<-- Finalizado %s em %v", r.URL.Path, time.Since(start))
	})
}

// # --- Função Principal (O Coração do Programa) --- #

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

	// Cria o servidor de arquivos que aponta para o nosso diretório raiz
	fileServer := http.FileServer(http.Dir(webrootDir))

	router := http.NewServeMux()

	// Rota especial para os eventos de Live Reload
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
				if !open { return }
				fmt.Fprintf(w, "data: %s\n\n", message)
				flusher.Flush()
			case <-r.Context().Done():
				return
			}
		}
	})

	// Rota principal que serve os arquivos
	// O injetor de script agora está junto com o fileServer.
	router.Handle("/", liveReloadInjectorMiddleware(fileServer))

	// Encadeia apenas os middlewares globais.
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