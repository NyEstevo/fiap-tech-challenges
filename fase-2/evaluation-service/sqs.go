package main

import (
	"context"
	"encoding/json"
	"log"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/sqs"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// Evento que será enviado para a fila
type EvaluationEvent struct {
	UserID    string    `json:"user_id"`
	FlagName  string    `json:"flag_name"`
	Result    bool      `json:"result"`
	Timestamp time.Time `json:"timestamp"`
}

// traceAttributes grava o contexto W3C do trace (traceparent/tracestate) como
// MessageAttributes. SQS nao e HTTP: sem isto o analytics-service, do outro
// lado da fila, abriria um trace novo (ver trace_context() no app.py dele).
func traceAttributes(ctx context.Context) map[string]*sqs.MessageAttributeValue {
	carrier := propagation.MapCarrier{}
	otel.GetTextMapPropagator().Inject(ctx, carrier)
	attrs := make(map[string]*sqs.MessageAttributeValue, len(carrier))
	for k, v := range carrier {
		attrs[k] = &sqs.MessageAttributeValue{DataType: aws.String("String"), StringValue: aws.String(v)}
	}
	return attrs
}

// sendEvaluationEvent envia um evento para a fila SQS, dentro de um span
// PRODUCER filho da requisicao que gerou o evento.
func (a *App) sendEvaluationEvent(ctx context.Context, userID, flagName string, result bool) {
	// Se a URL da fila não foi configurada, apenas loga localmente e sai.
	if a.SqsSvc == nil || a.SqsQueueURL == "" {
		log.Printf("[SQS_DISABLED] Evento: User '%s', Flag '%s', Result '%t'", userID, flagName, result)
		return
	}

	event := EvaluationEvent{
		UserID:    userID,
		FlagName:  flagName,
		Result:    result,
		Timestamp: time.Now().UTC(),
	}

	body, err := json.Marshal(event)
	if err != nil {
		log.Printf("Erro ao serializar evento SQS: %v", err)
		return
	}

	queueName := a.SqsQueueURL[strings.LastIndex(a.SqsQueueURL, "/")+1:]
	ctx, span := otel.Tracer("evaluation-service").Start(ctx, "send "+queueName,
		trace.WithSpanKind(trace.SpanKindProducer),
		trace.WithAttributes(
			attribute.String("messaging.system", "aws_sqs"),
			attribute.String("messaging.operation.type", "send"),
			attribute.String("messaging.destination.name", queueName),
		),
	)
	defer span.End()

	// Envia a mensagem (o traceparent vai junto, como atributo)
	out, err := a.SqsSvc.SendMessageWithContext(ctx, &sqs.SendMessageInput{
		MessageBody:       aws.String(string(body)),
		QueueUrl:          aws.String(a.SqsQueueURL),
		MessageAttributes: traceAttributes(ctx),
	})

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "falha ao enviar para o SQS")
		log.Printf("Erro ao enviar mensagem para SQS: %v", err)
	} else {
		span.SetAttributes(attribute.String("messaging.message.id", aws.StringValue(out.MessageId)))
		log.Printf("Evento de avaliação enviado para SQS (Flag: %s)", flagName)
	}
}
