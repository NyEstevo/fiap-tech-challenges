package main

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/XSAM/otelsql"
	_ "github.com/jackc/pgx/v4/stdlib"
	"github.com/joho/godotenv"
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
)

// App struct (para injeção de dependência)
type App struct {
	DB        *sql.DB
	MasterKey string
	Chaos     *ChaosInjector
}

func main() {
	// Carrega o .env para desenvolvimento local. Em produção, isso não fará nada.
	_ = godotenv.Load()

	// SIGTERM (rollout/restart do Kubernetes) cancela este contexto.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	shutdownTelemetry, err := setupTelemetry(ctx)
	if err != nil {
		log.Fatalf("Não foi possível iniciar a telemetria: %v", err)
	}

	// --- Configuração ---
	port := os.Getenv("PORT")
	if port == "" {
		port = "8001" // Porta padrão
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL deve ser definida")
	}

	masterKey := os.Getenv("MASTER_KEY")
	if masterKey == "" {
		log.Fatal("MASTER_KEY deve ser definida")
	}

	// --- Conexão com o Banco ---
	db, err := connectDB(databaseURL)
	if err != nil {
		log.Fatalf("Não foi possível conectar ao banco de dados: %v disparo do argo", err)
	}
	defer db.Close()

	app := &App{
		DB:        db,
		MasterKey: masterKey,
		Chaos:     NewChaosInjector(),
	}

	// --- Rotas da API ---
	mux := http.NewServeMux()
	mux.HandleFunc("/health", app.healthHandler)

	// Endpoint público para validar uma chave
	mux.HandleFunc("/validate", app.validateKeyHandler)

	// Endpoints de "admin" para criar/gerenciar chaves
	// Eles são protegidos pelo middleware de autenticação
	mux.Handle("/admin/keys", app.masterKeyAuthMiddleware(http.HandlerFunc(app.createKeyHandler)))

	// Injecao de falha da fase-4 (chaos.go): so existe com CHAOS_ENABLED=true.
	var handler http.Handler = mux
	if os.Getenv("CHAOS_ENABLED") == "true" {
		mux.Handle("/admin/chaos", app.masterKeyAuthMiddleware(http.HandlerFunc(app.chaosHandler)))
		handler = app.Chaos.Middleware(mux)
	}

	srv := &http.Server{Addr: ":" + port, Handler: instrumentHandler(handler)}
	go func() {
		log.Printf("Serviço de Autenticação (Go) rodando na porta %s", port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	}()

	// Desligamento gracioso: termina as requisicoes em andamento e esvazia os
	// buffers de traces/metricas -- o rollout restart do self-healing nao
	// perde a telemetria do momento do incidente.
	<-ctx.Done()
	log.Println("Sinal de parada recebido, desligando...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("Erro ao desligar o servidor HTTP: %v", err)
	}
	if err := shutdownTelemetry(shutdownCtx); err != nil {
		log.Printf("Erro ao desligar a telemetria: %v", err)
	}
}

// connectDB inicializa e testa a conexão com o PostgreSQL
func connectDB(databaseURL string) (*sql.DB, error) {
	// Cada query vira um span filho da requisicao (desde que o handler use os
	// metodos *Context). db.system (convencao antiga, a dos servicos Python)
	// + db.system.name (nova): o banco aparece igual no APM para os 5.
	attrs := []attribute.KeyValue{semconv.DBSystemNamePostgreSQL, attribute.String("db.system", "postgresql")}
	if u, err := url.Parse(databaseURL); err == nil {
		attrs = append(attrs, semconv.ServerAddress(u.Hostname()), semconv.DBNamespace(u.Path[min(1, len(u.Path)):]))
	}
	db, err := otelsql.Open("pgx", databaseURL,
		otelsql.WithAttributes(attrs...),
		// So o que importa no trace: a query. Sem spans de rows/prepare/reset.
		otelsql.WithSpanOptions(otelsql.SpanOptions{OmitRows: true, OmitConnPrepare: true, OmitConnResetSession: true}),
	)
	if err != nil {
		return nil, err
	}

	if err = db.Ping(); err != nil {
		return nil, err
	}

	log.Println("Conectado ao PostgreSQL com sucesso!")
	return db, nil
}
