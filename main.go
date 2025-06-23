package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec" // Import para executar comandos externos
	"path/filepath"
	"strings"
	"sync" // Para proteger o mapa de clientes WebSocket
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/gorilla/websocket"
)

// ProxyRule define uma regra para o reverse proxy
type ProxyRule struct {
	Path   string `json:"path"`
	Target string `json:"target"`
}

// RewriteRule define uma regra de reescrita de URL
type RewriteRule struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// RedirectRule define uma regra de redirecionamento de URL
type RedirectRule struct {
	From string `json:"from"`
	To   string `json:"to"`
	Code int    `json:"code"`
}

// CommandWebhookRule define uma regra para executar um comando externo em um evento
type CommandWebhookRule struct {
	Event   string   `json:"event"` // "file_change", "server_start", "server_stop"
	Path    string   `json:"path"`  // Optional: regex or prefix for file path (for file_change)
	Command string   `json:"command"`
	Args    []string `json:"args"`
}

// Configuração do servidor
type Config struct {
	Port                   int                  `json:"port"`
	ServeDir               string               `json:"serve_dir"`
	InjectJSPath           string               `json:"inject_js_path"`
	InjectCSSPath          string               `json:"inject_css_path"`
	SPAFallbackEnabled     bool                 `json:"spa_fallback_enabled"`
	DirListingEnabled      bool                 `json:"dir_listing_enabled"`
	GzipEnabled            bool                 `json:"gzip_enabled"`
	Custom404PagePath      string               `json:"custom_404_page_path"`
	ProxyRules             []ProxyRule          `json:"proxy_rules"`
	Rewrites               []RewriteRule        `json:"rewrites"`
	Redirects              []RedirectRule       `json:"redirects"`
	WatchDebounceMs        int                  `json:"watch_debounce_ms"`
	WatchExcludeDirs       []string             `json:"watch_exclude_dirs"`
	LogFilePath            string               `json:"log_file_path"`
	APIToken               string               `json:"api_token"`
	NotificationWebhookURL string               `json:"notification_webhook_url"`
	CommandWebhooks        []CommandWebhookRule `json:"command_webhooks"`
}

// Global para o upgrader de WebSocket
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// Cliente WebSocket
type Client struct {
	conn *websocket.Conn
	send chan []byte
}

// Pool de clientes WebSocket
var clients = make(map[*Client]bool)
var clientsMutex = &sync.Mutex{} // Mutex para proteger o mapa de clientes
var broadcast = make(chan []byte)
var serverStartTime = time.Now() // Para o endpoint /api/status

// handleConnections lida com novas conexões WebSocket
func handleConnections(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Erro ao fazer upgrade para WebSocket: %v", err)
		return
	}
	defer ws.Close()

	client := &Client{conn: ws, send: make(chan []byte, 256)}
	clientsMutex.Lock()
	clients[client] = true
	clientsMutex.Unlock()

	go client.writePump()

	for {
		_, _, err := ws.ReadMessage()
		if err != nil {
			clientsMutex.Lock()
			delete(clients, client)
			clientsMutex.Unlock()
			break
		}
	}
}

// writePump envia mensagens do canal 'send' do cliente para a conexão WebSocket
func (c *Client) writePump() {
	defer c.conn.Close()
	for {
		select {
		case message, ok := <-c.send:
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}
		}
	}
}

// handleMessages envia mensagens do canal de broadcast para todos os clientes conectados
func handleMessages() {
	for {
		message := <-broadcast
		clientsMutex.Lock()
		for client := range clients {
			select {
			case client.send <- message:
			default:
				close(client.send)
				delete(clients, client)
			}
		}
		clientsMutex.Unlock()
	}
}

