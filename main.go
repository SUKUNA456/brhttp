package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"compress/gzip" // Importação para Gzip

	"github.com/fsnotify/fsnotify"
	"github.com/gorilla/websocket"
 )

// Configuração do servidor
type Config struct {
	Port                int
	ServeDir            string
	InjectJSPath        string // Caminho para o arquivo JS a ser injetado
	InjectCSSPath       string // Caminho para o arquivo CSS a ser injetado
	SPAFallbackEnabled  bool   // Habilita o fallback para index.html em 404
	DirListingEnabled   bool   // Habilita a listagem de diretórios
	GzipEnabled         bool   // Habilita a compressão Gzip
	Custom404PagePath   string // Caminho para um arquivo HTML personalizado a ser servido em caso de 404
}

// Global para o upgrader de WebSocket
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request ) bool {
		return true // Permite qualquer origem para o WebSocket
	},
}

// Cliente WebSocket
type Client struct {
	conn *websocket.Conn
	send chan []byte
}

// Pool de clientes WebSocket
var clients = make(map[*Client]bool)
var broadcast = make(chan []byte) // Canal para enviar mensagens a todos os clientes

// handleConnections lida com novas conexões WebSocket
func handleConnections(w http.ResponseWriter, r *http.Request ) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Erro ao fazer upgrade para WebSocket: %v", err)
		return
	}
	defer ws.Close()

	// CORREÇÃO AQUI: send deve ser um canal, não um slice
	client := &Client{conn: ws, send: make(chan []byte, 256)}
	clients[client] = true

	go client.writePump() // Inicia o pump de escrita para este cliente

	// Mantém a conexão aberta para receber mensagens (se necessário, para live reload não é)
	for {
		_, _, err := ws.ReadMessage()
		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				log.Printf("Cliente WebSocket desconectado: %v", err)
			} else {
				log.Printf("Erro de leitura WebSocket: %v", err)
			}
			delete(clients, client)
			break
		}
	}
}

// writePump envia mensagens do canal 'send' do cliente para a conexão WebSocket
func (c *Client) writePump() {
	defer func() {
		c.conn.Close()
	}()
	for {
		select {
		case message, ok := <-c.send:
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				log.Printf("Erro ao escrever mensagem WebSocket: %v", err)
				return
			}
		}
	}
}

// handleMessages envia mensagens do canal de broadcast para todos os clientes conectados
func handleMessages() {
	for {
		message := <-broadcast
		for client := range clients {
			select {
			case client.send <- message:
			default:
				close(client.send)
				delete(clients, client)
			}
		}
	}
}

// watchFiles monitora o diretório de serviço para mudanças e envia sinal de recarga
func watchFiles(dir string) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatalf("Erro ao criar watcher: %v", err)
	}
	defer watcher.Close()

	done := make(chan bool)

	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				// Ignora eventos de modificação de diretórios ou arquivos temporários
				if event.Op&fsnotify.Write == fsnotify.Write || event.Op&fsnotify.Create == fsnotify.Create || event.Op&fsnotify.Remove == fsnotify.Remove {
					// Ignora arquivos temporários criados por editores
					if strings.HasPrefix(filepath.Base(event.Name), ".") || strings.HasSuffix(event.Name, "~") || strings.HasSuffix(event.Name, ".tmp") {
						continue
					}

					relPath, err := filepath.Rel(dir, event.Name)
					if err != nil {
						log.Printf("Erro ao obter caminho relativo para %s: %v", event.Name, err)
						continue
					}
					// Converte o caminho do sistema de arquivos para URL path
					urlPath := "/" + strings.ReplaceAll(relPath, string(os.PathSeparator), "/")

					var msgType string
					if strings.HasSuffix(event.Name, ".css") {
						msgType = "css-update"
					} else if strings.HasSuffix(event.Name, ".js") {
						msgType = "js-update"
					} else {
						msgType = "reload" // Default para HTML e outros
					}

					message, _ := json.Marshal(map[string]string{
						"type": msgType,
						"path": urlPath,
					})
					log.Printf("Arquivo modificado: %s. Enviando sinal de %s...", event.Name, msgType)
					broadcast <- message
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Printf("Erro do watcher: %v", err)
			}
		}
	}()

	// Adiciona o diretório e seus subdiretórios ao watcher
	err = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return watcher.Add(path)
		}
		return nil
	})

	if err != nil {
		log.Fatalf("Erro ao adicionar diretórios ao watcher: %v", err)
	}

	<-done
}

