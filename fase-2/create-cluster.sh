#!/bin/bash
set -e

# =============================================
# CONFIGURAÇÕES — ajuste se necessário
# =============================================
CLUSTER_NAME="tech-challenge-fase-2"
REGION="us-east-1"
NAMESPACE="tech-challenge-fase-2"
CLUSTER_ROLE_ARN="arn:aws:iam::506394430088:role/c213722a5401358l15440605t1w506394-LabEksClusterRole-5y6O0hQrRacM"
NODE_ROLE_ARN="arn:aws:iam::506394430088:role/c213722a5401358l15440605t1w506394430-LabEksNodeRole-n9hlx7PUzdU4"
REDIS="evaluation-service-2-bszzdn.serverless.use1.cache.amazonaws.com:6379"

echo "========================================"
echo "  Criando cluster EKS: $CLUSTER_NAME"
echo "========================================"

# 1. Cria o cluster via eksctl
echo ""
echo "[1/6] Criando cluster EKS (aguarde ~15-20 min)..."
cat > /tmp/cluster.yaml << YAML
apiVersion: eksctl.io/v1alpha5
kind: ClusterConfig

metadata:
  name: $CLUSTER_NAME
  region: $REGION

iam:
  serviceRoleARN: $CLUSTER_ROLE_ARN

managedNodeGroups:
  - name: workers
    instanceType: t3.medium
    desiredCapacity: 2
    minSize: 1
    maxSize: 4
    iam:
      instanceRoleARN: $NODE_ROLE_ARN
YAML

eksctl create cluster -f /tmp/cluster.yaml

# 2. Configura o kubectl
echo ""
echo "[2/6] Configurando kubectl..."
aws eks update-kubeconfig --name $CLUSTER_NAME --region $REGION

# 3. Verifica os nodes
echo ""
echo "[3/6] Verificando nodes..."
kubectl get nodes

# 4. Instala o NGINX Ingress Controller
echo ""
echo "[4/6] Instalando NGINX Ingress Controller..."
kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/controller-v1.10.1/deploy/static/provider/aws/deploy.yaml

echo "Aguardando NGINX Ingress Controller ficar pronto..."
kubectl wait --namespace ingress-nginx \
  --for=condition=ready pod \
  --selector=app.kubernetes.io/component=controller \
  --timeout=120s

# 5. Cria os secrets de todos os microserviços
echo ""
echo "[5/6] Criando secrets de todos os microserviços..."

# Pega as credenciais da sessão atual (renovadas a cada execução)
AWS_ACCESS_KEY=$(aws configure get aws_access_key_id)
AWS_SECRET_KEY=$(aws configure get aws_secret_access_key)
AWS_SESSION=$(aws configure get aws_session_token)

if [ -z "$AWS_ACCESS_KEY" ] || [ -z "$AWS_SECRET_KEY" ]; then
  echo "ERRO: Credenciais AWS não encontradas. Rode 'aws configure' primeiro."
  exit 1
fi

echo "  Credenciais AWS obtidas com sucesso!"

# Garante que o namespace existe antes de criar os secrets
kubectl get namespace $NAMESPACE 2>/dev/null || kubectl create namespace $NAMESPACE

# Deleta secrets antigos e recria com valores frescos
echo "  Recriando secrets..."

# analytics-service-secrets (só credenciais AWS)
kubectl delete secret analytics-service-secrets -n $NAMESPACE 2>/dev/null || true
kubectl create secret generic analytics-service-secrets \
  --namespace $NAMESPACE \
  --from-literal=AWS_ACCESS_KEY_ID="$AWS_ACCESS_KEY" \
  --from-literal=AWS_SECRET_ACCESS_KEY="$AWS_SECRET_KEY" \
  --from-literal=AWS_SESSION_TOKEN="$AWS_SESSION"
echo "  [✓] analytics-service-secrets"

# auth-service-secrets (valores fixos)
kubectl delete secret auth-service-secrets -n $NAMESPACE 2>/dev/null || true
kubectl create secret generic auth-service-secrets \
  --namespace $NAMESPACE \
  --from-literal=DATABASE_URL="postgres://postgres:password-auth-service@auth-service-db.crxebqesqajc.us-east-1.rds.amazonaws.com:5432/postgres" \
  --from-literal=MASTER_KEY="cadaumnasua"
echo "  [✓] auth-service-secrets"

# evaluation-service-secrets (credenciais AWS + service api key)
kubectl delete secret evaluation-service-secrets -n $NAMESPACE 2>/dev/null || true
kubectl create secret generic evaluation-service-secrets \
  --namespace $NAMESPACE \
  --from-literal=AWS_ACCESS_KEY_ID="$AWS_ACCESS_KEY" \
  --from-literal=AWS_SECRET_ACCESS_KEY="$AWS_SECRET_KEY" \
  --from-literal=AWS_SESSION_TOKEN="$AWS_SESSION" \
  --from-literal=SERVICE_API_KEY="tm_key_5aa29b27b5b066053bcc1d9610e2e585cd08b2f95efbbd29ee0aecd1a276e40c"
echo "  [✓] evaluation-service-secrets"

# flags-service-secret (valor fixo)
kubectl delete secret flags-service-secret -n $NAMESPACE 2>/dev/null || true
kubectl create secret generic flags-service-secret \
  --namespace $NAMESPACE \
  --from-literal=DATABASE_URL="postgresql://postgres:password-flags-service@flags-service-db.crxebqesqajc.us-east-1.rds.amazonaws.com:5432/postgres"
echo "  [✓] flags-service-secret"

# targeting-service-secret (valor fixo)
kubectl delete secret targeting-service-secret -n $NAMESPACE 2>/dev/null || true
kubectl create secret generic targeting-service-secret \
  --namespace $NAMESPACE \
  --from-literal=DATABASE_URL="postgresql://postgres:password-targeting-service@targeting-service-db.crxebqesqajc.us-east-1.rds.amazonaws.com:5432/postgres"
echo "  [✓] targeting-service-secret"

echo "  Todos os secrets criados com sucesso!"

# 7. Aplica os manifestos
echo ""
echo "[6/6] Aplicando manifestos Kubernetes..."
kubectl apply -f k8s/
sleep 5
kubectl apply -f analytics-service/k8s/
kubectl apply -f auth-service/k8s/
kubectl apply -f evaluation-service/k8s/
kubectl apply -f flag-service/k8s/
kubectl apply -f targeting-service/k8s/

# Resultado final
echo ""
echo "========================================"
echo "  Deploy concluído!"
echo "========================================"
echo ""
echo "Pods:"
kubectl get pods -n $NAMESPACE
echo ""
echo "Ingress URL:"
kubectl get svc -n ingress-nginx ingress-nginx-controller \
  --query='' 2>/dev/null || \
kubectl get svc -n ingress-nginx ingress-nginx-controller

kubectl apply -f evaluation-service/k8s/configmap.yaml -n $NAMESPACE
kubectl rollout restart deployment evaluation-service-deployment -n $NAMESPACE