// executeCommandWebhook executa um comando externo
func executeCommandWebhook(rule CommandWebhookRule, eventDetails map[string]string) {
	cmdArgs := make([]string, len(rule.Args))
	for i, arg := range rule.Args {
		replacedArg := arg
		for k, v := range eventDetails {
			replacedArg = strings.ReplaceAll(replacedArg, fmt.Sprintf("{{%s}}", k), v)
		}
		cmdArgs[i] = replacedArg
	}

	cmd := exec.Command(rule.Command, cmdArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	log.Printf("Executando comando webhook: %s %v", rule.Command, cmdArgs)
	if err := cmd.Run(); err != nil {
		log.Printf("Erro ao executar comando webhook '%s': %v", rule.Command, err)
	}
}

// sendNotificationWebhook envia um POST para a URL de notificação
func sendNotificationWebhook(url string, payload map[string]string) {
	if url == "" {
		return
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Erro ao serializar payload para webhook de notificação: %v", err)
		return
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonPayload))
	if err != nil {
		log.Printf("Erro ao criar requisição para webhook de notificação: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Erro ao enviar webhook de notificação para %s: %v", url, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Webhook de notificação para %s retornou status %d", url, resp.StatusCode)
	} else {
		log.Printf("Webhook de notificação enviado com sucesso para %s", url)
	}
}

// watchFiles monitora o diretório de serviço para mudanças e envia sinal de recarga
func watchFiles(dir string, debounceMs int, excludeDirs []string, notificationWebhookURL string, commandWebhooks []CommandWebhookRule) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatalf("Erro fatal: não foi possível criar o file watcher: %v", err)
	}
	defer watcher.Close()

	var timer *time.Timer
	debounceDuration := time.Duration(debounceMs) * time.Millisecond

	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if strings.HasPrefix(filepath.Base(event.Name), ".") || strings.HasSuffix(event.Name, "~") || strings.HasSuffix(event.Name, ".tmp") {
					continue
				}

				if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Remove) != 0 {
					if timer != nil {
						timer.Stop()
					}
					timer = time.AfterFunc(debounceDuration, func() {
						relPath, err := filepath.Rel(dir, event.Name)
						if err != nil {
							log.Printf("Erro ao obter caminho relativo para %s: %v", event.Name, err)
							return
						}
						urlPath := "/" + strings.ReplaceAll(relPath, string(os.PathSeparator), "/")

						var msgType string
						ext := strings.ToLower(filepath.Ext(event.Name))
						switch ext {
						case ".css":
							msgType = "css-update"
						case ".js":
							msgType = "js-update"
						default:
							msgType = "reload"
						}

						message, _ := json.Marshal(map[string]string{
							"type": msgType,
							"path": urlPath,
						})
						broadcast <- message
						log.Printf("Mudança detectada em %s, enviando %s", event.Name, msgType)

						eventDetails := map[string]string{
							"event_type": "file_change",
							"file_path":  event.Name,
							"rel_path":   relPath,
							"op":         event.Op.String(),
							"timestamp":  time.Now().Format(time.RFC3339),
						}
						sendNotificationWebhook(notificationWebhookURL, eventDetails)

						for _, rule := range commandWebhooks {
							if rule.Event == "file_change" {
								if rule.Path == "" || strings.HasPrefix(relPath, rule.Path) || strings.Contains(relPath, rule.Path) {
									go executeCommandWebhook(rule, eventDetails)
								}
							}
						}
					})
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Printf("Erro do watcher: %v", err)
			}
		}
	}()

	err = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			log.Printf("Erro ao caminhar pelo diretório %s: %v", path, err)
			return nil
		}

		if info.IsDir() {
			for _, exclude := range excludeDirs {
				absExclude, _ := filepath.Abs(filepath.Join(dir, exclude))
				absPath, _ := filepath.Abs(path)
				if strings.HasPrefix(absPath, absExclude) {
					log.Printf("Excluindo diretório do watcher: %s", path)
					return filepath.SkipDir
				}
			}
			err = watcher.Add(path)
			if err != nil {
				log.Printf("Erro ao adicionar watcher para %s: %v", path, err)
			}
		}
		return nil
	})

	if err != nil {
		log.Fatalf("Erro fatal ao configurar o watcher de arquivos: %v", err)
	}

	select {}
}

// loggingMiddleware registra informações sobre cada requisição HTTP
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("[%s] %s %s %s", r.Method, r.URL.Path, r.RemoteAddr, time.Since(start))
	})
}

// noCacheMiddleware adiciona cabeçalhos para prevenir cache em requisições
func noCacheMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")
		next.ServeHTTP(w, r)
	})
}

// corsMiddleware adiciona os cabeçalhos CORS necessários
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Credentials", "true")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
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
	Headers    http.Header
}

func newResponseRecorder(w http.ResponseWriter) *responseRecorder {
	return &responseRecorder{
		ResponseWriter: w,
		StatusCode:     http.StatusOK,
		Body:           new(bytes.Buffer),
		Headers:        make(http.Header),
	}
}

