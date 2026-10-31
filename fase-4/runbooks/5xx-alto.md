# Runbook: Taxa de Erros 5xx Alta

## Descrição do Alerta

Este alerta dispara quando a taxa de erros HTTP 5xx (Server Error) ultrapassa **5% do total de requisições** em um microsserviço do ToggleMaster, sustentado por mais de **2 minutos**.

O alerta é baseado na métrica `http_server_request_duration_seconds_count` do OpenTelemetry, que registra todas as requisições HTTP (exceto `/health`, que fica fora da instrumentação).

## Por Que Este Alerta Importa

- **Sintoma de falha silenciosa**: erros 5xx são falhas no lado do servidor, não do cliente. Uma taxa alta indica que o serviço está tentando processar requisições, mas falhando.
- **Impacto direto no usuário**: requisições retornam erro; a experiência quebra.
- **Pode estar invisível em métricas de CPU/memória**: o serviço pode parecer "saudável" em recursos, mas estar quebrando em lógica de negócio.

## Passos de Investigação

### 1. **Confirme o alerta no Grafana**
   - Abra o dashboard `togglemaster-saude` em `http://localhost:3000/d/togglemaster-saude` (com `kubectl -n observability port-forward svc/kube-prometheus-stack-grafana 3000:80`).
   - Procure o painel de **"Taxa de erros 5xx por serviço"**.
   - Confirme qual serviço está acima de 5% e em qual horário começou.

### 2. **Verifique os logs do serviço**
   ```bash
   # Acesse os logs centralizados no Loki (via Grafana)
   # Ou via kubectl:
   kubectl -n toggle logs -l app=<svc> --tail=100 --since=5m
   ```
   - Procure por stack traces de exceção, warnings ou erros.
   - Identifique a linha/função que está quebrando.

### 3. **Verifique as dependências do serviço**
   - **Banco de dados**: está up? Há lock ou slow query?
     ```bash
     kubectl -n toggle get pods -l app=<svc>
     kubectl -n toggle describe pod <pod-name>
     ```
   - **Cache (Redis)**: se aplicável (evaluation-service), está respondendo?
   - **Fila (SQS)**: se aplicável, há deadletter ou mensagens com poison?
   - **Downstream**: se o serviço chama outro, o downstream está saudável?

### 4. **Verifique a métrica de tráfego (RPS)**
   - Abra o painel de **"Taxa de requisições por microsserviço"** no dashboard.
   - Se o RPS aumentou drasticamente, pode haver pico de tráfego legítimo.
   - Se o RPS é normal mas os 5xx aumentaram, é um problema de qualidade (não de carga).

## Ações Imediatas

### Opção 1: Self-Healing Automático (padrão)
O Grafana já chamou o workflow `self-healing.yml` (aba Actions do GitHub, branch `lab`), que executa:
```bash
kubectl rollout restart deployment/<svc>-deployment -n toggle
kubectl rollout status deployment/<svc>-deployment -n toggle
```

Isso mata os pods antigos e deixa os novos subirem. Se o problema é uma memória vazada ou estado corrompido, um restart frequentemente resolve (se for rápido).

**Aguarde 1–2 minutos e verifique se o alerta volta ao normal.**

### Opção 2: Rollback para versão anterior
Se o restart não resolver:
```bash
# Verifique qual versão está em execução
kubectl -n toggle get deployment <svc>-deployment -o jsonpath='{.spec.template.spec.containers[0].image}'

# O ArgoCD gerencia a imagem. Para rollback, reverta o commit que bumped a versão.
git revert <hash-do-commit-que-bumped>
git push origin lab
# ArgoCD vai sincronizar em até 3 minutos (padrão).
```

### Opção 3: Scale out temporária
Se for gargalo de CPU/memória. Atenção: o ArgoCD está com `selfHeal`, então ele
desfaz um `replicas` alterado à mão em poucos minutos; o ajuste permanente é
no git (overlay do serviço em `fase-3/gitops/manifests/<svc>/`).
```bash
kubectl -n toggle patch deployment <svc>-deployment -p \
  '{"spec":{"replicas":3}}'
```

## Monitoramento Pós-Ação

1. **Taxa de 5xx**: deve voltar para ~0% em até 5 minutos.
2. **CPU/memória**: se ficar alta após restart, há problema estrutural (code leak ou config ruim).
3. **Trace distribuído**: abra um trace de uma requisição bem-sucedida vs. uma que deu erro, compare latência e dependências.

## Investigação Profunda (se o problema voltar)

- **Leak de memória**: `kubectl -n toggle top pod` antes e depois de um restart; se sobe rápido demais, há leak.
- **Deadlock**: se o serviço está aguardando lock de BD infinitamente, os logs vão mostrar.
- **Versão quebrada**: compare `git log` e procure por commit recente de feature no serviço afetado.

## Escalação

Se após **30 minutos** o alerta não resolver com restart:
1. Abra uma issue no repositório linkando este runbook e o período do alerta.
2. Passe para a equipe de backend do serviço com os logs do Loki e trace distribuído.
3. Considere rollback total do serviço para a última versão estável.

---

**Dashboard**: `togglemaster-saude`
**Métrica**: `http_server_request_duration_seconds_count`
**Limite**: 5% de 5xx
**Condição**: por 2 minutos
**Self-Healing**: GitHub Action `self-healing.yml` (restart de deployment, cooldown de 10 min)
**Disparo manual**: `gh workflow run self-healing.yml --ref lab -f service=<svc>`
