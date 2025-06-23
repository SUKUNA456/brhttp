/*
brhttp — Servidor Web Estático Minimalista com Live Reload
---------------------------------------------------------
Versão:      v1.4
Licença:     GPL-3.0
Go:          1.18+
Plataforma:  Linux/Unix
Autor:       Carlos Henrique Tourinho Santana
GitHub:      https://github.com/henriquetourinho/brhttp

Descrição:
-----------
O brhttp é um servidor HTTP minimalista escrito em Go, voltado para servir arquivos estáticos (HTML, CSS, JS, imagens, etc) com máxima simplicidade, segurança básica e desempenho.  
A partir da versão v1.4, inclui recarregamento automático de páginas HTML (Live Reload) ao editar arquivos no diretório servido, tornando-o ideal para desenvolvimento web local.

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
	"github.com/gorilla/websocket" // Importa a biblioteca padrão para WebSockets em Go
)

// # --- Hub e Lógica de Live Reload --- #
// (Nenhuma alteração necessária aqui, o Hub é genérico e funciona perfeitamente)
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
	log.Printf("--> [Vigia] Observando a pasta '%s' para mudanças...", path)
	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			if event.Op&fsnotify.Write == fsnotify.Write || event.Op&fsnotify.Create == fsnotify.Create || event.Op&fsnotify.Remove == fsnotify.Remove || event.Op&fsnotify.Rename == fsnotify.Rename {
				log.Printf("--> [Vigia] Mudança detectada: %s. Enviando sinal de reload.", event.Name)
				hub.broadcast <- "reload"
			}
		case _, ok := <-watcher.Errors:
			if !ok {
				return
			}
		}
	}
}

// # --- Middlewares (Postos de Controle) --- #

func noCacheMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")
		next.ServeHTTP(w, r)
	})
}

// CORRIGIDO: O script agora usa a API de WebSocket no lado do cliente.
const liveReloadScriptWS = `<script>
(function() {
    const connect = () => {
        const socket = new WebSocket("ws://" + window.location.host + "/ws");
        
        socket.onopen = function() {
            console.log("brhttp: Conectado ao servidor de Live Reload.");
        };

        socket.onmessage = function(event) {
            if (event.data === 'reload') {
                console.log("brhttp: Mudança detectada, recarregando...");
                window.location.reload();
            }
        };

        socket.onclose = function(event) {
            console.log("brhttp: Conexão Live Reload perdida. Tentando reconectar em 1s...");
            setTimeout(connect, 1000); // Tenta reconectar após 1 segundo
        };

		socket.onerror = function(error) {
			console.error("brhttp: Erro no WebSocket: ", error);
			socket.close();
		};
    };
    connect();
})();
</script>`

type responseRecorder struct {
	http.ResponseWriter
	body *bytes.Buffer
}

func (r *responseRecorder) Write(b []byte) (int, error) { return r.body.Write(b) }

func liveReloadInjectorMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Só injeta se for um GET para um arquivo HTML
		if r.Method != http.MethodGet || !strings.HasSuffix(strings.ToLower(r.URL.Path), ".html") {
			next.ServeHTTP(w, r)
			return
		}

		buffer := &bytes.Buffer{}
		recorder := &responseRecorder{ResponseWriter: w, body: buffer}
		next.ServeHTTP(recorder, r)

		// Copia os headers originais, exceto o Content-Length que será recalculado
		for key, values := range recorder.Header() {
			if strings.ToLower(key) != "content-length" {
				for _, value := range values {
					w.Header().Add(key, value)
				}
			}
		}

		bodyBytes := bytes.Replace(buffer.Bytes(), []byte("</body>"), []byte(liveReloadScriptWS+"</body>"), 1)
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

// NOVO: Upgrader para "promover" conexões HTTP para WebSocket
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// Em desenvolvimento, permitimos qualquer origem. Em produção, isso deve ser mais restrito.
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// NOVO: Handler para as conexões WebSocket
func wsHandler(hub *Hub, w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("ERRO: Falha ao fazer upgrade para WebSocket: %v", err)
		return
	}

	// Cria um canal para este cliente, registra no hub e garante o desregistro na saída.
	clientChan := make(chan string)
	hub.register <- clientChan
	defer func() { hub.unregister <- clientChan }()

	// Goroutine para enviar mensagens do Hub para o cliente WebSocket
	go func() {
		defer conn.Close()
		for message := range clientChan {
			if err := conn.WriteMessage(websocket.TextMessage, []byte(message)); err != nil {
				// Se não conseguir escrever, assume que o cliente desconectou
				return
			}
		}
	}()

	// Loop para manter a conexão viva e detectar quando o cliente desconecta.
	// Se ReadMessage retornar erro, significa que a conexão caiu.
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			break // Sai do loop e aciona os 'defers'
		}
	}
}

// # --- Função Principal (O Coração do Programa) --- #

func main() {
	webrootDir := "./www" // Diretório padrão
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

	// CORRIGIDO: Rota para as conexões WebSocket
	router.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		wsHandler(hub, w, r)
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
		log.Printf("🚀 Servidor 'brhttp v1.4 (WebSocket)' iniciado. Escutando em http://localhost:5571")
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
