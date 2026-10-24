package main

import (
	"log"
	"math/rand"
	"sync"
	"time"
)

// ChaosInjector gerencia o estado de injeção de falhas em memória.
// Todas as operações são thread-safe.
type ChaosInjector struct {
	mu               sync.Mutex
	enabled          bool
	errorRatePercent int // 0-100: porcentagem de requisições que devem falhar
}

// NewChaosInjector cria uma nova instância do injector desligado.
func NewChaosInjector() *ChaosInjector {
	return &ChaosInjector{
		enabled:          false,
		errorRatePercent: 100, // Default: todas as requisições falham quando ligado
	}
}

// Enable liga o chaos com uma taxa de erro especificada (0-100).
func (c *ChaosInjector) Enable(errorRatePercent int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if errorRatePercent < 0 || errorRatePercent > 100 {
		log.Printf("Taxa de erro inválida: %d (deve estar entre 0 e 100)", errorRatePercent)
		return
	}
	c.enabled = true
	c.errorRatePercent = errorRatePercent
	log.Printf("Chaos injetado ligado (taxa de erro: %d%%)", errorRatePercent)
}

// Disable desliga o chaos.
func (c *ChaosInjector) Disable() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.enabled = false
	log.Println("Chaos injetado desligado")
}

// Status retorna o estado atual do chaos.
func (c *ChaosInjector) Status() (enabled bool, errorRate int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.enabled, c.errorRatePercent
}

// ShouldFail retorna true se a requisição deve falhar com 500 baseado na taxa configurada.
// Usa rand.Intn, que é determinístico em testes se a seed for definida.
func (c *ChaosInjector) ShouldFail() bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.enabled {
		return false
	}

	// Usa rand com seed baseada no nanotime (não determinístico em produção).
	// Em testes, o teste pode definir a seed para ter comportamento previsível.
	// Não é necessário usar crypto/rand aqui: o objetivo é apenas gerar uma taxa aleatória,
	// não gerar uma sequência criptograficamente segura. G404 é esperado e aceitável.
	return rand.Intn(100) < c.errorRatePercent //nolint:gosec
}

// init() inicializa a seed do rand com o tempo atual.
func init() {
	rand.Seed(time.Now().UnixNano())
}
