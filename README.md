
# brhttp — Servidor Web Estático Minimalista em Go

<p align="left">
  <img src="https://img.shields.io/badge/vers%C3%A3o-v1.0-blue.svg" alt="Versão" />
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

- **Performance extrema:** Servidor leve, sem processamento dinâmico
- **Zero configuração:** Execute e já está funcionando
- **Segurança automática:** Impede listagem de diretórios
- **Binário único:** Sem dependências externas para deploy
- **Desligamento suave:** Finalização controlada via sinais do sistema

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

---

## 💡 Funcionamento Interno (Resumo)

| Arquivo Principal | Pasta Servida | Segurança                | Logs e Middlewares      | Desligamento Suave                    |
|:-----------------:|:-------------:|:------------------------:|:-----------------------:|:------------------------------------:|
| `main.go`         | `www/`        | Bloqueio da listagem     | Logging detalhado       | Aguarda 5 segundos após sinal do SO  |

---

## Código principal — `main.go`

```go
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

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		log.Printf("--> [%s] %s %s", r.RemoteAddr, r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
		log.Printf("<-- Finalizado %s em %v", r.URL.Path, time.Since(start))
	})
}

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
	fileServer := http.FileServer(http.Dir("./www"))
	handler := loggingMiddleware(noDirListing(fileServer))

	server := &http.Server{
		Addr:    "[::]:5571",
		Handler: handler,
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Println("🚀 Servidor 'brhttp' iniciado. Acesse http://localhost:5571")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Erro ao iniciar servidor: %v", err)
		}
	}()

	<-quit
	log.Println("... Servidor recebendo sinal para desligar ...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Erro no desligamento: %v", err)
	}

	log.Println("✅ Servidor desligado com sucesso.")
}
```

---

## 🧠 O que o código faz?

- Serve arquivos estáticos do diretório `www` na porta `5571`
- Loga cada requisição com IP, método, rota e tempo de resposta
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
