#!/bin/bash
set -euo pipefail

NAMESPACE="tech-challenge-fase-2"

echo "=== Removendo namespace $NAMESPACE e todos os recursos ==="
kubectl delete namespace "$NAMESPACE" --ignore-not-found

echo "Aguardando remoção completa..."
kubectl wait --for=delete namespace/"$NAMESPACE" --timeout=60s 2>/dev/null || true

echo "=== Teardown concluído! ==="
