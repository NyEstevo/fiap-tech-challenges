#!/bin/bash
# prova-real.sh: liga/desliga/consulta o chaos injection no auth-service
# para demonstrar o ciclo de alerta, notificação e self-healing.
#
# Uso: ./prova-real.sh <acao> [taxa] [endpoints...]
# Ações: on|off|status
# Taxa: 0-100 (default 100). Exemplo: prova-real.sh on 50 http://localhost:8080
# Endpoints: URLs para testar. Se vazio, usa port-forward automático.
#
# Exemplos:
#   ./prova-real.sh status
#   ./prova-real.sh on 100
#   ./prova-real.sh on 50 http://lab.example.com
#   ./prova-real.sh off

set -euo pipefail

# Cores para output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

ACAO="${1:-status}"
TAXA="${2:-100}"
ENDPOINT="${3:-}"

# Helper para imprimir mensagens
log_info() {
	echo -e "${BLUE}[INFO]${NC} $*"
}

log_success() {
	echo -e "${GREEN}[OK]${NC} $*"
}

log_error() {
	echo -e "${RED}[ERRO]${NC} $*"
}

log_warn() {
	echo -e "${YELLOW}[AVISO]${NC} $*"
}

# Se não forneceu endpoint, tenta descobrir via kubectl
if [ -z "$ENDPOINT" ]; then
	log_info "Buscando endpoint do auth-service via kubectl..."

	# Tenta obter o IP do ingress
	INGRESS_IP=$(kubectl get ingress ingress -n toggle -o jsonpath='{.status.loadBalancer.ingress[0].ip}' 2>/dev/null || echo "")

	if [ -n "$INGRESS_IP" ]; then
		ENDPOINT="http://$INGRESS_IP"
		log_success "Usando ingress: $ENDPOINT"
	else
		# Fallback: port-forward
		log_warn "Ingress não disponível, usando port-forward..."
		kubectl port-forward -n toggle svc/auth-service 8001:8001 &
		PF_PID=$!
		trap "kill $PF_PID 2>/dev/null || true" EXIT
		sleep 2
		ENDPOINT="http://localhost:8001"
		log_success "Port-forward estabelecido (PID: $PF_PID)"
	fi
fi

log_info "Endpoint: $ENDPOINT"

# Obtém a MASTER_KEY do Secret auth-secret
log_info "Obtendo MASTER_KEY do Secret auth-secret..."
MASTER_KEY=$(kubectl get secret auth-secret -n toggle -o jsonpath='{.data.MASTER_KEY}' 2>/dev/null | base64 -d)

if [ -z "$MASTER_KEY" ]; then
	log_error "Não foi possível obter MASTER_KEY do Secret"
	exit 1
fi

log_success "MASTER_KEY obtida"

# Função para fazer requisição ao /admin/chaos
call_chaos_api() {
	local method="$1"
	local body="$2"

	if [ -z "$body" ]; then
		curl -s -X "$method" \
			-H "Authorization: Bearer $MASTER_KEY" \
			-H "Content-Type: application/json" \
			"${ENDPOINT}/admin/chaos"
	else
		curl -s -X "$method" \
			-H "Authorization: Bearer $MASTER_KEY" \
			-H "Content-Type: application/json" \
			-d "$body" \
			"${ENDPOINT}/admin/chaos"
	fi
}

case "$ACAO" in
status)
	log_info "Consultando status do chaos..."
	response=$(call_chaos_api "GET" "")
	echo "$response" | jq . 2>/dev/null || echo "$response"
	;;

on)
	if [ "$TAXA" -lt 0 ] || [ "$TAXA" -gt 100 ]; then
		log_error "Taxa deve estar entre 0 e 100, recebido: $TAXA"
		exit 1
	fi

	log_info "Ligando chaos com taxa de erro: ${TAXA}%"
	body="{\"error_rate_percent\": $TAXA}"
	response=$(call_chaos_api "POST" "$body")
	echo "$response" | jq . 2>/dev/null || echo "$response"
	log_success "Chaos ligado!"

	# Mostra dica de próximos passos
	log_info "Próximas ações:"
	echo "  1. Ver tráfego (novo terminal): ./scripts/gerar-trafego.sh $ENDPOINT"
	echo "  2. Ver pods: kubectl get pods -n toggle -w"
	echo "  3. Acompanhar dashboard do Grafana (métricas, logs, erros)"
	echo "  4. Desligar: ./scripts/prova-real.sh off $ENDPOINT"
	;;

off)
	log_info "Desligando chaos..."
	response=$(call_chaos_api "DELETE" "")
	echo "$response" | jq . 2>/dev/null || echo "$response"
	log_success "Chaos desligado!"
	;;

*)
	log_error "Ação desconhecida: $ACAO"
	echo "Ações disponíveis: on, off, status"
	exit 1
	;;
esac