// loggingMiddleware registra informações sobre cada requisição HTTP
func loggingMiddleware(next http.Handler ) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request ) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("[%s] %s %s %s", r.Method, r.URL.Path, r.RemoteAddr, time.Since(start))
	})
}

// noCacheMiddleware adiciona cabeçalhos para prevenir cache em requisições
func noCacheMiddleware(next http.Handler ) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request ) {
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")
		next.ServeHTTP(w, r)
	})
}

// corsMiddleware adiciona os cabeçalhos CORS necessários para permitir requisições de qualquer origem.
func corsMiddleware(next http.Handler ) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request ) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Credentials", "true")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK )
			return
		}
		next.ServeHTTP(w, r)
	})
}

// responseRecorder é um http.ResponseWriter que armazena o status e o corpo da resposta.
type responseRecorder struct {
	http.ResponseWriter
	StatusCode int
	Body       *bytes.Buffer
	Headers    http.Header // Para armazenar os cabeçalhos
}

func newResponseRecorder(w http.ResponseWriter ) *responseRecorder {
	return &responseRecorder{
		ResponseWriter: w,
		StatusCode:     http.StatusOK,
		Body:           new(bytes.Buffer ),
		Headers:        make(http.Header ), // Inicializa o mapa de cabeçalhos
	}
}

// Header retorna o mapa de cabeçalhos interno do recorder.
func (r *responseRecorder) Header() http.Header {
	return r.Headers
}

// WriteHeader armazena o status code, mas não escreve para o ResponseWriter original ainda.
func (r *responseRecorder ) WriteHeader(statusCode int) {
	r.StatusCode = statusCode
}

// Write escreve o corpo da resposta para o buffer interno, mas não para o ResponseWriter original ainda.
func (r *responseRecorder) Write(b []byte) (int, error) {
	return r.Body.Write(b)
}

// CopyTo copia os cabeçalhos, o status e o corpo do recorder para o ResponseWriter original.
func (r *responseRecorder) CopyTo(w http.ResponseWriter ) {
	// Copia os cabeçalhos armazenados para o ResponseWriter real
	for k, v := range r.Headers {
		w.Header()[k] = v
	}
	// Escreve o status code armazenado
	w.WriteHeader(r.StatusCode)
	// Escreve o corpo armazenado
	w.Write(r.Body.Bytes())
}

