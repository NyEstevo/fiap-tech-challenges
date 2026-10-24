package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestChaosInjector(t *testing.T) {
	c := NewChaosInjector()
	for range 100 {
		if c.ShouldFail() {
			t.Fatal("desligado nunca deveria falhar")
		}
	}

	if !c.Enable(100) {
		t.Fatal("Enable(100) deveria ser aceito")
	}
	for range 100 {
		if !c.ShouldFail() {
			t.Fatal("com 100% deveria sempre falhar")
		}
	}

	c.Disable()
	if enabled, _ := c.Status(); enabled || c.ShouldFail() {
		t.Fatal("Disable deveria desligar")
	}

	for _, rate := range []int{-1, 101} {
		if c.Enable(rate) {
			t.Fatalf("taxa %d deveria ser recusada", rate)
		}
	}
	if enabled, _ := c.Status(); enabled {
		t.Fatal("taxa invalida nao pode ligar o chaos")
	}
}

func TestChaosInjector_PartialRate(t *testing.T) {
	c := NewChaosInjector()
	c.Enable(50)
	fails := 0
	for range 2000 {
		if c.ShouldFail() {
			fails++
		}
	}
	if fails < 800 || fails > 1200 {
		t.Fatalf("esperado ~50%% de falhas, veio %d/2000", fails)
	}
}

// Com o chaos ligado, rotas comuns dao 500, mas /health (probes do
// Kubernetes) e /admin/chaos (para desligar) seguem respondendo.
func TestChaosMiddleware(t *testing.T) {
	c := NewChaosInjector()
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	h := c.Middleware(ok)

	call := func(path string) int {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		return w.Code
	}

	if got := call("/validate"); got != http.StatusOK {
		t.Fatalf("chaos desligado: /validate = %d, esperado 200", got)
	}

	c.Enable(100)
	cases := map[string]int{
		"/validate":    http.StatusInternalServerError,
		"/admin/keys":  http.StatusInternalServerError,
		"/health":      http.StatusOK,
		"/admin/chaos": http.StatusOK,
	}
	for path, want := range cases {
		if got := call(path); got != want {
			t.Errorf("chaos ligado: %s = %d, esperado %d", path, got, want)
		}
	}
}

func TestChaosHandler(t *testing.T) {
	app := &App{MasterKey: "mk", Chaos: NewChaosInjector()}
	h := app.masterKeyAuthMiddleware(http.HandlerFunc(app.chaosHandler))

	call := func(method, key, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "/admin/chaos", strings.NewReader(body))
		r.Header.Set("Authorization", "Bearer "+key)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}

	if w := call(http.MethodPost, "errada", ""); w.Code != http.StatusForbidden {
		t.Fatalf("sem MASTER_KEY: %d, esperado 403", w.Code)
	}
	if enabled, _ := app.Chaos.Status(); enabled {
		t.Fatal("requisicao sem MASTER_KEY nao pode ligar o chaos")
	}

	if w := call(http.MethodPost, "mk", ""); w.Code != http.StatusOK {
		t.Fatalf("POST sem corpo: %d", w.Code)
	}
	if enabled, rate := app.Chaos.Status(); !enabled || rate != 100 {
		t.Fatalf("POST sem corpo deveria ligar com 100%%, veio %v/%d", enabled, rate)
	}

	if w := call(http.MethodPost, "mk", `{"error_rate_percent": 30}`); !strings.Contains(w.Body.String(), `"error_rate_percent":30`) {
		t.Fatalf("POST 30%%: corpo %s", w.Body.String())
	}

	if w := call(http.MethodPost, "mk", `{"error_rate_percent": 150}`); w.Code != http.StatusBadRequest {
		t.Fatalf("taxa 150: %d, esperado 400", w.Code)
	}

	if w := call(http.MethodDelete, "mk", ""); !strings.Contains(w.Body.String(), `"enabled":false`) {
		t.Fatalf("DELETE: corpo %s", w.Body.String())
	}
}
