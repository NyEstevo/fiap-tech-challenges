package main

// Instrumentacao OpenTelemetry (traces + metricas via OTLP).
//
// Toda a configuracao vem das variaveis padrao do OTel, definidas no cluster
// (fase-3/components/otel): OTEL_SERVICE_NAME, OTEL_EXPORTER_OTLP_ENDPOINT,
// OTEL_EXPORTER_OTLP_PROTOCOL, OTEL_RESOURCE_ATTRIBUTES,
// OTEL_METRIC_EXPORT_INTERVAL. Sem OTEL_EXPORTER_OTLP_ENDPOINT (testes,
// docker-compose local) nada e ligado e os providers globais seguem no-op.
//
// Este arquivo e identico nos servicos Go.

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// setupTelemetry liga traces e metricas e devolve a funcao que esvazia os
// buffers no desligamento.
func setupTelemetry(ctx context.Context) (func(context.Context) error, error) {
	// O propagador W3C vale mesmo sem exporter: o servico continua repassando
	// o traceparent que recebe para quem ele chama.
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{}, propagation.Baggage{},
	))

	if os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") == "" {
		log.Println("OTEL_EXPORTER_OTLP_ENDPOINT ausente: telemetria desligada")
		return func(context.Context) error { return nil }, nil
	}

	// WithFromEnv: OTEL_SERVICE_NAME + OTEL_RESOURCE_ATTRIBUTES. WithHost: o
	// nome do pod vira host.name (1 processo por pod = 1 instancia).
	res, err := resource.New(ctx,
		resource.WithFromEnv(),
		resource.WithTelemetrySDK(),
		resource.WithHost(),
	)
	if err != nil {
		return nil, err
	}

	traceExporter, err := otlptracehttp.New(ctx)
	if err != nil {
		return nil, err
	}
	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExporter),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tracerProvider)

	metricExporter, err := otlpmetrichttp.New(ctx)
	if err != nil {
		return nil, err
	}
	// Intervalo via OTEL_METRIC_EXPORT_INTERVAL (ms).
	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter)),
		sdkmetric.WithResource(res),
	)
	otel.SetMeterProvider(meterProvider)

	log.Println("telemetria OTel ligada")
	return func(ctx context.Context) error {
		return errors.Join(tracerProvider.Shutdown(ctx), meterProvider.Shutdown(ctx))
	}, nil
}

// instrumentHandler envolve o mux: cada requisicao vira um span SERVER (que
// continua o trace de quem chamou) e entra na metrica
// http.server.request.duration.
//
// /health fica de fora: as probes do Kubernetes rodam a cada poucos segundos
// e sempre respondem 200 -- diluiriam a taxa de erro do alerta de 5xx e
// encheriam o APM de traces sem valor.
//
// O middleware de chaos injection é aplicado ANTES do otelhttp.NewHandler,
// para que os erros injetados sejam contabilizados nas métricas e nos traces.
// /health é excluído do chaos: continua respondendo 200 mesmo quando o chaos está ligado,
// reproduzindo a "falha silenciosa" do enunciado.
func instrumentHandler(h http.Handler) http.Handler {
	// Cria o handler de chaos (será aplicado dentro do otelhttp)
	chaosHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// /health e /admin/chaos nunca falham
		if r.URL.Path == "/health" || r.URL.Path == "/admin/chaos" {
			h.ServeHTTP(w, r)
			return
		}

		// Checa se o chaos está ligado e se deve falhar
		if getAppFromContext(r.Context()).Chaos.ShouldFail() {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"erro":"injeção de falha do chaos","mensagem":"este erro foi propositalmente injetado para demonstrar o sistema de alerta e self-healing"}`))
			log.Printf("Chaos: erro injetado em %s %s", r.Method, r.URL.Path)
			return
		}

		h.ServeHTTP(w, r)
	})

	return otelhttp.NewHandler(chaosHandler, "http.server",
		otelhttp.WithFilter(func(r *http.Request) bool { return r.URL.Path != "/health" }),
		otelhttp.WithSpanNameFormatter(func(_ string, r *http.Request) string {
			// Os servicos so tem rotas fixas (sem IDs no path): o path cabe
			// no nome do span sem explodir a cardinalidade.
			return r.Method + " " + r.URL.Path
		}),
	)
}

// getAppFromContext recupera o App do contexto da requisição.
// Isso é uma solução temporary para acessar o ChaosInjector sem passar como parâmetro.
// Em produção, seria melhor usar um tipo de contexto específico.
func getAppFromContext(ctx context.Context) *App {
	// Isso é um hack: precisamos acessar o ChaosInjector do App.
	// Como o instrumentation happen antes de passar para os handlers da mux,
	// usamos uma variável global (não é ótimo, mas é funcional).
	return globalApp
}

// globalApp é usada pela instrumentação para acessar o ChaosInjector.
// Definida no main() após criar o App.
var globalApp *App
