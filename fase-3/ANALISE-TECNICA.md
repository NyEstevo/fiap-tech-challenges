# Análise Técnica — Fase 3

Este documento consolida a análise do que foi entregue na Fase 3 (IaC, CI/DevSecOps
e GitOps), cruzando o histórico real de commits/PRs (`main`, `lab` e as branches de
hotfix `fix/lab-*`, `#55`–`#87`) com os requisitos do PDF do desafio. Serve de apoio
para o relatório de entrega e para o roteiro do vídeo de demonstração.

## Índice

- [1. Decisões técnicas](#1-decisões-técnicas)
- [2. Fora do escopo do desafio e fora das práticas atuais de mercado](#2-fora-do-escopo-do-desafio-e-fora-das-práticas-atuais-de-mercado)
- [3. Além do escopo do desafio e dentro das melhores práticas de mercado](#3-além-do-escopo-do-desafio-e-dentro-das-melhores-práticas-de-mercado)
- [4. Débitos técnicos](#4-débitos-técnicos)
- [5. Dificuldades encontradas](#5-dificuldades-encontradas)
- [6. Cronograma para o vídeo de demonstração (até 20 min)](#6-cronograma-para-o-vídeo-de-demonstração-até-20-min)

---

## 1. Decisões técnicas

**Infraestrutura (Terraform)**
- Projeto modularizado em 9 módulos reutilizáveis (`networking`, `eks`, `rds`,
  `elasticache`, `dynamodb`, `sqs`, `ecr`, `addons`, `iam_oidc_github`),
  compartilhados entre `envs/lab` (aplicado) e `envs/prod` (pronto, não aplicado).
- Restrição do AWS Academy resolvida com `data source` da `LabRole` + variáveis
  `cluster_role_arn`/`node_role_arn` opcionais no módulo `eks`, que caem para
  `lab_role_arn` quando vazias — permite reusar o mesmo módulo em prod (IAM
  próprio) sem tocar no lab.
- Backend remoto: S3 (`tc-fiap-tfstate-...`) + lock via tabela DynamoDB, em vez
  de `use_lockfile` nativo, porque o projeto está pinado em Terraform 1.9.8
  (recurso exige ≥1.10).
- `.checkov.yaml` bloqueante (`soft-fail: false`) com baseline de exceções
  justificada por comentários (KMS CMK indisponível no Academy, Multi-AZ/
  deletion protection desligados por custo em ambiente efêmero).

**CI/DevSecOps**
- Um único `_reusable-ci-service.yml` parametrizado (`service_name`/
  `service_language`/`working_directory`) chamado por 5 workflows finos,
  evitando duplicar pipeline por serviço.
- Composite action `aws-auth` alterna estratégia de autenticação por branch:
  `lab` → chaves estáticas de sessão do Academy; `main` → OIDC — decisão para
  contornar a ausência de IAM roles duráveis no Academy.
- Pipeline sequencial e bloqueante: `build-and-test` → `lint` →
  `security-sast-sca` (SCA `trivy fs` + SAST `gosec`/`bandit`, quebra em
  CRITICAL) → `docker-build-scan-push` (`trivy image` antes do push) →
  `gitops-update`.
- Imagens Docker endurecidas: builder `golang:1.25-alpine` + runtime
  `alpine:3.21` mínimo (sem toolchain), usuário non-root, `-trimpath`/
  `-ldflags` para reduzir superfície de CVE.

**GitOps/CD**
- Pasta `fase-3/gitops/` separada, reaproveitando overlays Kustomize da Fase 2
  sem duplicar código.
- ArgoCD em modelo app-of-apps (`root-app.yaml` → 6 `Application`: 5 serviços +
  `platform`/ESO com `sync-wave: -1`), `syncPolicy.automated` com `prune`+
  `selfHeal`.
- `secrets.yaml` versionado substituído por External Secrets Operator +
  `ClusterSecretStore` lendo do AWS Secrets Manager.
- Como o Academy não permite IRSA, ESO/KEDA/pods de analytics-evaluation usam
  credenciais estáticas de sessão via `Secret` recriado a cada
  `terraform apply` (`null_resource` com `trigger=timestamp()`) — decisão
  pragmática e documentada como limitação do ambiente educacional.
- `ServerSideDiff` habilitado no ArgoCD para eliminar falso "OutOfSync" causado
  pelos defaults de schema que o webhook do ESO injeta.

---

## 2. Fora do escopo do desafio e fora das práticas atuais de mercado

Itens que a equipe implementou (ou aceitou como trade-off) que **não eram exigidos
pelo PDF do desafio** e que **também destoam do que o mercado faz hoje** em
produção. Em quase todos os casos, a causa é uma restrição concreta do ambiente
(AWS Academy) ou do prazo do desafio — não uma escolha de design livre.

| Item | Por que está fora do desafio | Por que está fora da prática de mercado atual |
|---|---|---|
| **Credenciais AWS estáticas de sessão para ESO/KEDA/pods** (`Secret` recriado a cada `apply`, expira em ~4h) | O desafio não menciona autenticação de workloads a serviços AWS a partir dos pods. | O mercado usa IRSA (IAM Roles for Service Accounts) ou EKS Pod Identity — nunca credenciais estáticas de longa duração dentro de `Secret`s do Kubernetes. **Motivo:** AWS Academy não permite criar IAM Roles/Policies, tornando IRSA literalmente inviável; foi o único caminho para autenticar o ESO/KEDA sem IAM próprio. |
| **Endpoint público do cluster EKS** | O desafio não pede rede privada nem VPN. | Times de plataforma maduros usam endpoint privado + acesso via VPN/bastion/runner self-hosted dentro da VPC. **Motivo:** o runner do GitHub Actions (hospedado pela GitHub) precisa falar com a API do cluster pela internet; não havia tempo/escopo para VPN ou runner self-hosted num desafio educacional. |
| **Lista extensa de skip-checks no Checkov** (Multi-AZ, deletion protection, PITR, flow logs, IAM DB auth, criptografia em trânsito do ElastiCache, rotação de secrets) | O desafio não exige hardening de compliance além do "scan bloqueante em CRÍTICO". | Nenhum desses controles seria desativado num ambiente produtivo real. **Motivo:** parte é bloqueio direto do Academy (KMS CMK); o restante é trade-off consciente de custo/velocidade para um ambiente efêmero de estudo (Multi-AZ dobra o custo de RDS, deletion protection atrapalha o destroy rápido que as sessões do Academy exigem). |
| **GitOps com push direto de bump na branch `lab`, sem PR/aprovação** | O desafio só pede "atualizar a tag da imagem no repositório de GitOps" — não especifica o mecanismo. | O mercado atual usa PRs automatizados para o bump (ex. Flux Image Automation, Renovate), auditáveis e com possibilidade de bloqueio antes do merge. **Motivo:** simplicidade para o prazo do desafio; a branch `lab` não tem proteção de branch configurada. |
| **Grande volume de branches de hotfix mescladas direto em `lab` sem revisão formal** (`fix/lab-*`) | Não solicitado pelo desafio. | Qualquer processo de engenharia maduro exige revisão de código antes do merge, mesmo em homologação. **Motivo:** iteração rápida sob pressão do tempo de sessão do AWS Academy (pods não subindo, credenciais expirando a cada ~4h) — corrigir e reaplicar rápido pesou mais que o processo formal. |
| **ElastiCache sem TLS/AUTH token em trânsito** | O desafio não exige criptografia de tráfego para o Redis. | Prática atual de mercado recomenda TLS + AUTH token sempre, mesmo em ambientes não produtivos. **Motivo:** o `evaluation-service` (código herdado da Fase 2) não implementa cliente Redis com TLS; corrigir isso seria mudança de aplicação, fora do escopo de infraestrutura da Fase 3. |

---

## 3. Além do escopo do desafio e dentro das melhores práticas de mercado

Itens que **não eram exigidos** pelo PDF, mas que a equipe decidiu implementar
porque refletem o que um time de plataforma maduro faria — oportunidades usadas
para elevar o projeto além do mínimo de portfólio acadêmico.

| Item | O que o desafio pedia | O que foi entregue a mais |
|---|---|---|
| **CI reutilizável por serviço** (`_reusable-ci-service.yml`) | "Crie workflows para cada um dos 5 microsserviços" — nada impede 5 pipelines duplicados. | Um único workflow parametrizável chamado pelos 5 `ci-<svc>.yml`, eliminando duplicação e centralizando manutenção (prática DRY padrão em times de CI/CD maduros). |
| **Testes automatizados nos 5 serviços** (`go test -race -cover` / `pytest --cov`) | "Rodar testes unitários **(se houver)**" — era condicional/opcional. | A equipe escreveu testes novos (`key_test.go`, `evaluator_test.go`, `test_app.py`) especificamente para ativar essa etapa e gerar cobertura real, em vez de deixar o job vazio. |
| **Ambiente `prod` completo como código** (Terraform + OIDC + IAM least-privilege), embora nunca aplicado | O desafio cobre apenas o essencial de um ambiente. | Multi-ambiente pronto desde já, com autenticação federada via OIDC (sem chaves estáticas de CI) e IAM least-privilege — demonstra maturidade de portfólio sem custo adicional (nada é aplicado). |
| **Blocos `validation {}` nos módulos Terraform** | Não exigido. | Validação de inputs críticos (CIDR, contagem de AZ, classe de instância, faixa de nodes) direto no módulo — prática de *shift-left* que evita aplicar infraestrutura malformada. |
| **Gate de IaC bloqueante** (Checkov + `trivy config`, `soft-fail: false`) | O desafio pede scan bloqueante só para dependências/código-fonte/imagem dos microsserviços. | A equipe estendeu o mesmo princípio de "vulnerabilidade crítica barra o pipeline" também para a camada de infraestrutura — shift-left security completo, cobrindo aplicação **e** IaC. |
| **External Secrets Operator + AWS Secrets Manager** | O desafio só cita o problema ("credenciais em arquivo de texto"), sem prescrever ferramenta. | Adoção do padrão atual de mercado para sincronizar segredos de um cofre gerenciado para o Kubernetes, em vez de Secrets estáticos aplicados manualmente ou via Terraform. |
| **`ServerSideDiff` no ArgoCD** | Não mencionado. | Ajuste de maturidade operacional (evita falso drift causado por webhooks de admissão) que a maioria dos tutoriais de GitOps não cobre. |
| **Estimativa de custo automática no PR** (Infracost) | O desafio pede só um "print da estimativa de custos" no relatório final (processo manual). | Comentário automático de custo estimado em cada PR de infraestrutura — governança de custo (FinOps) *shift-left*, antes do `apply`. |
| **Golden files / contract testing dos manifests renderizados** (`make render-diff`, `fase-3/gitops/.render/`) | Não solicitado. | Garante que a saída do Kustomize não sofre drift silencioso entre commits — rigor de engenharia acima do "aplicar e ver se funcionou". |
| **Lint bloqueante** (`golangci-lint` v2, `ruff` endurecido, sem `continue-on-error`) | "Rodar ferramentas de linting" — sem especificar se deve bloquear. | Lint vira gate real de qualidade, não apenas um relatório informativo. |
| **Imagens Docker mínimas e non-root** (multi-stage real, `alpine` puro, `-trimpath`/`-ldflags`) | "Construir a imagem Docker e escanear" — bastaria passar no scanner. | Hardening proativo da imagem (remoção do toolchain do runtime, usuário non-root) reduz superfície de ataque além do mínimo necessário para o `trivy image` passar. |

---

## 4. Débitos técnicos

- **Documentação incompleta:** o `README.md` chegou a ter "Dificuldades
  Encontradas: A confirmar com o grupo", `[link do vídeo]` e
  `[link do diagrama SVG]` como placeholders, e referenciava
  `.github/estimativa-custos-fase3.png`, que **não existe** no repositório (só
  há um diagrama da Fase 2 com nome diferente). *(a seção de dificuldades já
  foi corrigida — ver PR #87; os demais placeholders seguem pendentes)*.
- **Ambiente prod nunca validado:** todo o código de `envs/prod`/
  `bootstrap/prod` (OIDC, IAM least-privilege, HA) está pronto mas nunca rodou
  `terraform apply` de verdade — só passou por `terraform validate`.
- **Credenciais estáticas como stopgap:** a autenticação AWS de ESO/KEDA via
  sessão expira em ~4h e depende de refresh manual documentado no
  `PASSO-A-PASSO.md` — não é solução sustentável fora do Academy.
- **`use_lockfile` não adotado:** segue dependendo da tabela DynamoDB de lock;
  migração documentada como trabalho futuro.
- **Capacity planning por tentativa e erro:** réplicas dos 5 serviços oscilaram
  entre 1, 2 e 3 (branches `fix/lab-1-replica`, `fix/lab-2-replicas`,
  `fix/lab-reduce-pod-count`) sem um dimensionamento definitivo; o cluster já
  passou por pelo menos um destroy/rebuild completo.
- **Histórico fragmentado:** dezenas de branches `sync/lab-from-main-N` e
  `fix/lab-*` sem PR mesclado formalmente — dificulta auditoria.
- **`PLANO-DE-PRS.md` desatualizado:** previa 12 PRs pequenos e sequenciais; a
  execução real consolidou vários escopos num só PR (#57) e gerou mais de 25
  PRs adicionais de hotfix pós-deploy não previstos no plano.
- **Infracost incompleto:** o step de estimativa de custo no `infra-tf-plan`
  depende de `INFRACOST_API_KEY`; sem o secret, falha silenciosamente no fim
  do job.
- **Rename tardio** `flag` → `flags` (código, k8s e infra) na PR #84 — indica
  retrabalho de uma decisão de nomenclatura tomada tarde.

---

## 5. Dificuldades encontradas

- **Conta/região herdadas erradas da Fase 2:** os manifests e o Terraform
  vinham com `us-east-2`/`047719652987` (fase anterior); o Academy usa outra
  conta (`361075236043`) e não libera `us-east-2` no Learner Lab.
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

---

## 6. Cronograma para o vídeo de demonstração (até 20 min)

| Tempo | Bloco | O que mostrar |
|---|---|---|
| 0:00–1:30 | Abertura | Time, o problema da Fase 2 (deploy manual, credenciais em texto, CVE em produção, ambiente não reprodutível) e a resposta da Fase 3 (IaC + DevSecOps + GitOps). |
| 1:30–4:00 | IaC (Terraform) | Estrutura de módulos, LabRole via `data source`, backend remoto (S3 + lock DynamoDB). Rodar/mostrar `terraform plan` e os recursos já aplicados no console AWS: VPC, EKS, os 3 RDS, ElastiCache, DynamoDB, SQS, 5 repositórios ECR. |
| 4:00–6:00 | Pipeline de CI | Abrir `_reusable-ci-service.yml` e explicar os estágios (build-test → lint → security-sast-sca → docker-build-scan-push → gitops-update) e a composite action `aws-auth`. |
| 6:00–10:00 | DevSecOps na prática | Seguir o roteiro do `DEVSECOPS-DEMO.md`: criar a branch `demo/devsecops-vuln-block`, adicionar `PyYAML==5.3.1` (CVE crítico), abrir PR e mostrar o job `security-sast-sca` falhando no `trivy fs`. Corrigir a dependência e mostrar o pipeline passando. |
| 10:00–13:00 | GitOps | Mostrar o job `gitops-update` rodando `kustomize edit set image` e commitando o bump na branch `lab`; passar pela estrutura `fase-3/gitops/` (app-of-apps, kustomizations, ExternalSecret). |
| 13:00–16:00 | ArgoCD | Abrir a UI do ArgoCD, mostrar as 6 Applications (5 serviços + platform) e o sync automático detectando o commit de bump e aplicando no cluster sem `kubectl apply` manual. |
| 16:00–18:00 | Decisões e dificuldades | Resumo rápido dos pontos mais fortes para a nota: restrições de IAM do Academy, ausência de IRSA (credenciais estáticas), capacidade dos nós, CVEs corrigidas no pipeline. |
| 18:00–19:00 | Custos e arquitetura | Print da estimativa de custos (Cost Explorer/Pricing Calculator) e o diagrama de arquitetura. |
| 19:00–20:00 | Encerramento | Link da documentação e do repositório, integrantes do grupo. |

Isso cobre os 4 pontos que o PDF pede explicitamente no vídeo (IaC, Pipeline
DevSecOps falhando/corrigindo, GitOps atualizando a tag, ArgoCD sincronizando)
mais os tópicos extras exigidos na documentação separada por tópicos.
