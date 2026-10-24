# Roteiro: Prova Real do Alerta e Self-Healing

Este roteiro documenta o passo a passo para demonstrar o ciclo completo de **detecção de falha, alerta, notificação e self-healing automático** no ToggleMaster.

## Contexto

O `auth-service` foi instrumentado com injeção de falha configurável (chaos injection). Quando ativado, o serviço passa a responder com **HTTP 500** em todas as rotas (exceto `/health` e `/admin/chaos`), simulando uma falha silenciosa:

- `/health` continua retornando **200 OK** (liveness probe não mata o pod)
- `/validate` e `/admin/keys` retornam **500** com mensagem explicativa
- A métrica `http.server.request.duration_seconds_count` com label `http_response_status_code=500` sobe
- O alerta no Grafana dispara quando **5xx > 5%** por 5 minutos

## Pré-requisitos

1. **Cluster lab rodando** com ArgoCD sincronizando a stack de observabilidade (Prometheus, Loki, Grafana)
2. **OTel Collector** recebendo métricas do auth-service
3. **AlertManager** ou **Grafana Alerting** configurado
4. **PagerDuty/OpsGenie** integrado
5. **Slack/Discord** conectado para notificações
6. **GitHub Actions** configurado para self-healing
7. Scripts em `fase-4/scripts/`:
   - `gerar-trafego.sh` — gera requisições contínuas
   - `prova-real.sh` — controla o chaos

## Passo a Passo

### 1. Preparação: Abrir 3 terminais

```bash
# Terminal 1: Dashboard Grafana
# Acesse http://<grafana-ip>:3000 (credenciais: admin/admin)
# Abra o dashboard "Saúde do Ecossistema ToggleMaster"
# Foque no painel "Taxa de erros HTTP 5xx por microsserviço"

# Terminal 2: Monitoramento de pods
kubectl get pods -n toggle -w

# Terminal 3: Comandos de tráfego e chaos
cd fase-4/scripts
```

### 2. Gerar tráfego contínuo

**No Terminal 3:**

```bash
./gerar-trafego.sh http://<ingress-ip>
```

Aguarde 30-60 segundos para estabilizar o tráfego. Você deve ver:
- Muitas requisições GET `/health` e `/validate` com **200** ou **401** (chave inválida)
- Taxa de requisições constante (RPS)
- Taxa de erro inicial próxima a **0%**

### 3. Ligar o chaos

**No Terminal 3 (novo processo):**

```bash
./prova-real.sh on 100
```

Saída esperada:
```
[OK] Endpoint: http://<ingress-ip>
[OK] MASTER_KEY obtida
[INFO] Ligando chaos com taxa de erro: 100%
{
  "enabled": true,
  "error_rate_percent": 100,
  "message": "Chaos injetado ligado"
}
[OK] Chaos ligado!
```

### 4. Observar o impacto (2-3 minutos)

**Terminal 1 (Grafana):**
- Painel de **taxa de erros 5xx**: sobe de ~0% para **~100%** (exceto `/health`)
- Painel de **réplicas**: observe se o HPA começa a escalar (consome mais recursos)
- Painel de **logs**: filtre por `erro` ou `falha injetada` para ver as mensagens

**Terminal 2 (pods):**
```
auth-service-deployment-xxxx   1/1     Running   0          5m
flags-service-deployment-yyyy  1/1     Running   0          5m
targeting-service-deploy-zzzz  1/1     Running   0          5m
evaluation-service-dep-wwww    1/1     Running   0          5m
```

Note que o `auth-service` continua com status **Running** (liveness probe passa).

**Terminal 3 (tráfego):**
```
[16:45:23] 200 GET /health (total: 45, ok: 23, erro: 22)
[16:45:24] 500 GET /validate (total: 46, ok: 23, erro: 23)
[16:45:25] 500 GET /validate (total: 47, ok: 23, erro: 24)
...
```

A taxa de erro sobe para **~50%** (alternando /health e /validate).

### 5. Alerta disparando

**Após 5 minutos:**

Grafana Alerting / AlertManager status muda de **"Inactive"** para **"Firing"** para o alerta `AuthServiceHighErrorRate`.

**Webhook dispara:**
- PagerDuty/OpsGenie: novo incidente criado (severidade Critical/High)
- Slack/Discord: mensagem detalhada com link para o dashboard e runbook

### 6. Self-Healing executando (automático)

Assim que o alerta entra em **Firing**, o webhook dispara um GitHub Actions workflow parametrizado:

```yaml
name: Self-Healing
on:
  repository_dispatch:
    types: [service-alert]

jobs:
  restart-service:
    runs-on: ubuntu-latest
    steps:
      - uses: aws-auth
      - run: |
          SERVICE_NAME=${{ github.event.client_payload.service }}
          kubectl rollout restart deployment/${SERVICE_NAME}-deployment -n toggle
          kubectl rollout status deployment/${SERVICE_NAME}-deployment -n toggle
```

**Terminal 2 (pods):**

O `auth-service-deployment` entra em **1/0 (terminating)**, depois volta a **1/1 (Running)** após ~30 segundos.

```
auth-service-deployment-old    0/1     Terminating   0          10m
auth-service-deployment-new    1/1     Running       0          5s
```

