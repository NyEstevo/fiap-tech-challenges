#!/usr/bin/env bash
# Teste manual ponta a ponta dos 5 microsservicos via curl.
# Uso: ./smoke-test.sh [local|lab|prod]
#
# local -> http://localhost (Kind/Minikube com ingress-nginx local)
# lab   -> le o endereco do ingress no cluster EKS da AWS Academy (namespace toggle)
# prod  -> le o endereco do ingress no cluster EKS da conta pessoal (namespace toggle)
#
# A MASTER_KEY do auth-service e lida direto do secret no cluster (ESO), nunca do git.
#
# NOTA sobre flags-service: o Ingress remove o prefixo /flags e o app atende
# na raiz. O contorno antigo /flags/flags (de quando o app duplicava o prefixo)
# respondia 405 no POST e caia em GET /<name> ("flags" virava nome de flag).

set -euo pipefail

FLAGS_PATH="/flags"

ENV="${1:-local}"
NAMESPACE="toggle"

case "$ENV" in
  local)
    BASE_URL="http://localhost"
    ;;
  lab|prod)
    HOST="$(kubectl get ingress ingress -n "$NAMESPACE" \
      -o jsonpath='{.status.loadBalancer.ingress[0].hostname}{.status.loadBalancer.ingress[0].ip}')"
    if [ -z "$HOST" ]; then
      echo "Nao consegui ler o endereco do ingress no namespace $NAMESPACE (kubectl configurado pro cluster certo?)" >&2
      exit 1
    fi
    BASE_URL="http://$HOST"
    ;;
  *)
    echo "Uso: $0 [local|lab|prod]" >&2
    exit 1
    ;;
esac

echo "== BASE_URL: $BASE_URL =="

echo
echo "== 1. Health checks =="
for svc in auth flags targeting evaluation analytics; do
  printf '%-12s ' "$svc:"
  curl -s -o /dev/null -w '%{http_code}\n' "$BASE_URL/$svc/health"
done

echo
echo "== 2. auth-service: gerar API key =="
if [ "$ENV" = "local" ]; then
  MASTER_KEY="${MASTER_KEY:-}"
  if [ -z "$MASTER_KEY" ]; then
    echo "Defina MASTER_KEY como variavel de ambiente antes de rodar em local." >&2
    exit 1
  fi
else
  MASTER_KEY="$(kubectl get secret auth-secret -n "$NAMESPACE" \
    -o jsonpath='{.data.MASTER_KEY}' | base64 -d)"
fi

KEY_RESPONSE="$(curl -s -X POST "$BASE_URL/auth/admin/keys" \
  -H "Authorization: Bearer $MASTER_KEY" \
  -H "Content-Type: application/json" \
  -d '{"name": "smoke-test"}')"
echo "$KEY_RESPONSE"

API_KEY="$(echo "$KEY_RESPONSE" | grep -o '"key"[[:space:]]*:[[:space:]]*"[^"]*"' | sed -E 's/.*"([^"]+)"$/\1/')"
if [ -z "$API_KEY" ]; then
  echo "Nao consegui extrair a API key da resposta acima." >&2
  exit 1
fi
echo "API_KEY obtida: $API_KEY"

echo
echo "== 3. auth-service: validar a key =="
curl -s "$BASE_URL/auth/validate" \
  -H "Authorization: Bearer $API_KEY"
echo

echo
echo "== 4. flags-service: criar flag =="
curl -s -X POST "$BASE_URL$FLAGS_PATH" \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"name": "feature_x", "description": "smoke test", "is_enabled": true}'
echo

echo
echo "== 5. flags-service: listar flags =="
curl -s "$BASE_URL$FLAGS_PATH" \
  -H "Authorization: Bearer $API_KEY"
echo

echo
echo "== 6. targeting-service: criar regra =="
curl -s -X POST "$BASE_URL/targeting/rules" \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"flag_name": "feature_x", "rules": {"type": "PERCENTAGE", "value": 50}, "is_enabled": true}'
echo

echo
echo "== 7. evaluation-service: avaliar a flag para um usuario =="
curl -s "$BASE_URL/evaluation/evaluate?user_id=user123&flag_name=feature_x"
echo

echo
echo "== 8. limpeza: apagar regra e flag de teste =="
curl -s -X DELETE "$BASE_URL/targeting/rules/feature_x" \
  -H "Authorization: Bearer $API_KEY"
curl -s -X DELETE "$BASE_URL$FLAGS_PATH/feature_x" \
  -H "Authorization: Bearer $API_KEY"

echo
echo "== Fim =="
