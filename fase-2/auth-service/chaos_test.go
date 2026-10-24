package main

import (
	"bytes"
	"encoding/json"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestChaosInjector_Disabled(t *testing.T) {
	c := NewChaosInjector()

	// Quando desligado, ShouldFail sempre retorna false
	for i := 0; i < 100; i++ {
		if c.ShouldFail() {
			t.Fatal("ChaosInjector desligado deveria sempre retornar false")
		}
	}
}

func TestChaosInjector_Enable(t *testing.T) {
	c := NewChaosInjector()

	// Liga com 100% de taxa
	c.Enable(100)

	enabled, errorRate := c.Status()
	if !enabled {
		t.Fatal("ChaosInjector deveria estar ligado")
	}
	if errorRate != 100 {
		t.Fatalf("Taxa de erro esperada 100, got %d", errorRate)
	}

	// Quando ligado com 100%, ShouldFail sempre retorna true
	for i := 0; i < 100; i++ {
		if !c.ShouldFail() {
			t.Fatal("ChaosInjector com 100% deveria sempre retornar true")
		}
	}
}

func TestChaosInjector_PartialRate(t *testing.T) {
	c := NewChaosInjector()

	// Liga com 50% de taxa, com seed determinística
	rand.Seed(12345)
	c.Enable(50)

	// Com seed determinística, devemos ter aproximadamente 50% de falhas
	failCount := 0
	iterations := 1000
	for i := 0; i < iterations; i++ {
		if c.ShouldFail() {
			failCount++
		}
	}

	// Esperamos ~500 falhas (50%), com tolerância de ±100
	if failCount < 400 || failCount > 600 {
		t.Fatalf("Taxa de erro esperada ~50%%, got %d/%d (%.1f%%)", failCount, iterations, float64(failCount)*100/float64(iterations))
	}
}

func TestChaosInjector_Disable(t *testing.T) {
	c := NewChaosInjector()

	c.Enable(100)
	enabled, _ := c.Status()
	if !enabled {
		t.Fatal("ChaosInjector deveria estar ligado após Enable(100)")
	}

	c.Disable()
	enabled, _ = c.Status()
	if enabled {
		t.Fatal("ChaosInjector deveria estar desligado após Disable()")
	}

	// Após desligar, ShouldFail deve retornar false
	for i := 0; i < 100; i++ {
		if c.ShouldFail() {
			t.Fatal("ChaosInjector desligado deveria sempre retornar false")
		}
	}
}

func TestChaosHandler_GetStatus(t *testing.T) {
	app := &App{
		DB:        nil,
		MasterKey: "test-master-key",
		Chaos:     NewChaosInjector(),
	}

	// Cria requisição GET
	req := httptest.NewRequest("GET", "/admin/chaos", nil)
	req.Header.Set("Authorization", "Bearer test-master-key")
	w := httptest.NewRecorder()

	app.chaosHandler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Status code esperado 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Erro ao decodificar resposta: %v", err)
	}

	if enabled, ok := resp["enabled"].(bool); !ok || enabled {
		t.Fatal("chaos deveria estar desligado por padrão")
	}
}

func TestChaosHandler_Enable(t *testing.T) {
	app := &App{
		DB:        nil,
		MasterKey: "test-master-key",
		Chaos:     NewChaosInjector(),
	}

	// Cria requisição POST com taxa customizada
	body := []byte(`{"error_rate_percent": 75}`)
	req := httptest.NewRequest("POST", "/admin/chaos", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer test-master-key")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	app.chaosHandler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Status code esperado 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Erro ao decodificar resposta: %v", err)
	}

	if enabled, ok := resp["enabled"].(bool); !ok || !enabled {
		t.Fatal("chaos deveria estar ligado")
	}

	if rate, ok := resp["error_rate_percent"].(float64); !ok || rate != 75 {
		t.Fatalf("Taxa de erro esperada 75, got %v", resp["error_rate_percent"])
	}

	// Verifica que o estado interno foi atualizado
	isEnabled, errorRate := app.Chaos.Status()
	if !isEnabled || errorRate != 75 {
		t.Fatalf("Estado do chaos não foi atualizado corretamente: %v, %d", isEnabled, errorRate)
	}
}

func TestChaosHandler_Disable(t *testing.T) {
	app := &App{
		DB:        nil,
		MasterKey: "test-master-key",
		Chaos:     NewChaosInjector(),
	}

	// Primeiro liga
	app.Chaos.Enable(100)

	// Depois desliga via DELETE
	req := httptest.NewRequest("DELETE", "/admin/chaos", nil)
	req.Header.Set("Authorization", "Bearer test-master-key")
	w := httptest.NewRecorder()

	app.chaosHandler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Status code esperado 200, got %d", w.Code)
	}

	// Verifica que o estado foi atualizado
	isEnabled, _ := app.Chaos.Status()
	if isEnabled {
		t.Fatal("chaos deveria estar desligado após DELETE")
	}
}

func TestChaosHandler_ForbiddenWithoutMasterKey(t *testing.T) {
	app := &App{
		DB:        nil,
		MasterKey: "correct-master-key",
		Chaos:     NewChaosInjector(),
	}

	tests := []string{"GET", "POST", "DELETE"}
	for _, method := range tests {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/admin/chaos", nil)
			// Usa uma chave incorreta
			req.Header.Set("Authorization", "Bearer wrong-key")
			w := httptest.NewRecorder()

			// Simula o middleware de autenticação
			middleware := app.masterKeyAuthMiddleware(http.HandlerFunc(app.chaosHandler))
			middleware.ServeHTTP(w, req)

			if w.Code != http.StatusForbidden {
				t.Fatalf("Status code esperado 403, got %d para método %s", w.Code, method)
			}
		})
	}
}

func TestChaosHandler_InvalidErrorRate(t *testing.T) {
	app := &App{
		DB:        nil,
		MasterKey: "test-master-key",
		Chaos:     NewChaosInjector(),
	}

	tests := []int{-1, 101, 1000}
	for _, rate := range tests {
		t.Run("rate_"+string(rune(rate)), func(t *testing.T) {
			body, _ := json.Marshal(map[string]int{"error_rate_percent": rate})
			req := httptest.NewRequest("POST", "/admin/chaos", bytes.NewReader(body))
			req.Header.Set("Authorization", "Bearer test-master-key")
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			app.chaosHandler(w, req)

			// A taxa inválida é rejeitada, chaos permanece desligado
			isEnabled, _ := app.Chaos.Status()
			if isEnabled {
				t.Fatalf("chaos deveria estar desligado com taxa inválida %d", rate)
			}
		})
	}
}

func TestChaosHandler_DefaultErrorRate(t *testing.T) {
	app := &App{
		DB:        nil,
		MasterKey: "test-master-key",
		Chaos:     NewChaosInjector(),
	}

	// POST sem especificar error_rate_percent
	req := httptest.NewRequest("POST", "/admin/chaos", bytes.NewReader([]byte("{}")))
	req.Header.Set("Authorization", "Bearer test-master-key")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	app.chaosHandler(w, req)

	_, errorRate := app.Chaos.Status()
	if errorRate != 100 {
		t.Fatalf("Taxa de erro esperada 100 (default), got %d", errorRate)
	}
}
