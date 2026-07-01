#!/bin/bash
set -e

echo "Aplicando recursos compartilhados..."
kubectl apply -f k8s/

echo "Aguardando 5 segundos..."
sleep 5

echo "Aplicando microserviços..."
kubectl apply -f analytics-service/k8s/
kubectl apply -f auth-service/k8s/
kubectl apply -f evaluation-service/k8s/
kubectl apply -f flag-service/k8s/
kubectl apply -f targeting-service/k8s/

echo "Deploy concluído!"
kubectl get pods -A