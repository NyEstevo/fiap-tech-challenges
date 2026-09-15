<<<<<<< HEAD:fase-2/flag-service/test_app.py
"""Smoke test do flag-service.
=======
"""Smoke test do flags-service.
>>>>>>> lab:fase-2/flags-service/test_app.py

app.py conecta no Postgres e chama sys.exit(1) se faltar env -> definimos as
variaveis e trocamos o pool de conexao por um mock antes de importar o modulo.
A rota /health nao passa pelo @require_auth nem toca o banco.
"""

import importlib
import os
from unittest import mock

os.environ.setdefault("DATABASE_URL", "postgresql://user:pass@localhost:5432/flags_db")
os.environ.setdefault("AUTH_SERVICE_URL", "http://auth.local")

with mock.patch("psycopg2.pool.SimpleConnectionPool"):
    app_module = importlib.import_module("app")

client = app_module.app.test_client()


def test_health_ok():
    resp = client.get("/health")
    assert resp.status_code == 200
    assert resp.get_json() == {"status": "ok"}


def test_flags_sem_auth_header_retorna_401():
<<<<<<< HEAD:fase-2/flag-service/test_app.py
    resp = client.get("/flags")
=======
    resp = client.get("/")
>>>>>>> lab:fase-2/flags-service/test_app.py
    assert resp.status_code == 401
