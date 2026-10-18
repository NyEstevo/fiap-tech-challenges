"""Config do gunicorn, lida automaticamente do diretorio de trabalho (/app).

O bind e os workers continuam no CMD do Dockerfile; aqui fica so o hook.
"""


def post_fork(server, worker):
    # Roda dentro de cada worker, ANTES de o app.py ser importado: os
    # instrumentors precisam modificar Flask/psycopg2/requests/botocore
    # antes de o app criar o Flask() e o pool de conexoes.
    from telemetry import setup_telemetry

    setup_telemetry()
