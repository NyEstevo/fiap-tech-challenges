package main

// Injecao de falha para a "prova real" da fase-4 (fatia 11).
//
// Com CHAOS_ENABLED=true o servico ganha a rota /admin/chaos (protegida pela
// MASTER_KEY) que liga um estado EM MEMORIA de "responder 500". /health segue
// 200 -- e a falha silenciosa do enunciado: o Kubernetes acha que esta tudo
// bem, so as metricas/traces mostram. Como o estado vive so na memoria do
// processo, o rollout restart do self-healing realmente cura.

import (
	"encoding/json"
	"log"
	"math/rand/v2"
	"net/http"
	"sync"
)

// ChaosInjector guarda se o chaos esta ligado e com qual % de falha.
type ChaosInjector struct {
	mu               sync.Mutex
	enabled          bool
	errorRatePercent int
}

func NewChaosInjector() *ChaosInjector {
	return &ChaosInjector{errorRatePercent: 100}
}

// Enable liga o chaos; taxa fora de 0-100 e recusada.
func (c *ChaosInjector) Enable(errorRatePercent int) bool {
	if errorRatePercent < 0 || errorRatePercent > 100 {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.enabled = true
	c.errorRatePercent = errorRatePercent
	log.Printf("chaos ligado (taxa de erro: %d%%)", errorRatePercent)
	return true
}

func (c *ChaosInjector) Disable() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.enabled = false
	log.Println("chaos desligado")
}

func (c *ChaosInjector) Status() (enabled bool, errorRatePercent int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.enabled, c.errorRatePercent
}

// ShouldFail sorteia se esta requisicao falha. Nao precisa de crypto/rand:
// e so uma proporcao de falhas, nada de seguranca depende do sorteio.
func (c *ChaosInjector) ShouldFail() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.enabled && rand.IntN(100) < c.errorRatePercent // #nosec G404 -- proporcao de falhas da demo, nao e segredo
}

// Middleware devolve 500 nas requisicoes sorteadas. Fica DENTRO do
// instrumentHandler (otelhttp), para os 500 entrarem na metrica
// http.server.request.duration e nos traces. /health e /admin/chaos nunca
// falham: um mantem o pod "saudavel" para o Kubernetes, o outro permite
// desligar o chaos.
func (c *ChaosInjector) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/health" && r.URL.Path != "/admin/chaos" && c.ShouldFail() {
			log.Printf("chaos: 500 injetado em %s %s", r.Method, r.URL.Path)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":"falha injetada (chaos)"}`))
			return
		}
		next.ServeHTTP(w, r)
	})
}

// chaosHandler: GET consulta, POST liga ({"error_rate_percent": 0-100,
// padrao 100}), DELETE desliga.
func (a *App) chaosHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
	case http.MethodPost:
		req := struct {
			ErrorRatePercent int `json:"error_rate_percent"`
		}{ErrorRatePercent: 100}
		if r.Body != nil {
			_ = json.NewDecoder(r.Body).Decode(&req) // corpo vazio = padrao 100
		}
		if !a.Chaos.Enable(req.ErrorRatePercent) {
			http.Error(w, "error_rate_percent deve estar entre 0 e 100", http.StatusBadRequest)
			return
		}
	case http.MethodDelete:
		a.Chaos.Disable()
	default:
		http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
		return
	}

	enabled, rate := a.Chaos.Status()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"enabled":            enabled,
		"error_rate_percent": rate,
	})
}