**Terminal 3 (tráfego):**

Após o restart, o chaos em memória é perdido. As requisições voltam a:
- `/health`: **200** (sempre)
- `/validate`: **401** ou **200** (depende da API key)

```
[16:51:30] 200 GET /health (total: 380, ok: 190, erro: 190)
[16:51:31] 200 GET /validate (total: 381, ok: 190, erro: 191)
[16:51:32] 200 GET /health (total: 382, ok: 191, erro: 191)
[16:51:33] 200 GET /validate (total: 383, ok: 192, erro: 191)  <- Erros param de crescer
```

### 7. Taxa de erro volta a 0%

**Terminal 1 (Grafana):**

Painel de **taxa de erros 5xx** desce de **100%** para **0%** (em tempo real).

Alerta status muda de **"Firing"** para **"Resolved"** após 5 minutos de taxa normal.

**Incidente no PagerDuty/OpsGenie:** status muda para **Resolved** ou **Acknowledged + Auto-resolved**.

**Notificação no Slack/Discord:** mensagem de resolução.

### 8. Desligar o chaos (opcional, para novo teste)

```bash
./prova-real.sh off
./prova-real.sh status
```

## Efeito em Cascata

### Observação

Se o `auth-service` está retornando 500 em `/validate`, os serviços que chamam essa rota também falham:

- **flags-service** → chama `auth-service` GET `/validate` para autenticar → **500 erro de upstream**
- **targeting-service** → chama `auth-service` GET `/validate` para autenticar → **500 erro de upstream**
- **evaluation-service** → chama `flags-service` e `targeting-service` → **500 erro de upstream**

Portanto, **todos os 5 serviços podem ver aumento de 5xx**, não apenas o `auth-service`.

### Alertas que disparam

1. **"AuthServiceHighErrorRate"** (primário)
   - Condição: `http_server_request_duration_seconds_count{http_response_status_code="500", service="auth"} / total > 5%`
   - Webhook dispara restart do `auth-service`

2. **"FlagsServiceHighErrorRate"** (cascata)
   - Condição: `http_server_request_duration_seconds_count{http_response_status_code="500", service="flags"} / total > 5%`
   - Webhook dispara restart do `flags-service`
   - **MAS** isso não resolve o problema original (erro vem do `auth`)

3. **Idem para `targeting-service` e `evaluation-service`**

### Resolução esperada

Apenas o restart do `auth-service` resolve o problema (cura a "raiz"):

1. Mesmo que flags/targeting/evaluation também disparem, seus alertas só resolvem se **auth ficar saudável primeiro**
2. Após restart do auth:
   - auth volta a 200 em /validate
   - flags/targeting/evaluation param de receber 500 (podem ainda dar erro por outra razão, mas não por auth)
   - Seus alertas ficam pendentes ou também resolvem se a causa foi só auth

Isso é **aceitável e documentado** neste roteiro.

## Checklist para o Vídeo

- [ ] Cluster EKS e ArgoCD visíveis (Applications em Synced/Healthy)
- [ ] Dashboard Grafana aberto, painel de 5xx visível
- [ ] Terminal com `kubectl get pods -w` aberto
- [ ] Tráfego começando (`gerar-trafego.sh` rodando)
- [ ] Taxa de erro ~0% antes do chaos
- [ ] Chaos ligado (`./prova-real.sh on 100`)
- [ ] Taxa de erro sobe para ~100% (no painel 5xx)
- [ ] Alerta passa para "Firing" (status ou notificação visível)
- [ ] Incidente criado no PagerDuty/OpsGenie (screenshot ou print)
- [ ] Notificação chegando no Slack/Discord
- [ ] GitHub Actions rodando (log de self-healing)
- [ ] Pod do auth-service reiniciando (Terminal 2)
- [ ] Taxa de erro voltando a 0%
- [ ] Alerta "Resolved"
- [ ] Incidente fechado no PagerDuty/OpsGenie

## Notas de Execução

### Timing

- **Geração de tráfego**: deixe rodando continuamente
- **Aguardando avaliação do alerta**: 5-10 minutos (depende da janela de avaliação configurada)
- **Restart do pod**: ~30 segundos
- **Resolução do alerta**: mais 5 minutos após volta ao normal

**Tempo total esperado: 15-25 minutos**

### Troubleshooting

**O chaos não liga:**
```bash
# Verifica se a rota existe (CHAOS_ENABLED=true)
kubectl get configmap auth-configmap -n toggle -o jsonpath='{.data.CHAOS_ENABLED}'
```

**O pod não reinicia:**
```bash
# Verifica logs do GitHub Actions (se houver webhook)
gh run list --repo NyEstevo/fiap-tech-challenges

# Manual fallback:
kubectl rollout restart deployment/auth-service-deployment -n toggle
```

**Alerta não dispara:**
```bash
# Verifica Prometheus:
kubectl port-forward -n toggle svc/prometheus 9090:9090
# Abra http://localhost:9090/graph
# Query: http_server_request_duration_seconds_count{http_response_status_code="500", service="auth"}
```

### Limpeza

Sempre desligar o chaos ao terminar:

```bash
./prova-real.sh off
```

Assim, o cluster volta ao estado limpo para o próximo teste ou apresentação.
