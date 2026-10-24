"""Instrumentacao OpenTelemetry do servico (traces + metricas via OTLP).

Chamado pelo hook `post_fork` do gunicorn (gunicorn.conf.py), UMA vez dentro
de cada worker -- e nao pelo `opentelemetry-instrument` no processo-pai:

- as threads que exportam spans/metricas nao sobrevivem ao fork;
- cada worker precisa de um `service.instance.id` proprio. Com 2 workers
  exportando a MESMA serie, o Prometheus ve o contador "voltar" e as taxas
  (rate) saem erradas.

Toda a configuracao vem das variaveis padrao do OTel, definidas no cluster
(fase-3/components/otel): OTEL_SERVICE_NAME, OTEL_EXPORTER_OTLP_ENDPOINT,
OTEL_RESOURCE_ATTRIBUTES, ... Sem OTEL_EXPORTER_OTLP_ENDPOINT (testes,
docker-compose local) nada e ligado.

Este arquivo e identico nos servicos Python. Quem decide o que e
instrumentado e o requirements.txt de cada servico: liga-se o instrumentor
cujo PACOTE de instrumentacao esta instalado (e nao a biblioteca alvo -- o
`requests`, por exemplo, vem junto com o exporter OTLP em todo servico).
"""

import importlib.util
import logging
import os
import socket

log = logging.getLogger(__name__)

# (modulo do instrumentor, classe) -- ligado so se o pacote estiver instalado
_INSTRUMENTORS = [
    ("opentelemetry.instrumentation.flask", "FlaskInstrumentor"),
    ("opentelemetry.instrumentation.psycopg2", "Psycopg2Instrumentor"),
    ("opentelemetry.instrumentation.requests", "RequestsInstrumentor"),
    ("opentelemetry.instrumentation.botocore", "BotocoreInstrumentor"),
]


def setup_telemetry():
    if not os.getenv("OTEL_EXPORTER_OTLP_ENDPOINT"):
        log.info("OTEL_EXPORTER_OTLP_ENDPOINT ausente: telemetria desligada")
        return

    from opentelemetry import metrics, trace
    from opentelemetry.exporter.otlp.proto.http.metric_exporter import OTLPMetricExporter
    from opentelemetry.exporter.otlp.proto.http.trace_exporter import OTLPSpanExporter
    from opentelemetry.instrumentation.logging import LoggingInstrumentor
    from opentelemetry.sdk.metrics import MeterProvider
    from opentelemetry.sdk.metrics.export import PeriodicExportingMetricReader
    from opentelemetry.sdk.resources import SERVICE_INSTANCE_ID, Resource
    from opentelemetry.sdk.trace import TracerProvider
    from opentelemetry.sdk.trace.export import BatchSpanProcessor

    # Resource.create junta OTEL_SERVICE_NAME + OTEL_RESOURCE_ATTRIBUTES.
    resource = Resource.create({SERVICE_INSTANCE_ID: f"{socket.gethostname()}-{os.getpid()}"})

    tracer_provider = TracerProvider(resource=resource)
    tracer_provider.add_span_processor(BatchSpanProcessor(OTLPSpanExporter()))
    trace.set_tracer_provider(tracer_provider)

    # Intervalo via OTEL_METRIC_EXPORT_INTERVAL (ms).
    reader = PeriodicExportingMetricReader(OTLPMetricExporter())
    metrics.set_meter_provider(MeterProvider(resource=resource, metric_readers=[reader]))

    for instrumentor_module, class_name in _INSTRUMENTORS:
        if importlib.util.find_spec(instrumentor_module) is None:
            continue
        instrumentor = getattr(importlib.import_module(instrumentor_module), class_name)
        instrumentor().instrument()

    # Acrescenta trace_id/span_id em toda linha de log: o log no Loki aponta
    # para o trace no APM. Troca o formato do logging raiz (o basicConfig do
    # app.py, que roda depois, vira no-op).
    LoggingInstrumentor().instrument(set_logging_format=True, log_level=logging.INFO)

    log.info("telemetria OTel ligada (pid %s)", os.getpid())
