<div align="center">
  <br/>
  <hr style="border: none; border-top: 1px solid #ED145B; width: 100%; margin: 0 auto"/>
</div>

![/.github/LOGO-FIAP.png](/.github/LOGO-FIAP.png)

# Tech Challenge - Objetivos da Fase 3

Na Fase 2, evoluímos o Toggle Master de monolito para uma arquitetura de microsserviços containerizada em Kubernetes. No entanto, a operação continuava manual: deploys feitos via `kubectl apply` a partir de máquinas locais, credenciais de banco em arquivos de texto e ambientes recriados manualmente pelo console AWS. Na Fase 3, o objetivo é eliminar esse trabalho manual: "se não está no código, não existe".

Automatizamos toda a infraestrutura e o ciclo de vida dos 5 microsserviços do ToggleMaster (`auth`, `flag`, `targeting`, `evaluation`, `analytics`) com **Infraestrutura como Código (Terraform)**, **pipelines de CI/DevSecOps** e **entrega contínua via GitOps com ArgoCD**.

## Entregáveis da Fase 3

1. **Vídeo de Demonstração (até 20 minutos):** [link do vídeo]

2. **Documentação Separada por Tópicos:**

- [O Problema: Operação Insustentável](#o-problema-operação-insustentável);
- [Infraestrutura como Código (Terraform)](#infraestrutura-como-código-terraform);
- [Backend Remoto do Terraform State](#backend-remoto-do-terraform-state);
- [Pipeline de CI & DevSecOps](#pipeline-de-ci--devsecops);
- [Entrega Contínua (CD) & GitOps](#entrega-contínua-cd--gitops);
- [Dificuldades Encontradas](#dificuldades-encontradas);
- [Estimativa de Custos](#estimativa-de-custos);
- [Diagrama de Arquitetura](#diagrama-de-arquitetura);
- [Integrantes do Grupo](#integrantes-do-grupo);

3. **Código Fonte no Repositório:**

- Código Terraform completo, organizado em módulos.
- Workflows de CI/CD (GitHub Actions) com os passos de DevSecOps.
- Manifestos Kubernetes ajustados para GitOps.

4. **Relatório de Entrega (.PDF ou .txt):** nomes dos participantes, link da documentação e do vídeo, resumo de desafios/decisões e print da estimativa de custos da AWS.

5. **Integrantes do Grupo**:

- [Aline Estevo da Silva](https://www.linkedin.com/in/aline-estevo)
- [Thiago de Melo Macedo](https://www.linkedin.com/in/thiago-melo-macedo)
- [Jefferson Fernandes de Freitas](#)
- [Vinicius Jorge de Oliveira](#)

> **Importante — Ambiente de Nuvem (Terraform & IAM):** assim como nas fases anteriores, trabalhamos sob as restrições do AWS Academy: o código Terraform **não cria Roles ou Policies de IAM**. O cluster EKS e os Node Groups utilizam a `LabRole` existente, importada via `data source` do Terraform.

## O Problema: Operação Insustentável

A arquitetura de microsserviços aprovada na Fase 2 trouxe quatro problemas operacionais que motivam a Fase 3:

- **Deploys manuais e conflitantes:** desenvolvedores rodando `kubectl apply` a partir de máquinas locais, sem fonte única de verdade, gerando conflitos de versão entre ambientes.
- **Credenciais inseguras:** credenciais de banco de dados trafegando em arquivos de texto sem criptografia ou gestão de secrets.
- **Vulnerabilidades não detectadas:** uma vulnerabilidade em uma biblioteca Go chegou à produção sem ser barrada em nenhuma etapa do processo.
- **Ambientes não reprodutíveis:** recriar o ambiente de homologação levava dias, por ter sido provisionado manualmente no console AWS.

A resposta a esses quatro pontos é, respectivamente: GitOps (fonte única de verdade), DevSecOps com secrets gerenciados, security scanning obrigatório no pipeline, e Infraestrutura como Código.

## Infraestrutura como Código (Terraform)

Toda a infraestrutura que era provisionada manualmente na Fase 2 foi substituída por um projeto Terraform organizado em módulos, provisionando:

1. **Networking:** VPC, Subnets públicas e privadas, Internet Gateway e Route Tables.
2. **Cluster EKS:** cluster Kubernetes e Node Groups, associados à `LabRole` do AWS Academy.
3. **Bancos de Dados:**
   - 3 instâncias RDS (PostgreSQL) — uma para cada serviço com dados relacionais (`auth-service`, `flag-service`, `targeting-service`), preservando o isolamento de *database-per-service* já adotado na Fase 2.
   - 1 Cluster ElastiCache (Redis).
   - 1 tabela DynamoDB (`ToggleMasterAnalytics`), usada pelo `analytics-service`.
4. **Mensageria:** 1 fila SQS, consumida pelo `analytics-service` e usada como gatilho de escalabilidade do KEDA.
5. **Repositórios:** 5 repositórios ECR (um por microsserviço), provisionados via Terraform.

Árvore real do Terraform (`fase-3/infra/`):

```
infra/
├── bootstrap/                 # state local; roda 1x fora do CI
│   ├── (lab)  main/variables/outputs.tf     # bucket S3 + lock DynamoDB
│   └── prod/  main/variables/outputs.tf     # bucket S3 + lock + OIDC + IAM (conta pessoal)
└── terraform/
    ├── modules/
    │   ├── networking/        # VPC, subnets pub/priv, IGW, NAT, route tables
    │   ├── eks/               # cluster + node group (LabRole no lab; roles próprias no prod)
    │   ├── rds/               # instância PostgreSQL + SG + secret (for_each: auth/flag/targeting)
    │   ├── elasticache/       # replication group Redis
    │   ├── dynamodb/          # tabela de eventos do analytics
    │   ├── sqs/               # fila + DLQ (SSE gerenciado)
    │   ├── ecr/               # repositórios (for_each), scan-on-push, lifecycle
    │   ├── addons/            # helm_release: metrics-server, ingress-nginx, keda, external-secrets, argocd
    │   └── iam_oidc_github/   # OIDC provider + role (usado só pelo bootstrap/prod)
    ├── envs/
    │   ├── lab/               # APLICADO — LabRole, 2 AZs, single-AZ RDS
    │   └── prod/              # código pronto, NÃO aplicado — HA, IAM próprio, OIDC
    ├── .checkov.yaml          # gate bloqueante (soft-fail:false) + baseline Academy
    └── .tflint.hcl
```

Regras seguidas: `required_providers` fixados com `~>`; `for_each` onde a chave
importa (serviços RDS, repos ECR); `locals` para transformações; blocos
`validation {}` nos inputs críticos dos módulos; outputs sensíveis com
`sensitive = true`; `tags`/`default_tags` padronizadas via `local.common_tags`.

## Backend Remoto do Terraform State

O `terraform.tfstate` não é mantido localmente. O backend remoto é um **Bucket
S3** (`tc-fiap-tfstate-361075236043`, versionado e com encryption), com
**state locking via tabela DynamoDB** (`tc-fiap-tflock`) — `use_lockfile` (lock
nativo no S3) exige Terraform ≥ 1.10 e o projeto fixa `1.9.8`; a migração para
`use_lockfile` está documentada como passo futuro. O lock evita aplicações
concorrentes e perda de estado entre máquinas/pipelines. `envs/lab` e `envs/prod`
usam chaves distintas (`fase-3/lab/…` e `fase-3/prod/…`) no mesmo bucket (lab) ou
em bucket próprio (prod, criado por `bootstrap/prod`).

## Pipeline de CI & DevSecOps

Cada um dos 5 microsserviços tem seu próprio workflow de CI
(`.github/workflows/ci-<svc>.yml`), disparado em Pull Requests e em pushes para
`lab`/`main` que tocam `fase-2/<svc>-service/**`. Todos chamam o workflow
reutilizável **`_reusable-ci-service.yml`** (parametrizado por `service_name`,
`service_language`, `working_directory`). A autenticação AWS é a composite
action **`.github/actions/aws-auth`** (lab → chaves estáticas de sessão;
`main` → OIDC). Estágios sequenciais — cada job depende do anterior:

1. **`build-and-test`:** Go → `go build` + `go vet` + `go test -race -cover`;
   Python → `pip install` + `pytest --cov`. Cobertura publicada como artifact.
2. **`lint` (bloqueante):** Go → `golangci-lint` v2 (config `.golangci.yml` por
   serviço); Python → `ruff check` + `ruff format --check` (`fase-2/.ruff.toml`).
3. **`security-sast-sca` (bloqueante em CRÍTICO):**
   - **SCA:** `trivy fs --severity CRITICAL --exit-code 1` nas dependências.
   - **SAST:** `gosec -severity high` (Go) e `bandit -ll -ii` (Python).
4. **`docker-build-scan-push`:** `docker build --target prod` → **`trivy image
   --severity CRITICAL --exit-code 1` antes do push** → `amazon-ecr-login` →
   push `:<github.sha>` (`auth` também publica a imagem `-migrate-image`).
   Só em push (não em PR).
5. **`gitops-update`** (só push na `lab`): `kustomize edit set image` no overlay
   do serviço e commit `[skip ci]` na branch `lab`.

**Infra:** `infra-tf-plan` (PR) e `infra-tf-apply` (push) rodam
`terraform fmt/validate` + `tflint` + **`checkov` com `soft-fail: false`**
(`.checkov.yaml` com baseline justificada para o AWS Academy) +
**`trivy config --severity CRITICAL --exit-code 1`**.

A demonstração da regra de bloqueio (dependência com CVE crítico → pipeline
vermelho) está em [`DEVSECOPS-DEMO.md`](./DEVSECOPS-DEMO.md).

## Entrega Contínua (CD) & GitOps

Abandonamos o push direto de manifests via CI em favor de **GitOps**:

1. **Pasta de GitOps dedicada:** [`fase-3/gitops/`](./gitops/) — separada do
   código dos serviços. Os overlays Kustomize (`gitops/manifests/<svc>/`)
   reaproveitam os manifests da Fase 2 (`fase-2/<svc>-service/k8s/`) sem
   duplicar e **omitem o `secrets.yaml`** (base64), substituído por
   `ExternalSecret` + `ClusterSecretStore` do External Secrets Operator.
2. **ArgoCD:** instalado no EKS via Terraform (`modules/addons`, `helm_release`).
   Modelo **app-of-apps**: `gitops/root-app.yaml` (aponta para `gitops/apps/`
   com `directory.recurse`) gera 6 `Application` — os 5 microsserviços + a
   `platform` (ESO), esta com `sync-wave: -1`. `syncPolicy.automated` com
   `prune` e `selfHeal`.
3. **Atualização automática:** o job `gitops-update` do CI roda
   `kustomize edit set image` no `kustomization.yaml` do serviço e commita o
   bump (tag = `github.sha`) na branch `lab`.
4. **Sync:** o ArgoCD observa a branch `lab`, detecta o commit e sincroniza o
   cluster — sem `kubectl apply` local.

Cada microsserviço é uma `Application` independente, sincronizada do respectivo
`fase-3/gitops/manifests/<svc>`.

## Dificuldades Encontradas

- **Conta/região herdadas erradas da Fase 2:** os manifests e o Terraform vinham
  com `us-east-2`/`047719652987` (fase anterior); o Academy usa outra conta
  (`361075236043`) e não libera `us-east-2` no Learner Lab. Exigiu ajuste em
  bootstrap, envs, módulos, gitops e workflows.
- **Access Entry errado impedindo os nodes de entrar no cluster:** a `LabRole`
  (role dos nodes) estava com Access Entry `STANDARD` em vez de `EC2_LINUX`.
  Sem o tipo certo, os nodes não entram em `system:nodes`, o EKS não assina os
  CSRs `kubelet-serving` e `metrics-server`/`kubectl top|logs|exec` quebram com
  `tls: internal error` — causa raiz nada óbvia de se rastrear.
- **Versões de EKS/RDS fora de suporte na região:** `cluster_version 1.30` saiu
  de suporte (sem AMI de node group disponível) e o `engine_version 16.4` do
  RDS não existe em `us-east-1`; precisou bump para EKS 1.31 e Postgres 16.9.
- **`kubernetes_manifest` exige API viva já no `plan`:** a Application raiz do
  ArgoCD usava esse recurso e quebrava o primeiro `apply` (cluster ainda não
  existe). Resolvido colocando-a atrás da flag `bootstrap_gitops_root_app`,
  ligada só no 3º apply (depois que EKS + ArgoCD já estão no ar).
- **Ausência de IRSA no AWS Academy:** sem permissão para criar IAM Roles, não
  há como dar credenciais AWS "nativas" a pods. Isso bloqueou o
  `ClusterSecretStore` do ESO, o `TriggerAuthentication` do KEDA e o worker SQS
  do `analytics`/`evaluation` (erro `NoCredentialProviders`/`no EC2 IMDS role
  found`). Resolvido com `Secret`s de credenciais estáticas de sessão,
  recriados a cada `terraform apply` (o token de sessão do Academy expira em
  ~4h).
- **Deadlock de sincronização no ArgoCD:** o Job de `migration` (hook
  `PreSync`) referenciava um Secret que só era materializado pelo
  `ExternalSecret` na fase `Sync` normal — ou seja, depois do `PreSync`. O Job
  travava em `CreateContainerConfigError` e bloqueava a fase `PreSync` inteira,
  o que impedia o próprio `ExternalSecret` de ser aplicado (deadlock completo
  em cluster novo). Resolvido tornando o `ExternalSecret` também um hook
  `PreSync`, numa `sync-wave` anterior à do Job.
- **Falso "OutOfSync" permanente no ArgoCD:** o webhook do External Secrets
  Operator injeta campos default de schema do CRD (`conversionStrategy`,
  `decodingStrategy`, `deletionPolicy`) que não existem no Git. O diff
  client-side do ArgoCD via isso como drift eterno e reaplicava um no-op sem
  parar. Corrigido ativando `ServerSideDiff` no `application-controller`.
- **Capacidade de pods insuficiente nos nós de lab:** 2× `t3.medium` suportam
  ~34 pods (VPC CNI sem prefix delegation); os 5 serviços com 3 réplicas +
  HPAs mínimos não cabiam, gerando `FailedScheduling` ("Too many pods").
  Precisou reduzir réplicas repetidas vezes (idas e vindas entre 1, 2 e 3) e
  remover o HPA do `analytics` que conflitava com o autoscaler do KEDA.
- **Cold start dos pods:** os probes de readiness falhavam com "context
  deadline exceeded" no `/health` logo após o cluster subir. Não era bug de
  aplicação — o limite de CPU (40m) era baixo demais para o boot do gunicorn +
  inicialização do pool de conexões Postgres + primeira chamada TLS a um RDS
  `t3.micro` frio, tudo competindo por CPU dentro do timeout do probe.
- **CVE crítica herdada da imagem base do Go:** o Dockerfile de `auth` e
  `evaluation` usava `golang:1.21-alpine` também no estágio final, então o
  runtime carregava o toolchain Go inteiro com a stdlib 1.21 vulnerável
  (CVE-2025-68121, `crypto/tls`, CRITICAL). Corrigido com multi-stage real:
  builder `golang:1.25-alpine` + runtime `alpine:3.21` mínimo, só com o
  binário.
- **`gunicorn` dependendo de pacote ausente:** a versão 20.1.0 fazia `import
  pkg_resources`, que não vem instalado por padrão em `python:3.12-alpine`
  (sem `setuptools`) — os pods de `flag`/`targeting` entravam em
  `CrashLoopBackOff`. Resolvido subindo para `gunicorn` 23 (usa
  `importlib.metadata`, sem essa dependência).
- **Instabilidade de Actions de terceiros no CI:** o `aquasecurity/trivy-action`
  mudou o formato de tag (sem prefixo `v`) e depois teve a tag referenciada
  pelo `setup-trivy` removida do repositório upstream; o
  `golangci-lint-action@v6` não suporta `golangci-lint` v2. Problemas de
  infraestrutura de terceiros fora do controle do time, exigindo
  investigação e re-pin a cada quebra.
- **Bump de imagem sem regenerar os artefatos de contrato:** o job
  `gitops-update` só commitava o `kustomization.yaml` com a nova tag; os
  golden files em `fase-3/gitops/.render/` ficavam desatualizados e o gate
  `make render-diff` ficava vermelho na branch `lab` depois de cada deploy,
  até alguém rodar `render-baseline` manualmente. Resolvido incluindo essa
  regeneração no mesmo commit do bump.

Vários desses problemas só apareceram depois que o ambiente `lab` foi
efetivamente provisionado e operado na AWS (não em `terraform validate`/CI),
inclusive após pelo menos um ciclo completo de destroy e reconstrução da
infraestrutura para controlar custo/tempo de sessão do AWS Academy.

## Estimativa de Custos

![Estimativa de Custos AWS](/.github/estimativa-custos-fase3.png)

> Inserir aqui o print da estimativa de custos da AWS (Cost Explorer ou AWS Pricing Calculator), conforme exigido no relatório de entrega.

## Diagrama de Arquitetura

- Diagrama (SVG): [link do diagrama SVG]

![Diagrama de Arquitetura Fase 3](/.github/DiagramaArquiteturaFase3.png)

## Integrantes do Grupo

- [Aline Estevo da Silva](https://www.linkedin.com/in/aline-estevo)
- [Thiago de Melo Macedo](https://www.linkedin.com/in/thiago-melo-macedo)
- [Jefferson Fernandes de Freitas](#)
- [Vinicius Jorge de Oliveira](#)
- [Maurício Magnago Castro Sá](https://www.linkedin.com/in/mcastrosa)