# Roteiro da "Prova Real": falha → alerta → incidente → self-healing

Este é o roteiro do trecho do vídeo que prova o ciclo completo da Fase 4:

```
auth começa a responder 500  (o /health continua 200: falha "silenciosa")
  → métricas OTel mostram 5xx > 5% por 2 min       (Grafana: alerta Firing)
  → PagerDuty abre incidente + Slack avisa          (contact point togglemaster-incidente)
  → Grafana chama a GitHub Action self-healing.yml  (workflow_dispatch, ref lab)
  → kubectl rollout restart do auth                 (pod novo, sem o estado de falha)
  → 5xx volta a 0 → alerta Resolved → incidente fecha sozinho
```

**Analogia:** é um teste de alarme de incêndio. Acendemos uma vela embaixo do sensor (chaos), e o alarme toca (Grafana). O alarme liga para os bombeiros (PagerDuty) e avisa no grupo do prédio (Slack), e o sprinkler automático (self-healing) apaga a vela. Ninguém precisa encostar em nada.

## Por que a falha é "honesta"

O chaos fica **só na memória** do processo do auth (`fase-2/auth-service/chaos.go`). Quando o self-healing reinicia o pod, o processo novo nasce sem o chaos, então o restart **cura de verdade**. Não é teatro.

Os outros serviços não entram em cascata: o `flags` e o `targeting` transformam a falha do auth em **401** (chave não validada), que é erro do cliente e não conta no alerta de 5xx. Só o `auth` dispara.

## Pré-requisitos (antes de gravar)

- [ ] PRs das fatias 7 a 11 mergeados e todas as Applications `Synced/Healthy` no ArgoCD.
- [ ] Secret `tc-observability` no AWS Secrets Manager com as 5 chaves (ver README da fase-4).
- [ ] Os secrets AWS do GitHub (`AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_SESSION_TOKEN`) atualizados **nesta** sessão do AWS Academy. Senão a Action não consegue falar com o EKS.
- [ ] Teste prévio do self-healing: `gh workflow run self-healing.yml --ref lab -f service=auth` termina verde.
- [ ] **Espere 10 min depois desse teste antes de gravar.** O cooldown do self-healing pula um restart feito há menos de 10 min.

## Terminais

```bash
# T1 - Grafana (http://localhost:3000, usuário admin)
kubectl -n observability get secret grafana-admin -o jsonpath='{.data.admin-password}' | base64 -d; echo
kubectl -n observability port-forward svc/kube-prometheus-stack-grafana 3000:80

# T2 - pods do auth (mostra o pod sendo trocado)
kubectl -n toggle get pods -l app=auth -w

# T3 - tráfego contínuo (via ingress; /validate=401 normal, 500 com chaos)
fase-4/scripts/gerar-trafego.sh

# T4 - controle do chaos
fase-4/scripts/prova-real.sh status
```

Abas do navegador: dashboard **Saúde do Ecossistema ToggleMaster**, **Alerting → Alert rules** (pasta ToggleMaster), PagerDuty (Incidents), canal do Slack, **GitHub → Actions → self-healing** e Datadog **APM → Service Map**.

## Passo a passo (≈ 6 a 8 min de vídeo)

| Tempo | Ação | O que mostrar |
|---|---|---|
| 0:00 | T3 rodando há pelo menos 1 min | dashboard com auth em 0% de 5xx; Service Map do Datadog com os 5 serviços |
| 0:30 | `fase-4/scripts/prova-real.sh on` | T3 passa a mostrar `/validate=500` e `/health=200` ("o Kubernetes acha que está tudo bem") |
| 1:00 | | painel "Taxa de erros 5xx" sobe para ~100% no auth; alerta em **Pending** |
| ~2:30 | | alerta **Firing** (2 min sustentado + avaliação a cada 30s) |
| ~2:45 | | incidente aberto no PagerDuty; mensagem "ALERTA: auth-service" no Slack com runbook e dashboard |
| ~3:00 | | GitHub Actions: execução `self-healing` em andamento (motivo = nome do alerta) |
| ~4:00 | | T2: pod antigo `Terminating`, pod novo `Running`; Slack: "auth-service reiniciado" |
| ~4:00 | | T3 volta a `/validate=401`; `prova-real.sh status` mostra `"enabled": false` |
| ~6:00 | | painel de 5xx desce (janela de 2 min); alerta **Resolved**; incidente **Resolved** no PagerDuty; Slack "RESOLVIDO" |

Extra para o APM: no Datadog, filtre `service:auth-service status:error` e abra um trace do período do chaos. Mostre também o log do mesmo `trace_id` no Loki (Grafana → Explore → Loki).

## Se algo não acontecer

| Sintoma | Onde olhar |
|---|---|
| `prova-real.sh` responde 404 | O pod ainda é anterior ao merge da fatia 11. Confira se `kubectl -n toggle exec deploy/auth-deployment -- env \| grep CHAOS` mostra `true`; o ConfigMap não reinicia o pod sozinho, então faça `kubectl -n toggle rollout restart deploy/auth-deployment`. |
| Alerta não sai de Normal | Precisa haver tráfego (> 0,1 req/s): o T3 está rodando? No Grafana → Explore, rode a query do alerta (está na regra). |
| Firing, mas sem PagerDuty/Slack | O Grafana subiu antes do secret existir: `kubectl -n observability rollout restart deploy/kube-prometheus-stack-grafana`. |
| Action não dispara | Grafana → Alerting → Contact points → `togglemaster-incidente` → erro do webhook. 401/403 = token sem a permissão *Actions: write*. |
| Action falha no passo AWS | Credenciais do AWS Academy expiradas: atualize os 3 secrets do GitHub juntos. |
| Action diz "cooldown" | Houve restart há menos de 10 min. É a trava contra loop funcionando; espere ou mostre isso como evidência. |

Plano B (sem esperar o alerta): `gh workflow run self-healing.yml --ref lab -f service=auth -f motivo=demo` mostra a mesma cura.

## Limpeza

```bash
fase-4/scripts/prova-real.sh off   # se o self-healing não rodou
# Ctrl+C no T3
```
