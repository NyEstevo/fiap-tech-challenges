<div align="center">
  <br/>
  <hr style="border: none; border-top: 1px solid #ED145B; width: 100%; margin: 0 auto"/>
</div>

![/.github/LOGO-FIAP.png](/.github/LOGO-FIAP.png)

# Tech Challenge - Objetivos da Fase 3

Na Fase 2, evoluímos o Toggle Master de monolito para uma arquitetura de microsserviços containerizada em Kubernetes. No entanto, a operação continuava manual: deploys feitos via `kubectl apply` a partir de máquinas locais, credenciais de banco em arquivos de texto e ambientes recriados manualmente pelo console AWS. Na Fase 3, o objetivo é eliminar esse trabalho manual: "se não está no código, não existe".

Automatizamos toda a infraestrutura e o ciclo de vida dos 5 microsserviços do ToggleMaster (`auth`, `flag`, `targeting`, `evaluation`, `analytics`) com **Infraestrutura como Código (Terraform)**, **pipelines de CI/DevSecOps** e **entrega contínua via GitOps com ArgoCD**.

## Entregáveis da Fase 3

1. **Vídeo de Demonstração (até 20 minutos):** [**Vídeo de Demonstração Tech Challenge Fase 3**](https://youtu.be/2QOhdMUwSYo)

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

4. **Integrantes do Grupo**:

- [Aline Estevo da Silva](https://www.linkedin.com/in/aline-estevo)
- [Thiago de Melo Macedo](https://www.linkedin.com/in/thiago-melo-macedo)

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
   - 3 instâncias RDS (PostgreSQL) — uma para cada serviço com dados relacionais (`auth-service`, `flags-service`, `targeting-service`), preservando o isolamento de *database-per-service* já adotado na Fase 2.
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

**Regras seguidas:**
- `required_providers` fixados com `~>`;
- `for_each` onde a chave importa (serviços RDS, repos ECR);
- `locals` para transformações;
- blocos `validation {}` nos inputs críticos dos módulos;
- outputs sensíveis com `sensitive = true`;
- `tags`/`default_tags` padronizadas via `local.common_tags`.

## Backend Remoto do Terraform State

O `terraform.tfstate` não é mantido localmente. O backend remoto é um **Bucket
S3** (`tc-fiap-tfstate-[account-id]`, versionado e com encryption), com
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
   reaproveitam os manifests da Fase 2 (`fase-2/<svc>-service/k8s/base`) sem
   duplicar e **omitem o `secrets.yaml`** (base64), substituído por
   `ExternalSecret` + `ClusterSecretStore` do External Secrets Operator
   (materializados a partir do AWS Secrets Manager). Cada overlay ainda soma
   os add-ons específicos do ambiente lab: Jobs de `migration`/`seed` dedicados
   (ex.: `auth` roda um hook PostSync que lê `SERVICE_API_KEY` do secret do
   `evaluation`) e o teto de réplicas (`1` por serviço, adequado ao cluster do
   AWS Academy).
2. **ArgoCD:** instalado no EKS via Terraform (`modules/addons`, `helm_release`).
   Modelo **ApplicationSet** (`gitops/applicationset.yaml`) com um único
   *git directory generator* sobre `gitops/manifests/*`, que gera/adota as
   7 `Application` do cluster (os 5 microsserviços + `platform`, com ESO, +
   `ingress`, com o NGINX único da Fase 2) — um nome por diretório, sem
   precisar editar o ApplicationSet a cada novo serviço. `platform` recebe
   `sync-wave: -1` via `templatePatch`, garantindo que o `ClusterSecretStore`
   exista antes dos `ExternalSecret` dos serviços. `syncPolicy.automated` com
   `prune` e `selfHeal`.
   - **Por que ApplicationSet e não app-of-apps:** o modelo anterior
     (`root-app.yaml` apontando para `gitops/apps/*.yaml`, um arquivo por
     serviço) exigia criar um novo arquivo de `Application` a cada serviço
     novo — na prática, violava a regra de "não criar arquivo por serviço"
     que já vale para Terraform e CI. Com o *directory generator*, basta criar
     a pasta `gitops/manifests/<novo-svc>/` que a `Application` correspondente
     é gerada automaticamente.
   - **Migração sem downtime:** trocar o "dono" das `Application` no ArgoCD é
     arriscado — se o ApplicationSet gerar um nome diferente do que já existe
     no cluster, o ArgoCD entende que a `Application` antiga ficou órfã e,
     com `prune` ativo, apaga os pods. Por isso a transição usa
     `applicationsSync: create-only` + `preserveResourcesOnDeletion: true`:
     o ApplicationSet **adota** (via `ownerReference`) as 7 `Application` que
     já existiam, sem reescrever spec nem deletar nada. Só depois de
     confirmar `Synced/Healthy` nas 7 é que a política aperta para `sync`,
     num segundo commit.
3. **Atualização automática:** o job `gitops-update` do CI roda
   `kustomize edit set image` no `kustomization.yaml` do serviço e commita o
   bump (tag = `github.sha`) na branch `lab`.
4. **Sync:** o ArgoCD observa a branch `lab` via o *generator* do
   ApplicationSet, detecta o commit e sincroniza o cluster — sem
   `kubectl apply` local.

Cada microsserviço (e cada recurso de plataforma) é uma `Application`
independente, gerada a partir do respectivo `fase-3/gitops/manifests/<svc>`.

## Dificuldades Encontradas

## 5. Dificuldades encontradas

- **Conta/região herdadas erradas da Fase 2:** os manifests e o Terraform
  vinham com `us-east-2`/`047719652987` (fase anterior); o Academy usa outra
  conta (`238903440658`) e não libera `us-east-2` no Learner Lab.
- **Access Entry errado impedindo os nodes de entrar no cluster:** a `LabRole`
  estava com Access Entry `STANDARD` em vez de `EC2_LINUX`, quebrando
  `metrics-server`/`kubectl top|logs|exec` com `tls: internal error` — causa
  raiz nada óbvia de se rastrear.
- **Versões de EKS/RDS fora de suporte na região:** `cluster_version 1.30`
  saiu de suporte e o `engine_version 16.4` do RDS não existe em
  `us-east-1`; precisou bump para EKS 1.31 e Postgres 16.9.
- **`kubernetes_manifest` exige API viva já no `plan`:** quebrava o primeiro
  `apply` (cluster ainda não existe); resolvido com a Application raiz do
  ArgoCD atrás de uma flag, ligada só no 3º apply.
- **Ausência de IRSA no AWS Academy:** bloqueou credenciais "nativas" para o
  ESO, o `TriggerAuthentication` do KEDA e o worker SQS de
  `analytics`/`evaluation`; resolvido com `Secret`s de credenciais estáticas.
- **Deadlock de sincronização no ArgoCD:** o Job de `migration` (`PreSync`)
  dependia de um Secret só materializado pelo `ExternalSecret` na fase `Sync`
  normal — travava em `CreateContainerConfigError` e bloqueava tudo. Resolvido
  tornando o `ExternalSecret` também um hook `PreSync` numa wave anterior.
- **Falso "OutOfSync" permanente no ArgoCD:** causado pelos defaults de schema
  que o webhook do ESO injeta; corrigido com `ServerSideDiff`.
- **Capacidade de pods insuficiente nos nós de lab:** 2× `t3.medium` suportam
  ~34 pods; 5 serviços × 3 réplicas + HPAs mínimos geravam
  `FailedScheduling`; precisou reduzir réplicas e remover o HPA do
  `analytics` que conflitava com o KEDA.
- **Cold start dos pods:** probes de readiness falhavam com "context deadline
  exceeded" porque o limite de CPU (40m) era baixo demais para o boot do
  gunicorn + pool de conexões Postgres + primeira chamada TLS a um RDS
  `t3.micro` frio.
- **CVE crítica herdada da imagem base do Go** (CVE-2025-68121): o runtime
  carregava o toolchain Go inteiro com stdlib 1.21 vulnerável; corrigido com
  multi-stage real (builder 1.25 + runtime alpine mínimo).
- **`gunicorn` dependendo de pacote ausente:** a versão 20.1.0 fazia
  `import pkg_resources`, ausente em `python:3.12-alpine`; pods em
  `CrashLoopBackOff`; resolvido subindo para `gunicorn` 23.
- **Instabilidade de Actions de terceiros no CI:** mudanças de formato de tag
  no `trivy-action` e remoção de tag pelo `setup-trivy`;
  `golangci-lint-action@v6` incompatível com `golangci-lint` v2.
- **Bump de imagem sem regenerar os artefatos de contrato:** o job
  `gitops-update` deixava `.render/` desatualizado, quebrando o gate
  `make render-diff`; resolvido incluindo a regeneração no mesmo commit.

Vários desses problemas só apareceram depois que o ambiente `lab` foi
efetivamente provisionado e operado na AWS (não em `terraform validate`/CI),
inclusive após pelo menos um ciclo completo de destroy e reconstrução da
infraestrutura para controlar custo/tempo de sessão do AWS Academy.

## Estimativa de Custos

![Estimativa de Custos AWS](/.github/EstimativaDeCustoFase3.png)
