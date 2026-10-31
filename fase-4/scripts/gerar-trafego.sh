#!/usr/bin/env bash
# gerar-trafego.sh -- trafego continuo no auth-service para a "prova real".
#
# Chama GET /validate com uma chave invalida: normalmente responde 401 (erro
# do cliente, NAO conta no alerta de 5xx); com o chaos ligado responde 500.
# Tambem chama /health, que segue 200 o tempo todo -- a "falha silenciosa".
#
# Uso:
#   fase-4/scripts/gerar-trafego.sh              # via ingress do lab
#   fase-4/scripts/gerar-trafego.sh http://localhost:18001   # outro endpoint
# Intervalo entre chamadas: INTERVALO (segundos, padrao 0.5).
set -euo pipefail

INTERVALO="${INTERVALO:-0.5}"
BASE="${1:-}"
if [[ -z "$BASE" ]]; then
  host=$(kubectl -n toggle get ingress ingress -o jsonpath='{.status.loadBalancer.ingress[0].hostname}')
  [[ -n "$host" ]] || { echo "ingress sem hostname; passe o endpoint como argumento" >&2; exit 1; }
  BASE="http://${host}/auth"
fi
echo "trafego em ${BASE} (Ctrl+C para parar)"

# De proposito NAO e uma chave valida: o normal e o auth responder 401.
chave_falsa="nao-e-uma-chave"

total=0; erros5xx=0
while true; do
  code=$(curl -s -o /dev/null -w '%{http_code}' -H "Authorization: Bearer ${chave_falsa}" "${BASE}/validate" || echo 000)
  health=$(curl -s -o /dev/null -w '%{http_code}' "${BASE}/health" || echo 000)
  total=$((total + 1))
  [[ "$code" == 5* ]] && erros5xx=$((erros5xx + 1))
  printf '%s  /validate=%s  /health=%s  (5xx: %d de %d)\n' "$(date +%T)" "$code" "$health" "$erros5xx" "$total"
  sleep "$INTERVALO"
done