// liveReloadInjector injeta o script de live reload e scripts/estilos personalizados em páginas HTML.
func liveReloadInjector(injectedJSContent, injectedCSSContent string, next http.Handler ) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request ) {
		if r.URL.Path == "/ws" || r.Method != http.MethodGet {
			next.ServeHTTP(w, r )
			return
		}

		recorder := newResponseRecorder(w)
		next.ServeHTTP(recorder, r)

		if strings.Contains(recorder.Header().Get("Content-Type"), "text/html") && recorder.StatusCode == http.StatusOK {
			body := recorder.Body.Bytes( )

			// Script de Live Reload e HMR
			liveReloadAndHMRScript := fmt.Sprintf(`
            <script>
                var ws = new WebSocket("ws://%s/ws");
                ws.onmessage = function(event) {
                    var message = JSON.parse(event.data);
                    if (message.type === "reload") {
                        console.log("Recarregando página...");
                        location.reload();
                    } else if (message.type === "css-update") {
                        var link = document.querySelector('link[href*="' + message.path + '"]');
                        if (link) {
                            var newHref = message.path + '?v=' + new Date().getTime();
                            link.href = newHref;
                            console.log('CSS atualizado: ' + message.path);
                        } else {
                            console.warn('Link CSS não encontrado para atualização: ' + message.path + '. Recarregando página.');
                            location.reload(); // Fallback se o link não for encontrado
                        }
                    } else if (message.type === "js-update") {
                        var script = document.querySelector('script[src*="' + message.path + '"]');
                        if (script) {
                            var newScript = document.createElement('script');
                            newScript.src = message.path + '?v=' + new Date().getTime();
                            newScript.async = true; // Garante que o script não bloqueie o render
                            newScript.onload = function() { console.log('JavaScript atualizado: ' + message.path + ' (re-executado)'); };
                            newScript.onerror = function() { console.error('Erro ao carregar script atualizado: ' + message.path); };
                            script.parentNode.replaceChild(newScript, script);
                            // ATENÇÃO: Isso re-executa o script. Não é um HMR "verdadeiro".
                            // Variáveis globais e listeners de eventos podem ser duplicados ou causar problemas.
                        } else {
                            console.warn('Script JS não encontrado para atualização: ' + message.path + '. Recarregando página.');
                            location.reload(); // Fallback se o script não for encontrado
                        }
                    }
                };
            </script>
            `, r.Host)

			// Scripts e estilos personalizados injetados
			var customInjections bytes.Buffer
			if injectedCSSContent != "" {
				customInjections.WriteString(fmt.Sprintf("<style>\n%s\n</style>\n", injectedCSSContent))
			}
			if injectedJSContent != "" {
				customInjections.WriteString(fmt.Sprintf("<script>\n%s\n</script>\n", injectedJSContent))
			}

			// Injetar estilos na <head>
			if idx := bytes.LastIndex(body, []byte("</head>")); idx != -1 {
				body = bytes.Join([][]byte{body[:idx], customInjections.Bytes(), body[idx:]}, nil)
			}

			// Injetar scripts no </body>
			if idx := bytes.LastIndex(body, []byte("</body>")); idx != -1 {
				body = bytes.Join([][]byte{body[:idx], []byte(liveReloadAndHMRScript), body[idx:]}, nil)
			} else {
				// Se não houver </body>, adicione ao final (menos ideal, mas fallback)
				body = bytes.Join([][]byte{body, []byte(liveReloadAndHMRScript)}, nil)
			}

			// Copia os cabeçalhos do recorder para o ResponseWriter real
			for k, v := range recorder.Headers {
				w.Header()[k] = v
			}
			w.Header().Set("Content-Length", fmt.Sprintf("%d", len(body)))
			w.WriteHeader(recorder.StatusCode) // Escreve o status code original
			w.Write(body)
			return
		}

		recorder.CopyTo(w)
	})
}

// spaFallbackMiddleware serve index.html se o arquivo não for encontrado e a flag estiver ativa.
func spaFallbackMiddleware(serveDir string, enabled bool, next http.Handler ) http.Handler {
	if !enabled {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request ) {
		recorder := newResponseRecorder(w)
		next.ServeHTTP(recorder, r)

		// Se for 404 e não for uma requisição para um arquivo com extensão (ex: .js, .css, .png)
		// e não for a rota do WebSocket, tenta servir index.html
		if recorder.StatusCode == http.StatusNotFound && !strings.Contains(filepath.Base(r.URL.Path ), ".") && r.URL.Path != "/ws" {
			indexPath := filepath.Join(serveDir, "index.html")
			if _, err := os.Stat(indexPath); err == nil {
				log.Printf("SPA Fallback: Servindo %s para %s", indexPath, r.URL.Path)
				// Resetar o recorder e servir o index.html diretamente
				http.ServeFile(w, r, indexPath )
				return
			}
		}
		recorder.CopyTo(w)
	})
}