func (r *responseRecorder) Header() http.Header {
	return r.Headers
}

func (r *responseRecorder) WriteHeader(statusCode int) {
	r.StatusCode = statusCode
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	return r.Body.Write(b)
}

func (r *responseRecorder) CopyTo(w http.ResponseWriter) {
	for k, v := range r.Headers {
		w.Header()[k] = v
	}
	w.WriteHeader(r.StatusCode)
	w.Write(r.Body.Bytes())
}

// liveReloadInjector injeta o script de live reload
func liveReloadInjector(injectedJSContent, injectedCSSContent string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ws" || r.Method != http.MethodGet {
			next.ServeHTTP(w, r)
			return
		}

		recorder := newResponseRecorder(w)
		next.ServeHTTP(recorder, r)

		if strings.Contains(recorder.Header().Get("Content-Type"), "text/html") && recorder.StatusCode == http.StatusOK {
			body := recorder.Body.Bytes()

			liveReloadAndHMRScript := fmt.Sprintf(`
            <script>
                var ws = new WebSocket("ws://%s/ws");
                ws.onmessage = function(event) {
                    var message = JSON.parse(event.data);
                    if (message.type === "reload") {
                        location.reload();
                    } else if (message.type === "css-update") {
                        var link = document.querySelector('link[href*="' + message.path + '"]');
                        if (link) {
                            var newHref = message.path + '?v=' + new Date().getTime();
                            link.href = newHref;
                        } else {
                            location.reload();
                        }
                    } else if (message.type === "js-update") {
                        var script = document.querySelector('script[src*="' + message.path + '"]');
                        if (script) {
                            var newScript = document.createElement('script');
                            newScript.src = message.path + '?v=' + new Date().getTime();
                            newScript.async = true;
                            script.parentNode.replaceChild(newScript, script);
                        } else {
                            location.reload();
                        }
                    }
                };
            </script>
            `, r.Host)

			var customInjections bytes.Buffer
			if injectedCSSContent != "" {
				customInjections.WriteString(fmt.Sprintf("<style>\n%s\n</style>\n", injectedCSSContent))
			}
			if injectedJSContent != "" {
				customInjections.WriteString(fmt.Sprintf("<script>\n%s\n</script>\n", injectedJSContent))
			}

			if idx := bytes.LastIndex(body, []byte("</head>")); idx != -1 {
				body = bytes.Join([][]byte{body[:idx], customInjections.Bytes(), body[idx:]}, nil)
			}

			if idx := bytes.LastIndex(body, []byte("</body>")); idx != -1 {
				body = bytes.Join([][]byte{body[:idx], []byte(liveReloadAndHMRScript), body[idx:]}, nil)
			} else {
				body = bytes.Join([][]byte{body, []byte(liveReloadAndHMRScript)}, nil)
			}

			for k, v := range recorder.Headers {
				w.Header()[k] = v
			}
			w.Header().Set("Content-Length", fmt.Sprintf("%d", len(body)))
			w.WriteHeader(recorder.StatusCode)
			w.Write(body)
			return
		}

		recorder.CopyTo(w)
	})
}

// spaFallbackMiddleware serve index.html se o arquivo não for encontrado
func spaFallbackMiddleware(serveDir string, enabled bool, next http.Handler) http.Handler {
	if !enabled {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		recorder := newResponseRecorder(w)
		next.ServeHTTP(recorder, r)

		if recorder.StatusCode == http.StatusNotFound && !strings.Contains(filepath.Base(r.URL.Path), ".") && r.URL.Path != "/ws" {
			indexPath := filepath.Join(serveDir, "index.html")
			if _, err := os.Stat(indexPath); err == nil {
				http.ServeFile(w, r, indexPath)
				return
			}
		}
		recorder.CopyTo(w)
	})
}

// customErrorPageMiddleware serve uma página de erro 404 personalizada
func customErrorPageMiddleware(custom404Path string, serveDir string, next http.Handler) http.Handler {
	if custom404Path == "" {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		recorder := newResponseRecorder(w)
		next.ServeHTTP(recorder, r)
		if recorder.StatusCode == http.StatusNotFound {
			full404Path := filepath.Join(serveDir, custom404Path)
			if _, err := os.Stat(full404Path); err == nil {
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				w.WriteHeader(http.StatusNotFound)
				http.ServeFile(w, r, full404Path)
				return
			}
		}
		recorder.CopyTo(w)
	})
}

