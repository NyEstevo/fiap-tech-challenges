#!/bin/bash
set -euo pipefail

NAMESPACE="tech-challenge-fase-2"
ECR_REGISTRY="506394430088.dkr.ecr.us-east-1.amazonaws.com"
REGION="us-east-1"

echo "[ecr-refresh] Verificando sessão AWS..."
if ! aws sts get-caller-identity --region "$REGION" > /dev/null 2>&1; then
  echo "[ecr-refresh] ERRO: sessão AWS inválida ou expirada. Renove suas credenciais e tente novamente."
  exit 1
fi

echo "[ecr-refresh] Obtendo token ECR..."
TOKEN=$(aws ecr get-login-password --region "$REGION")

echo "[ecr-refresh] Atualizando ecr-secret no namespace $NAMESPACE..."
kubectl delete secret ecr-secret -n "$NAMESPACE" --ignore-not-found
kubectl create secret docker-registry ecr-secret \
  --docker-server="$ECR_REGISTRY" \
  --docker-username=AWS \
  --docker-password="$TOKEN" \
  --namespace="$NAMESPACE"

echo "[ecr-refresh] Pronto! ecr-secret atualizado com sucesso."