// customErrorPageMiddleware serves a custom 404 page if enabled and the original handler returned 404.
func customErrorPageMiddleware(custom404Path string, serveDir string, next http.Handler ) http.Handler {
	if custom404Path == "" {
		return next // No custom 404 page configured
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request ) {
		recorder := newResponseRecorder(w)
		next.ServeHTTP(recorder, r) // Let the next handler process the request

		if recorder.StatusCode == http.StatusNotFound {
			full404Path := filepath.Join(serveDir, custom404Path ) // Assume custom404Path is relative to serveDir
			if _, err := os.Stat(full404Path); err == nil {
				log.Printf("Servindo página 404 personalizada: %s para %s", full404Path, r.URL.Path)
				// Clear existing headers and set new ones for the custom page
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				w.WriteHeader(http.StatusNotFound ) // Still return 404 status
				http.ServeFile(w, r, full404Path )
				return
			}
		}
		recorder.CopyTo(w) // If not 404 or custom page not found, copy original response
	})
}

// noDirListingFileSystem wraps http.Dir to prevent directory listings.
type noDirListingFileSystem struct {
	fs http.FileSystem
}

// noDirListingFile wraps http.File to prevent Readdir.
type noDirListingFile struct {
	http.File
}

// Readdir returns an error to prevent directory listing.
func (f noDirListingFile ) Readdir(count int) ([]os.FileInfo, error) {
	return nil, os.ErrNotExist // Retorna "não existe" para simular que não é um diretório listável
}

func (fs noDirListingFileSystem) Open(name string) (http.File, error ) {
	f, err := fs.fs.Open(name)
	if err != nil {
		return nil, err
	}
	stat, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if stat.IsDir() {
		return noDirListingFile{f}, nil // Retorna nosso wrapper para impedir Readdir
	}
	return f, nil
}

// gzipWriter é um ResponseWriter que comprime a saída usando gzip.
type gzipWriter struct {
	http.ResponseWriter
	Writer *gzip.Writer
}

func (g *gzipWriter ) Write(data []byte) (int, error) {
	return g.Writer.Write(data)
}

func (g *gzipWriter) WriteHeader(statusCode int) {
	g.ResponseWriter.Header().Del("Content-Length") // Remove Content-Length pois o tamanho muda após compressão
	g.ResponseWriter.WriteHeader(statusCode)
}

// gzipMiddleware comprime a resposta se o cliente aceitar gzip.
func gzipMiddleware(enabled bool, next http.Handler ) http.Handler {
	if !enabled {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request ) {
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			next.ServeHTTP(w, r)
			return
		}

		w.Header().Set("Content-Encoding", "gzip")
		gz := gzip.NewWriter(w)
		defer gz.Close()

		gzw := &gzipWriter{ResponseWriter: w, Writer: gz}
		next.ServeHTTP(gzw, r)
	})
}

// readInjectedFileContent lê o conteúdo de um arquivo para injeção.
func readInjectedFileContent(filePath string) string {
	if filePath == "" {
		return ""
	}
	content, err := os.ReadFile(filePath)
	if err != nil {
		log.Printf("Aviso: Não foi possível ler o arquivo de injeção '%s': %v", filePath, err)
		return ""
	}
	return string(content)
}

