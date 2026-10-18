"""Smoke test do analytics-service.

app.py exige AWS_REGION / AWS_SQS_URL / AWS_DYNAMODB_TABLE (senao sys.exit(1)),
cria clientes boto3 e sobe o worker SQS numa thread ao ser importado -> definimos
o env, mockamos boto3.Session e neutralizamos threading.Thread antes do import.
A rota /health nao toca AWS.
"""

import importlib
import os
from unittest import mock

os.environ.setdefault("AWS_REGION", "us-east-1")
os.environ.setdefault("AWS_SQS_URL", "https://sqs.us-east-1.amazonaws.com/000/tc-sqs")
os.environ.setdefault("AWS_DYNAMODB_TABLE", "tc-dynamo")

with mock.patch("boto3.Session"), mock.patch("threading.Thread"):
    app_module = importlib.import_module("app")

client = app_module.app.test_client()


def test_health_ok():
    resp = client.get("/health")
    assert resp.status_code == 200
    assert resp.get_json() == {"status": "ok"}


def test_trace_context_continua_o_trace_do_produtor():
    from opentelemetry import trace

    traceparent = "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"
    message = {
        "MessageAttributes": {"traceparent": {"DataType": "String", "StringValue": traceparent}}
    }

    span_ctx = trace.get_current_span(app_module.trace_context(message)).get_span_context()

    assert format(span_ctx.trace_id, "032x") == "4bf92f3577b34da6a3ce929d0e0e4736"
    assert format(span_ctx.span_id, "016x") == "00f067aa0ba902b7"
    assert span_ctx.is_remote


def test_trace_context_sem_atributos_nao_quebra():
    from opentelemetry import trace

    ctx = app_module.trace_context({"MessageId": "m-1"})
    assert not trace.get_current_span(ctx).get_span_context().is_valid


def test_process_message_salva_e_apaga_a_mensagem():
    message = {
        "MessageId": "m-1",
        "ReceiptHandle": "rh-1",
        "Body": '{"user_id": "u1", "flag_name": "f1", "result": true, "timestamp": "t"}',
    }
    with (
        mock.patch.object(app_module, "dynamodb_client") as ddb,
        mock.patch.object(app_module, "sqs_client") as sqs,
    ):
        app_module.process_message(message)

    ddb.put_item.assert_called_once()
    sqs.delete_message.assert_called_once_with(
        QueueUrl=os.environ["AWS_SQS_URL"], ReceiptHandle="rh-1"
    )
