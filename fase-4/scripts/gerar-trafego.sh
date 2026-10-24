#!/bin/bash
# gerar-trafego.sh: gera tráfego contínuo e realista no sistema ToggleMaster
# para demonstrar o efeito do chaos injection e do alerta de 5xx.
#
# Uso: ./gerar-trafego.sh [endpoint] [intervalo_segundos]
# Exemplo: ./gerar-trafego.sh http://localhost:8080 1

set -euo pipefail

ENDPOINT="${1:-http://localhost:8080}"
INTERVAL="${2:-1}"

# Cores para output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}[INFO]${NC} Iniciando geração de tráfego para $ENDPOINT (intervalo: ${INTERVAL}s)"
echo -e "${YELLOW}[INFO]${NC} Pressione Ctrl+C para parar"

# Verifica se o ingress está acessível (com retry)
max_retries=5
retry_count=0
while ! curl -s "${ENDPOINT}/health" > /dev/null; do
	retry_count=$((retry_count + 1))
	if [ $retry_count -ge $max_retries ]; then
		echo -e "${RED}[ERRO]${NC} Não foi possível acessar ${ENDPOINT}/health após $max_retries tentativas"
		exit 1
	fi
	echo -e "${YELLOW}[INFO]${NC} Aguardando endpoint... (tentativa $retry_count/$max_retries)"
	sleep 2
done

echo -e "${GREEN}[OK]${NC} Endpoint acessível"

# Contador de requisições
count=0
success_count=0
error_count=0

while true; do
	count=$((count + 1))

	# Alterna entre /validate (com API key) e /health (sem autenticação)
	if [ $((count % 2)) -eq 1 ]; then
		# GET /health (sem autenticação)
		response=$(curl -s -w "\n%{http_code}" "${ENDPOINT}/health")
		http_code=$(echo "$response" | tail -1)
		method="GET /health"
	else
		# GET /validate com Bearer token (simulando requisição autenticada)
		response=$(curl -s -w "\n%{http_code}" -H "Authorization: Bearer dummy-key" "${ENDPOINT}/validate")
		http_code=$(echo "$response" | tail -1)
		method="GET /validate"
	fi

	# Conta sucessos (2xx/3xx) e erros (4xx/5xx)
	if [ "$http_code" -lt 400 ]; then
		success_count=$((success_count + 1))
		status_color="${GREEN}"
	else
		error_count=$((error_count + 1))
		status_color="${RED}"
	fi

	# Output formatado
	timestamp=$(date '+%H:%M:%S')
	echo -e "[${timestamp}] ${status_color}${http_code}${NC} $method (total: $count, ok: $success_count, erro: $error_count)"

	sleep "$INTERVAL"
done