func main() {
	var config Config
	flag.IntVar(&config.Port, "port", 5571, "Porta para o servidor HTTP")
	flag.StringVar(&config.ServeDir, "dir", "www", "Diretório para servir arquivos estáticos")
	flag.StringVar(&config.InjectJSPath, "inject-js", "", "Caminho para um arquivo JavaScript a ser injetado em todas as páginas HTML.")
	flag.StringVar(&config.InjectCSSPath, "inject-css", "", "Caminho para um arquivo CSS a ser injetado em todas as páginas HTML.")
	flag.BoolVar(&config.SPAFallbackEnabled, "spa-fallback", false, "Habilita o fallback para index.html em rotas não encontradas (útil para SPAs).")
	flag.BoolVar(&config.DirListingEnabled, "enable-dir-listing", false, "Habilita a listagem de diretórios (desabilitado por padrão para segurança).")
	flag.BoolVar(&config.GzipEnabled, "enable-gzip", false, "Habilita a compressão Gzip para respostas (desabilitado por padrão).")
	flag.StringVar(&config.Custom404PagePath, "404-page", "", "Caminho para um arquivo HTML personalizado a ser servido em caso de 404 (ex: 404.html).")
	flag.Parse()

	// Garante que o diretório existe
	if _, err := os.Stat(config.ServeDir); os.IsNotExist(err) {
		log.Fatalf("Diretório '%s' não encontrado. Por favor, crie-o ou especifique um diretório válido.", config.ServeDir)
	}

	// Lê o conteúdo dos arquivos a serem injetados
	injectedJSContent := readInjectedFileContent(config.InjectJSPath)
	injectedCSSContent := readInjectedFileContent(config.InjectCSSPath)

	// Inicia o manipulador de mensagens WebSocket
	go handleMessages()

	// Inicia o watcher de arquivos
	go watchFiles(config.ServeDir)

	// Cria um novo multiplexador HTTP
	mux := http.NewServeMux( )

	// Rota para o WebSocket
	mux.HandleFunc("/ws", handleConnections)

	// Cria o manipulador de arquivos estáticos, controlando a listagem de diretórios
	var fileSystem http.FileSystem
	if config.DirListingEnabled {
		fileSystem = http.Dir(config.ServeDir )
		log.Println("Aviso: A listagem de diretórios está HABILITADA.")
	} else {
		fileSystem = noDirListingFileSystem{http.Dir(config.ServeDir )}
		log.Println("Listagem de diretórios está DESABILITADA (padrão).")
	}
	fileServer := http.FileServer(fileSystem )

	// Aplica os middlewares na ordem desejada
	// A ordem é crucial:
	// 1. loggingMiddleware: para registrar a requisição
	// 2. gzipMiddleware: para comprimir a resposta (se habilitado e aceito pelo cliente)
	// 3. noCacheMiddleware: para garantir que o navegador não cacheie
	// 4. corsMiddleware: para adicionar os cabeçalhos CORS
	// 5. customErrorPageMiddleware: para servir páginas de erro personalizadas (se houver 404)
	// 6. spaFallbackMiddleware: para lidar com rotas de SPA (antes do injetor para que index.html seja processado)
	// 7. liveReloadInjector: para injetar scripts/estilos e o live reload
	var handler http.Handler = fileServer
	handler = liveReloadInjector(injectedJSContent, injectedCSSContent, handler )
	handler = spaFallbackMiddleware(config.ServeDir, config.SPAFallbackEnabled, handler)
	handler = customErrorPageMiddleware(config.Custom404PagePath, config.ServeDir, handler)
	handler = corsMiddleware(handler)
	handler = noCacheMiddleware(handler)
	handler = gzipMiddleware(config.GzipEnabled, handler)
	handler = loggingMiddleware(handler)

	// Associa o manipulador com o caminho raiz
	mux.Handle("/", handler)

	// Configura o servidor HTTP
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", config.Port ),
		Handler: mux,
	}

	// Canal para sinais do sistema
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// Inicia o servidor em uma goroutine
	go func() {
		log.Printf("brhttp está rodando em http://localhost:%d (servindo '%s' )", config.Port, config.ServeDir)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Erro ao iniciar o servidor: %v", err )
		}
	}()

	// Espera pelo sinal de desligamento
	<-quit
	log.Println("Recebido sinal de desligamento. Desligando o servidor...")

	// Cria um contexto com timeout para o desligamento suave
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Tenta desligar o servidor suavemente
	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Servidor forçado a desligar: %v", err)
	}

	log.Println("Servidor desligado com sucesso.")
}
