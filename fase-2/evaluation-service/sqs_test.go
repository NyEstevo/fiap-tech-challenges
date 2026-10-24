package main

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// O traceparent gravado nos MessageAttributes e o que o analytics-service le
// para continuar o trace do outro lado da fila (trace_context() no app.py).
func TestTraceAttributesLevaOTraceparent(t *testing.T) {
	otel.SetTextMapPropagator(propagation.TraceContext{})

	traceID, _ := trace.TraceIDFromHex("4bf92f3577b34da6a3ce929d0e0e4736")
	spanID, _ := trace.SpanIDFromHex("00f067aa0ba902b7")
	ctx := trace.ContextWithSpanContext(context.Background(), trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: traceID, SpanID: spanID, TraceFlags: trace.FlagsSampled,
	}))

	attrs := traceAttributes(ctx)

	tp, ok := attrs["traceparent"]
	if !ok {
		t.Fatalf("traceparent ausente nos MessageAttributes: %v", attrs)
	}
	if got, want := *tp.StringValue, "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"; got != want {
		t.Errorf("traceparent = %q, quer %q", got, want)
	}
	if *tp.DataType != "String" {
		t.Errorf("DataType = %q, quer String (exigido pelo SQS)", *tp.DataType)
	}
}

// Sem trace ativo (ex.: telemetria desligada) nao ha o que propagar.
func TestTraceAttributesSemTraceFicaVazio(t *testing.T) {
	otel.SetTextMapPropagator(propagation.TraceContext{})
	if attrs := traceAttributes(context.Background()); len(attrs) != 0 {
		t.Errorf("esperava nenhum atributo, veio %v", attrs)
	}
}
