// Versão: 1.0
// Descrição: Servidor web estático puro, focado em performance e simplicidade.
// Autor: Henrique Tourinho
// Data: 22 de Junho de 2025
// Repositório: https://github.com/henriquetourinho/brhttp

package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

// loggingMiddleware registra detalhes de cada requisição.
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		log.Printf("--> [%s] %s %s", r.RemoteAddr, r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
		log.Printf("<-- Finalizado %s em %v", r.URL.Path, time.Since(start))
	})
}

// noDirListing impede que o servidor liste o conteúdo dos diretórios.
func noDirListing(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/") && len(r.URL.Path) > 1 {
			http.NotFound(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func main() {
	// Cria o handler base para servir arquivos do diretório 'www'.
	fileServer := http.FileServer(http.Dir("./www"))

	// Aplica os middlewares.
	handler := loggingMiddleware(noDirListing(fileServer))

	// Configuração do servidor com desligamento suave.
	server := &http.Server{
		Addr:    "[::]:5571",
		Handler: handler,
	}

	// Canal para escutar sinais do sistema operacional (como Ctrl+C).
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// Inicia o servidor em uma goroutine.
	go func() {
		log.Println("🚀 Servidor 'brhttp' iniciado. Escutando em http://localhost:5571")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Erro ao iniciar o servidor: %v", err)
		}
	}()

	// Bloqueia a execução até que um sinal de desligamento seja recebido.
	<-quit
	log.Println("... Servidor recebendo sinal para desligar ...")

	// Dá ao servidor 5 segundos para terminar as requisições ativas.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Erro no desligamento do servidor: %v", err)
	}

	log.Println("✅ Servidor desligado com sucesso.")
}