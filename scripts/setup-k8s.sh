#!/bin/bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
NAMESPACE="tech-challenge-fase-2"

echo "=== [1/5] Verificando cluster kind ==="
if ! kubectl cluster-info > /dev/null 2>&1; then
  echo "Nenhum cluster ativo. Criando cluster kind..."
  kind create cluster --name kind
fi

echo "=== [2/5] Aplicando namespace ==="
kubectl apply -f "$ROOT_DIR/fase-2/k8s/namespace.yaml"

echo "=== [3/5] Aplicando recursos compartilhados ==="
kubectl apply -f "$ROOT_DIR/fase-2/k8s/"

echo "=== [4/5] Criando ecr-secret ==="
"$ROOT_DIR/scripts/refresh-ecr-secret.sh"

echo "=== [5/5] Aplicando manifestos dos serviços ==="
kubectl apply -f "$ROOT_DIR/fase-2/auth-service/k8s/"
kubectl apply -f "$ROOT_DIR/fase-2/analytics-service/k8s/"
kubectl apply -f "$ROOT_DIR/fase-2/evaluation-service/k8s/"

echo ""
echo "=== Setup concluído! ==="
echo "Acompanhe os pods com:"
echo "  kubectl get pods -n $NAMESPACE -w"
