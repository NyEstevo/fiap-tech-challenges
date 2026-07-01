import logging
import os
import psycopg2

log = logging.getLogger(__name__)

MIGRATIONS_DIR = os.path.join(os.path.dirname(__file__), "migrations")

_CREATE_MIGRATIONS_TABLE = """
CREATE TABLE IF NOT EXISTS _schema_migrations (
    id VARCHAR(255) PRIMARY KEY,
    applied_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);
"""

def run_migrations(database_url: str) -> None:
    conn = psycopg2.connect(database_url)
    try:
        with conn.cursor() as cur:
            cur.execute(_CREATE_MIGRATIONS_TABLE)
            conn.commit()

            cur.execute("SELECT id FROM _schema_migrations ORDER BY id")
            applied = {row[0] for row in cur.fetchall()}

            files = sorted([
                f for f in os.listdir(MIGRATIONS_DIR)
                if f.endswith(".sql") and not f.endswith(".rollback.sql")
            ])

            pending = [f for f in files if f not in applied]

            if not pending:
                log.info("Migrations: nenhuma pendente.")
                return

            for filename in pending:
                path = os.path.join(MIGRATIONS_DIR, filename)
                with open(path) as f:
                    sql = f.read()
                log.info(f"Aplicando migration: {filename}")
                cur.execute(sql)
                cur.execute("INSERT INTO _schema_migrations (id) VALUES (%s)", (filename,))
                conn.commit()
                log.info(f"Migration aplicada: {filename}")
    except Exception:
        conn.rollback()
        raise
    finally:
        conn.close()