// noDirListingFileSystem previne listagem de diretórios
type noDirListingFileSystem struct {
	fs http.FileSystem
}

type noDirListingFile struct {
	http.File
}

func (f noDirListingFile) Readdir(count int) ([]os.FileInfo, error) {
	return nil, os.ErrNotExist
}

func (fs noDirListingFileSystem) Open(name string) (http.File, error) {
	f, err := fs.fs.Open(name)
	if err != nil {
		return nil, err
	}
	stat, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if stat.IsDir() {
		return noDirListingFile{f}, nil
	}
	return f, nil
}

// gzipWriter é um ResponseWriter que comprime a saída usando gzip
type gzipWriter struct {
	http.ResponseWriter
	Writer *gzip.Writer
}

func (g *gzipWriter) Write(data []byte) (int, error) {
	return g.Writer.Write(data)
}

func (g *gzipWriter) WriteHeader(statusCode int) {
	g.ResponseWriter.Header().Del("Content-Length")
	g.ResponseWriter.WriteHeader(statusCode)
}

// gzipMiddleware comprime a resposta
func gzipMiddleware(enabled bool, next http.Handler) http.Handler {
	if !enabled {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

// reverseProxyMiddleware encaminha requisições
func reverseProxyMiddleware(proxyRules []ProxyRule, next http.Handler) http.Handler {
	if len(proxyRules) == 0 {
		return next
	}

	proxies := make(map[string]*httputil.ReverseProxy)
	for _, rule := range proxyRules {
		targetURL, err := url.Parse(rule.Target)
		if err != nil {
			continue
		}
		proxy := httputil.NewSingleHostReverseProxy(targetURL)
		proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
			http.Error(w, "Bad Gateway", http.StatusBadGateway)
		}
		originalDirector := proxy.Director
		proxy.Director = func(req *http.Request) {
			originalDirector(req)
			req.URL.Path = strings.TrimPrefix(req.URL.Path, rule.Path)
		}
		proxies[rule.Path] = proxy
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for pathPrefix, proxy := range proxies {
			if strings.HasPrefix(r.URL.Path, pathPrefix) {
				proxy.ServeHTTP(w, r)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// rewriteRedirectMiddleware lida com reescritas e redirecionamentos
func rewriteRedirectMiddleware(rewrites []RewriteRule, redirects []RedirectRule, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, rule := range redirects {
			if strings.HasPrefix(r.URL.Path, rule.From) {
				newURL := strings.Replace(r.URL.Path, rule.From, rule.To, 1)
				http.Redirect(w, r, newURL, rule.Code)
				return
			}
		}
		for _, rule := range rewrites {
			if strings.HasPrefix(r.URL.Path, rule.From) {
				r.URL.Path = strings.Replace(r.URL.Path, rule.From, rule.To, 1)
				next.ServeHTTP(w, r)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// readInjectedFileContent lê o conteúdo de um arquivo para injeção.
func readInjectedFileContent(filePath string) string {
	if filePath == "" {
		return ""
	}
	content, err := os.ReadFile(filePath)
	if err != nil {
		log.Printf("Aviso: não foi possível ler o arquivo para injeção '%s': %v", filePath, err)
		return ""
	}
	return string(content)
}

// loadConfigFromFile lê a configuração de um arquivo JSON.
func loadConfigFromFile(filePath string, cfg *Config) error {
	if filePath == "" {
		return nil
	}
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("Erro: não foi possível ler o arquivo de configuração '%s': %w", filePath, err)
	}
	if err := json.Unmarshal(data, cfg); err != nil {
		return fmt.Errorf("Erro: não foi possível parsear o JSON do arquivo de configuração '%s': %w", filePath, err)
	}
	return nil
}

// apiAuthMiddleware verifica o token de autenticação para endpoints da API
func apiAuthMiddleware(apiToken string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if apiToken == "" {
			next.ServeHTTP(w, r)
			return
		}

		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Authorization header required", http.StatusUnauthorized)
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			http.Error(w, "Invalid Authorization header format. Expected 'Bearer <token>'", http.StatusUnauthorized)
			return
		}

		if parts[1] != apiToken {
			http.Error(w, "Invalid API token", http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func main() {
	var (
		configFilePathFlag         = flag.String("config", "", "Caminho para um arquivo de configuração JSON (ex: config.json).")
		portFlag                   = flag.Int("port", 5571, "Porta para o servidor HTTP")
		serveDirFlag               = flag.String("dir", "www", "Diretório para servir arquivos estáticos")
		injectJSPathFlag           = flag.String("inject-js", "", "Caminho para um arquivo JavaScript a ser injetado.")
		injectCSSPathFlag          = flag.String("inject-css", "", "Caminho para um arquivo CSS a ser injetado.")
		spaFallbackEnabledFlag     = flag.Bool("spa-fallback", false, "Habilita o fallback para index.html para SPAs.")
		dirListingEnabledFlag      = flag.Bool("enable-dir-listing", false, "Habilita a listagem de diretórios.")
		gzipEnabledFlag            = flag.Bool("enable-gzip", false, "Habilita a compressão Gzip.")
		custom404PagePathFlag      = flag.String("404-page", "", "Caminho para uma página 404 personalizada.")
		watchDebounceMsFlag        = flag.Int("watch-debounce-ms", 100, "Tempo de debounce para o watcher (ms).")
		watchExcludeDirsFlag       = flag.String("watch-exclude-dirs", "", "Diretórios para excluir do watcher (separados por vírgula).")
		logFilePathFlag            = flag.String("log-file", "server.log", "Caminho para o arquivo de log. Padrão: server.log")
		apiTokenFlag               = flag.String("api-token", "", "Token de autenticação para a API.")
		notificationWebhookURLFlag = flag.String("notification-webhook-url", "", "URL para webhooks de notificação.")
	)

	flag.Parse()

	cfg := Config{
		Port:                   *portFlag,
		ServeDir:               *serveDirFlag,
		InjectJSPath:           *injectJSPathFlag,
		InjectCSSPath:          *injectCSSPathFlag,
		SPAFallbackEnabled:     *spaFallbackEnabledFlag,
		DirListingEnabled:      *dirListingEnabledFlag,
		GzipEnabled:            *gzipEnabledFlag,
		Custom404PagePath:      *custom404PagePathFlag,
		ProxyRules:             []ProxyRule{},
		Rewrites:               []RewriteRule{},
		Redirects:              []RedirectRule{},
		WatchDebounceMs:        *watchDebounceMsFlag,
		WatchExcludeDirs:       []string{},
		LogFilePath:            *logFilePathFlag,
		APIToken:               *apiTokenFlag,
		NotificationWebhookURL: *notificationWebhookURLFlag,
		CommandWebhooks:        []CommandWebhookRule{},
	}

	if *watchExcludeDirsFlag != "" {
		cfg.WatchExcludeDirs = strings.Split(*watchExcludeDirsFlag, ",")
	}
	
	if *configFilePathFlag != "" {
		if err := loadConfigFromFile(*configFilePathFlag, &cfg); err != nil {
			log.Fatalf("Erro ao carregar configuração: %v", err)
		}
	}
	
	// Re-aplicar flags para garantir precedência
	cfg.Port = *portFlag
	cfg.ServeDir = *serveDirFlag
	cfg.InjectJSPath = *injectJSPathFlag
	cfg.InjectCSSPath = *injectCSSPathFlag
	cfg.SPAFallbackEnabled = *spaFallbackEnabledFlag
	cfg.DirListingEnabled = *dirListingEnabledFlag
	cfg.GzipEnabled = *gzipEnabledFlag
	cfg.Custom404PagePath = *custom404PagePathFlag
	cfg.WatchDebounceMs = *watchDebounceMsFlag
	if *watchExcludeDirsFlag != "" {
		cfg.WatchExcludeDirs = strings.Split(*watchExcludeDirsFlag, ",")
	}
	cfg.LogFilePath = *logFilePathFlag
	cfg.APIToken = *apiTokenFlag
	cfg.NotificationWebhookURL = *notificationWebhookURLFlag


	if cfg.LogFilePath != "" {
		logFile, err := os.OpenFile(cfg.LogFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			log.Fatalf("Erro fatal: não foi possível abrir o arquivo de log '%s': %v", cfg.LogFilePath, err)
		}
		log.SetOutput(logFile)
	}

	if _, err := os.Stat(cfg.ServeDir); os.IsNotExist(err) {
		log.Fatalf("Erro fatal: Diretório a ser servido '%s' não encontrado. Por favor, crie-o ou especifique um diretório válido.", cfg.ServeDir)
	}
    
	injectedJSContent := readInjectedFileContent(cfg.InjectJSPath)
	injectedCSSContent := readInjectedFileContent(cfg.InjectCSSPath)

	go handleMessages()
	go watchFiles(cfg.ServeDir, cfg.WatchDebounceMs, cfg.WatchExcludeDirs, cfg.NotificationWebhookURL, cfg.CommandWebhooks)

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", handleConnections)

	apiMux := http.NewServeMux()
	apiMux.HandleFunc("/api/reload", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost { http.Error(w, "Método não permitido", http.StatusMethodNotAllowed); return }
		message, _ := json.Marshal(map[string]string{"type": "reload"}); broadcast <- message
		w.WriteHeader(http.StatusOK); w.Write([]byte("Live reload disparado!"))
	})
	apiMux.HandleFunc("/api/status", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet { http.Error(w, "Método não permitido", http.StatusMethodNotAllowed); return }
		status := map[string]interface{}{"status": "running", "uptime": time.Since(serverStartTime).String(), "port": cfg.Port, "serve_dir": cfg.ServeDir, "connected_clients": len(clients)}
		json.NewEncoder(w).Encode(status)
	})
	apiMux.HandleFunc("/api/command", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost { http.Error(w, "Método não permitido", http.StatusMethodNotAllowed); return }
		var req struct { Command string `json:"command"`; Args []string `json:"args"` }
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil { http.Error(w, "Requisição inválida", http.StatusBadRequest); return }
		go func() {
			cmd := exec.Command(req.Command, req.Args...); cmd.Stdout = os.Stdout; cmd.Stderr = os.Stderr
			log.Printf("Executando comando via API: %s %v", req.Command, req.Args)
			if err := cmd.Run(); err != nil { log.Printf("Erro ao executar comando via API '%s': %v", req.Command, err) }
		}(); w.WriteHeader(http.StatusOK); w.Write([]byte("Comando enviado para execução."))
	})
	mux.Handle("/api/", apiAuthMiddleware(cfg.APIToken, apiMux))

	var fileServerHandler http.Handler
	if cfg.DirListingEnabled { fileServerHandler = http.FileServer(http.Dir(cfg.ServeDir)) } else { fileServerHandler = http.FileServer(noDirListingFileSystem{http.Dir(cfg.ServeDir)}) }

	handler := fileServerHandler
	handler = customErrorPageMiddleware(cfg.Custom404PagePath, cfg.ServeDir, handler)
	handler = spaFallbackMiddleware(cfg.ServeDir, cfg.SPAFallbackEnabled, handler)
	handler = liveReloadInjector(injectedJSContent, injectedCSSContent, handler)
	handler = reverseProxyMiddleware(cfg.ProxyRules, handler)
	handler = rewriteRedirectMiddleware(cfg.Rewrites, cfg.Redirects, handler)
	handler = corsMiddleware(handler)
	handler = noCacheMiddleware(handler)
	handler = gzipMiddleware(cfg.GzipEnabled, handler)
	handler = loggingMiddleware(handler)
	mux.Handle("/", handler)

	addr := fmt.Sprintf(":%d", cfg.Port)

	log.Printf("🚀 Servidor iniciado em http://localhost%s", addr)
	log.Printf("   Servindo diretório: %s", cfg.ServeDir)
	log.Printf("   Live Reload: Ativado")
    if cfg.LogFilePath != "" {
        log.Printf("   Logs sendo gravados em: %s", cfg.LogFilePath)
    }

	for _, rule := range cfg.CommandWebhooks {
		if rule.Event == "server_start" {
			go executeCommandWebhook(rule, map[string]string{ "timestamp": time.Now().Format(time.RFC3339), "port": fmt.Sprintf("%d", cfg.Port), "serve_dir": cfg.ServeDir, })
		}
	}

	log.Fatal(http.ListenAndServe(addr, mux))

	for _, rule := range cfg.CommandWebhooks {
		if rule.Event == "server_stop" {
			go executeCommandWebhook(rule, map[string]string{ "timestamp": time.Now().Format(time.RFC3339), "port": fmt.Sprintf("%d", cfg.Port), "serve_dir": cfg.ServeDir, })
		}
	}
}