#!/bin/bash
set -e

# =============================================
# CONFIGURAÇÕES — ajuste se necessário
# =============================================
CLUSTER_NAME="tech-challenge-fase-2"
REGION="us-east-1"

echo "========================================"
echo "  Destruindo recursos: $CLUSTER_NAME"
echo "========================================"

# 1. Deleta Load Balancers criados pelo Ingress
echo ""
echo "[1/5] Removendo Load Balancers..."
LB_ARNS=$(aws elbv2 describe-load-balancers \
  --region $REGION \
  --query "LoadBalancers[?contains(LoadBalancerName, '$CLUSTER_NAME') || contains(LoadBalancerName, 'ingress')].LoadBalancerArn" \
  --output text 2>/dev/null)

if [ -n "$LB_ARNS" ]; then
  for ARN in $LB_ARNS; do
    echo "  Deletando Load Balancer: $ARN"
    aws elbv2 delete-load-balancer --load-balancer-arn $ARN --region $REGION
  done
  echo "  Aguardando Load Balancers serem removidos..."
  sleep 30
else
  echo "  Nenhum Load Balancer encontrado."
fi

# 2. Deleta volumes EBS orphãos
echo ""
echo "[3/5] Removendo volumes EBS orphãos..."
VOLUMES=$(aws ec2 describe-volumes \
  --filters Name=status,Values=available \
  --region $REGION \
  --query 'Volumes[*].VolumeId' \
  --output text 2>/dev/null)

if [ -n "$VOLUMES" ]; then
  for VOL in $VOLUMES; do
    echo "  Deletando volume: $VOL"
    aws ec2 delete-volume --volume-id $VOL --region $REGION
  done
else
  echo "  Nenhum volume orphão encontrado."
fi

# 4. Deleta o cluster EKS
echo ""
echo "[4/5] Deletando cluster EKS (aguarde ~10-15 min)..."
eksctl delete cluster --name $CLUSTER_NAME --region $REGION

# 5. Verifica se sobrou alguma stack do CloudFormation
echo ""
echo "[5/5] Verificando stacks CloudFormation residuais..."
STACKS=$(aws cloudformation list-stacks \
  --stack-status-filter CREATE_FAILED ROLLBACK_COMPLETE DELETE_FAILED \
  --region $REGION \
  --query "StackSummaries[?contains(StackName, '$CLUSTER_NAME')].StackName" \
  --output text 2>/dev/null)

if [ -n "$STACKS" ]; then
  for STACK in $STACKS; do
    echo "  Removendo stack residual: $STACK"
    aws cloudformation update-termination-protection \
      --no-enable-termination-protection \
      --stack-name $STACK --region $REGION 2>/dev/null || true
    aws cloudformation delete-stack \
      --stack-name $STACK --region $REGION
  done
else
  echo "  Nenhuma stack residual encontrada."
fi

echo ""
echo "========================================"
echo "  Tudo destruído com sucesso!"
echo "========================================"
echo ""
echo "Verificação final — recursos rodando:"
aws ec2 describe-instances \
  --filters Name=instance-state-name,Values=running \
  --region $REGION \
  --query 'Reservations[*].Instances[*].[InstanceId,InstanceType]' \
  --output table 2>/dev/null || echo "Nenhuma instância rodando."