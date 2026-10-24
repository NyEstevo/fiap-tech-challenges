#!/usr/bin/env bash
# prova-real.sh -- liga, desliga ou consulta a injecao de falha do auth-service.
#
#   fase-4/scripts/prova-real.sh on [taxa]   # liga (taxa 0-100, padrao 100)
#   fase-4/scripts/prova-real.sh off         # desliga
#   fase-4/scripts/prova-real.sh status
#
# Fala direto com o Service (port-forward em 127.0.0.1:18001, porta que nao
# disputa com o port-forward do ArgoCD/Grafana). A MASTER_KEY sai do Secret
# auth-secret (materializado pelo ESO) e nunca e impressa.
# O estado vive na memoria do pod: e o rollout restart do self-healing que
# "cura" -- ou este `off`.
set -euo pipefail

acao="${1:-status}"
taxa="${2:-100}"
porta=18001

master_key=$(kubectl -n toggle get secret auth-secret -o jsonpath='{.data.MASTER_KEY}' | base64 -d)
[[ -n "$master_key" ]] || { echo "MASTER_KEY nao encontrada em toggle/auth-secret" >&2; exit 1; }

kubectl -n toggle port-forward svc/auth-service "${porta}:8001" >/dev/null 2>&1 &
pf=$!
trap 'kill "$pf" 2>/dev/null || true' EXIT
for _ in $(seq 20); do curl -s -o /dev/null "http://127.0.0.1:${porta}/health" && break; sleep 0.5; done

chaos() {
  curl -sS -X "$1" -H "Authorization: Bearer ${master_key}" -H 'Content-Type: application/json' \
    ${2:+-d "$2"} "http://127.0.0.1:${porta}/admin/chaos"
  echo
}

case "$acao" in
  on)     chaos POST "{\"error_rate_percent\": ${taxa}}" ;;
  off)    chaos DELETE ;;
  status) chaos GET ;;
  *)      echo "uso: $0 on [taxa] | off | status" >&2; exit 2 ;;
esac
