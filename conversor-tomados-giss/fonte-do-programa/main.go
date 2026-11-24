// ConversorTomadosGISS.exe: abre o Conversor de Tomados GISS no navegador e o mantem atualizado.
// Cada vez que a pagina e aberta ou recarregada (F5), baixa a versao mais nova do GitHub
// (riqueleomel1-ui/giss-atualizacoes). Sem internet, usa a ultima versao baixada ou a que vem dentro do programa.
package main

import (
	"bytes"
	_ "embed"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"time"
)

//go:embed index.html
var embutido []byte

// Porta fixa: as inscricoes municipais e ajustes ficam guardados no navegador por endereco.
// (A Calculadora DIFAL usa a 47321.)
const (
	endereco = "127.0.0.1:47322"
	pagina   = "http://127.0.0.1:47322/"
	marca    = "conversor-tomados-giss"
)

var fontes = []string{
	"https://raw.githubusercontent.com/riqueleomel1-ui/giss-atualizacoes/main/conversor-tomados-giss/index.html",
	"https://cdn.jsdelivr.net/gh/riqueleomel1-ui/giss-atualizacoes@main/conversor-tomados-giss/index.html",
}

var (
	mu        sync.Mutex
	ultimoUso = time.Now()
	cliente   = &http.Client{Timeout: 10 * time.Second}
)

func pastaCache() string {
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		base = os.TempDir()
	}
	d := filepath.Join(base, "ConversorTomadosGISS")
	_ = os.MkdirAll(d, 0o755)
	return d
}

func valida(b []byte) bool {
	return len(b) > 10000 && bytes.Contains(b, []byte("Conversor de Tomados GISS")) && bytes.Contains(b, []byte("</html>"))
}

func baixar() []byte {
	for _, u := range fontes {
		req, err := http.NewRequest("GET", u+"?t="+strconv.FormatInt(time.Now().UnixNano(), 36), nil)
		if err != nil {
			continue
		}
		req.Header.Set("Cache-Control", "no-cache")
		resp, err := cliente.Do(req)
		if err != nil {
			continue
		}
		b, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
		resp.Body.Close()
		if err == nil && resp.StatusCode == http.StatusOK && valida(b) {
			return b
		}
	}
	return nil
}

func conteudo() []byte {
	cache := filepath.Join(pastaCache(), "index.html")
	if b := baixar(); b != nil {
		tmp := cache + ".tmp"
		if os.WriteFile(tmp, b, 0o644) == nil {
			_ = os.Rename(tmp, cache)
		}
		return b
	}
	if b, err := os.ReadFile(cache); err == nil && valida(b) {
		return b
	}
	return embutido
}

func servir(w http.ResponseWriter, r *http.Request) {
	mu.Lock()
	ultimoUso = time.Now()
	mu.Unlock()
	switch r.URL.Path {
	case "/", "/index.html":
		b := conteudo()
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
		_, _ = w.Write(b)
	case "/__quem":
		_, _ = io.WriteString(w, marca)
	default:
		http.NotFound(w, r)
	}
}

func abrir(u string) {
	_ = exec.Command("rundll32", "url.dll,FileProtocolHandler", u).Start()
}

// Outra copia do programa ja esta rodando nesta porta?
func jaAberto() bool {
	c := &http.Client{Timeout: 2 * time.Second}
	resp, err := c.Get(pagina + "__quem")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 100))
	return string(b) == marca
}

func main() {
	ln, err := net.Listen("tcp", endereco)
	if err != nil {
		if jaAberto() {
			abrir(pagina)
			return
		}
		// porta ocupada por outro programa: abre uma copia local (sem atualizar no F5)
		f := filepath.Join(pastaCache(), "Conversor_Tomados_GISS.html")
		if os.WriteFile(f, conteudo(), 0o644) == nil {
			abrir(f)
		}
		return
	}
	// fecha sozinho depois de 12 horas sem uso
	go func() {
		for {
			time.Sleep(10 * time.Minute)
			mu.Lock()
			ocioso := time.Since(ultimoUso)
			mu.Unlock()
			if ocioso > 12*time.Hour {
				os.Exit(0)
			}
		}
	}()
	go func() {
		time.Sleep(300 * time.Millisecond)
		abrir(pagina)
	}()
	_ = http.Serve(ln, http.HandlerFunc(servir))
}
