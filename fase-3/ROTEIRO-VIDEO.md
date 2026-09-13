# Roteiro do Vídeo de Demonstração — Fase 3

Enredo completo para gravação (limite de 20 minutos), derivado da
[Análise Técnica da Fase 3](#) e do histórico real de commits/PRs.

Cada bloco traz **TELA** (o que está compartilhado), **FALA** (texto pronto para
narrar) e **AÇÕES** (o que clicar/executar). O tempo é orientativo: o que não
couber, corta-se dos blocos 7 e 8 — nunca dos blocos 4, 5 e 6, que são os que
provam os requisitos do desafio.

## Índice

- [0. Pré-produção (fazer antes de apertar REC)](#0-pré-produção-fazer-antes-de-apertar-rec)
- [1. Abertura — 0:00–1:30](#1-abertura--000130)
- [2. IaC com Terraform — 1:30–4:00](#2-iac-com-terraform--130400)
- [3. Pipeline de CI — 4:00–6:00](#3-pipeline-de-ci--400600)
- [4. DevSecOps na prática — 6:00–10:00](#4-devsecops-na-prática--6001000)
- [5. GitOps — 10:00–13:00](#5-gitops--10001300)
- [6. ArgoCD — 13:00–16:00](#6-argocd--13001600)
- [7. Decisões e dificuldades — 16:00–18:00](#7-decisões-e-dificuldades--16001800)
- [8. Custos e arquitetura — 18:00–19:00](#8-custos-e-arquitetura--18001900)
- [9. Encerramento — 19:00–20:00](#9-encerramento--19002000)
- [10. Mapa requisito → prova na tela](#10-mapa-requisito--prova-na-tela)
- [11. Planos B](#11-planos-b)

---

## 0. Pré-produção (fazer antes de apertar REC)

### Fatos do repositório que mudam a gravação

1. **Grave a partir da branch `lab`, não da `main`.** A `main` está em
   `#61`; a `lab` carrega o rename `flag` → `flags`, o `fase-3/Makefile`, os
   golden files em `fase-3/gitops/.render/` e os hotfixes de operação. Na `main`
   o workflow ainda é `ci-flag.yml` e o serviço ainda é `fase-2/flag-service`.
2. **A branch de demo do DevSecOps tem que sair da `lab`.** O
   `DEVSECOPS-DEMO.md` manda criar a branch a partir da `main`, mas o path que
   dispara o workflow é `fase-2/flags-service/**` — que só existe na `lab`.
   Saindo da `main`, o arquivo é `fase-2/flag-service/requirements.txt`, o
   filtro de path não casa e **o pipeline não roda**. Use:
   `git checkout lab && git checkout -b demo/devsecops-vuln-block`.
3. **Três placeholders ainda abertos no `fase-3/README.md`** (bloqueiam os
   blocos 8 e 9 se não forem resolvidos antes da gravação):
   - `## Dificuldades Encontradas` → ainda diz *"A confirmar com o grupo"*;
     o conteúdo real está na Análise Técnica, seção 5.
   - `.github/estimativa-custos-fase3.png` → **não existe** no repositório
     (o que existe é `.github/price-calculator.jpeg`, da Fase 2).
   - `.github/DiagramaArquiteturaFase3.png` e `[link do diagrama SVG]` →
     o que existe é `.github/toggle-master-arquitetura-distribuida.svg`
     (Fase 2).
   Se não der tempo de produzir o print e o diagrama novos, **diga na
   narração** que o diagrama exibido é o da Fase 2 mais a camada de
   plataforma da Fase 3 — assumir a limitação custa menos nota que exibir um
   link quebrado.
4. **`make render-diff` é um gate local, não um job de CI.** Quem regenera o
   baseline no pipeline é o step *"regenera .render (golden)"* dentro de
   `gitops-update`; o CI só bloqueia via `make -C fase-3 lint-registry`. Narre
   assim — dizer "gate de CI" seria impreciso e um avaliador atento checa.
5. **A sessão do AWS Academy expira em ~4h.** Renove as credenciais e rode
   `aws sts get-caller-identity` **imediatamente antes** de gravar. Se o
   ArgoCD estiver `Degraded` por credencial vencida, o bloco 6 morre.

### Checklist de ambiente (T-30 min)

- [ ] Sessão do AWS Academy nova; `AWS_ACCESS_KEY_ID`/`SECRET`/`SESSION_TOKEN`
      atualizados nos GitHub Secrets (o `aws-auth` da `lab` depende deles).
- [ ] `terraform -chdir=fase-3/infra/terraform/envs/lab init` já rodado
      (sem init ao vivo: o download de providers é tempo morto em vídeo).
- [ ] Cluster de pé e pods `Running`: `kubectl get pods -A`.
- [ ] ArgoCD: port-forward ou Ingress no ar, login feito, as 6 Applications
      `Synced/Healthy`.
- [ ] Um `terraform plan` já executado uma vez (para o cache quente) — no vídeo
      ele deve terminar em "No changes" ou num diff pequeno e explicável.
- [ ] Branch `demo/devsecops-vuln-block` **criada mas não pushada**, com o
      commit do `PyYAML==5.3.1` pronto para `git push` ao vivo.

### Higiene de tela (importante — é vídeo público)

- [ ] Fechar abas de secrets: AWS Secrets Manager com valores revelados,
      página de GitHub Secrets, `terraform.tfvars` com senha, output de
      `kubectl get secret -o yaml`.
- [ ] Não mostrar `aws configure list` nem o `~/.aws/credentials`.
- [ ] O `aws sts get-caller-identity` pode aparecer (mostra conta e role, não
      chave) — inclusive é bom: prova a conta `361075236043`.
- [ ] Terminal em fonte grande (16pt+), tema claro, largura ~120 colunas.
- [ ] Abas pré-abertas na ordem dos blocos: GitHub (repo, Actions, PR da demo),
      console AWS (VPC/EKS/RDS/ECR), ArgoCD, editor no `fase-3/`.

### Divisão entre os 5 integrantes (sugestão)

| Bloco | Quem narra |
|---|---|
| 1 Abertura + 9 Encerramento | Integrante A |
| 2 IaC | Integrante B |
| 3 CI + 4 DevSecOps | Integrante C |
| 5 GitOps + 6 ArgoCD | Integrante D |
| 7 Decisões/dificuldades + 8 Custos | Integrante E |

---

## 1. Abertura — 0:00–1:30

**TELA:** slide único com o nome do grupo, os 5 integrantes e duas colunas —
"Fase 2: como estava" / "Fase 3: como ficou".

**FALA:**

> Boa noite. Somos o grupo do Toggle Master — a plataforma de feature flags que
> construímos na Fase 2 com cinco microsserviços em Kubernetes.
>
> A Fase 2 funcionava, mas tinha quatro problemas que só aparecem quando você
> tenta operar de verdade. **Um:** a infraestrutura foi criada à mão no console
> da AWS — não era reprodutível; se a conta caísse, ninguém remontava o
> ambiente igual. **Dois:** as credenciais de banco viviam num `secrets.yaml`
> versionado em texto claro dentro do repositório. **Três:** o deploy era
> `kubectl apply` na máquina de quem estava com o terminal aberto — sem
> rastreabilidade de quem aplicou o quê. E **quatro:** a gente só descobria
> vulnerabilidade depois de a imagem já estar rodando.
>
> A Fase 3 é a resposta a esses quatro pontos, em três camadas: **IaC** com
> Terraform, **DevSecOps** no pipeline de CI e **GitOps** com ArgoCD. Nos
> próximos 18 minutos vamos mostrar cada uma delas rodando de verdade — e no
> meio do caminho vamos quebrar o pipeline de propósito, com uma CVE crítica,
> para provar que o gate de segurança realmente barra.

**AÇÕES:** nenhum comando; só o slide. Não gaste mais de 1:30 aqui.

---

## 2. IaC com Terraform — 1:30–4:00

**TELA:** editor em `fase-3/infra/terraform/`, depois console AWS.

**FALA:**

> Começando pela base. Todo o ambiente é Terraform, organizado em **nove
> módulos reutilizáveis** — `networking`, `eks`, `rds`, `elasticache`,
> `dynamodb`, `sqs`, `ecr`, `addons` e `iam_oidc_github`. Os mesmos módulos
> servem dois ambientes: `envs/lab`, que é o que está aplicado na AWS agora, e
> `envs/prod`, que está escrito, validado e versionado, mas nunca aplicado —
> por decisão de custo.
>
> *(abre `envs/lab/main.tf`)* Aqui no `lab` são oito chamadas de módulo. O
> `iam_oidc_github` não aparece porque é o único que vive só no `prod` e no
> `bootstrap` — e isso me leva à primeira decisão técnica interessante.
>
> O AWS Academy **não permite criar IAM Roles**. Então o módulo `eks` recebe
> `cluster_role_arn` e `node_role_arn` como variáveis **opcionais**: se vierem
> vazias, ele cai para o ARN da `LabRole`, que a gente lê com um `data source`.
> Isso deixa o mesmo módulo servir o `lab` — com a role emprestada do Academy —
> e o `prod`, com IAM próprio e least-privilege, sem nenhum `if` de ambiente
> espalhado pelo código.
>
> *(abre `envs/lab/backend.tf`)* O state é remoto: bucket S3 `tc-fiap-tfstate`
> com lock numa tabela DynamoDB. A gente não usou o `use_lockfile` nativo, que
> seria mais simples, porque o projeto está pinado no Terraform 1.9.8 e esse
> recurso exige 1.10 ou superior — está documentado como débito técnico.
>
> *(abre um módulo qualquer, ex. `modules/networking/variables.tf`)* Duas coisas
> que não eram exigidas pelo desafio e a gente colocou: blocos `validation`
> nos inputs críticos — CIDR, contagem de AZs, classe de instância, faixa de
> nodes — para o erro estourar no `plan` e não depois de meia infra aplicada; e
> um `.checkov.yaml` **bloqueante**, com `soft-fail: false`. Cada exceção na
> baseline tem comentário justificando: KMS com chave gerenciada pelo cliente é
> indisponível no Academy, Multi-AZ e deletion protection foram desligados por
> custo em ambiente efêmero.

**AÇÕES:**
1. `tree -L 2 fase-3/infra/terraform` (ou o painel do editor).
2. Abrir `envs/lab/main.tf` — rolar mostrando os `module "..."`.
3. Abrir o trecho do `data "aws_iam_role" "lab_role"` / variáveis opcionais do
   módulo `eks`.
4. `terraform -chdir=fase-3/infra/terraform/envs/lab plan` — **já inicializado**.
   Mostrar o final: "No changes. Your infrastructure matches the configuration."
5. Console AWS, em sequência rápida (15s cada): VPC, cluster EKS, os **3 RDS**,
   ElastiCache, tabela DynamoDB, fila SQS, os **5 repositórios ECR**.

---

## 3. Pipeline de CI — 4:00–6:00

**TELA:** `.github/workflows/` e o grafo de um run no GitHub Actions.

**FALA:**

> O desafio pede um workflow para cada um dos cinco microsserviços. A gente não
> escreveu cinco pipelines: escreveu **um** — o `_reusable-ci-service.yml` —
> parametrizado por `service_name`, `service_language` e `working_directory`, e
> cinco workflows finos que só o chamam com os parâmetros certos. Mudança de
> pipeline acontece num arquivo, não em cinco.
>
> *(abre `_reusable-ci-service.yml`)* São cinco estágios, **sequenciais e
> bloqueantes** — cada um com `needs` no anterior:
>
> **`build-and-test`** — `go build` e `go test -race -cover` nos serviços Go,
> `pytest --cov` nos Python. Vale dizer: o desafio pedia testes unitários
> *"se houver"* — era condicional. A gente escreveu testes novos, como
> `key_test.go` e `evaluator_test.go`, justamente para essa etapa não ser um
> job vazio.
>
> **`lint`** — `golangci-lint` v2 e `ruff` endurecido, **sem
> `continue-on-error`**. Lint aqui é gate, não relatório.
>
> **`security-sast-sca`** — `trivy fs` para SCA de dependências, `gosec` para
> o código Go e `bandit` para o Python. É o estágio que vamos quebrar de
> propósito no próximo bloco.
>
> **`docker-build-scan-push`** — e a ordem importa: o `trivy image` roda
> **antes** do push. Imagem vulnerável não chega no ECR.
>
> **`gitops-update`** — o elo com o GitOps, que é o bloco depois.
>
> *(abre `.github/actions/aws-auth/action.yml`)* Um detalhe de arquitetura que
> vale explicar: a autenticação na AWS é uma **composite action** que troca de
> estratégia por branch. Na `lab`, chaves estáticas de sessão do Academy; na
> `main`, OIDC, sem credencial de longa duração. Ela é composite, e não um
> reusable workflow, por um motivo concreto: credencial não se propaga entre
> jobs — os steps de credencial precisam rodar no mesmo job que vai usá-la. E
> repare no último step: `aws sts get-caller-identity` jogado no
> `GITHUB_STEP_SUMMARY`, para ficar auditável em qual conta o run entrou.

**AÇÕES:**
1. `ls .github/workflows/` — mostrar 1 reusable + 5 finos + 3 de infra.
2. Abrir `ci-flags.yml` (tem ~15 linhas) e depois o reusable.
3. GitHub → Actions → um run verde recente → mostrar o **grafo** dos 5 jobs
   encadeados.
4. Abrir o Step Summary de um run mostrando o `get-caller-identity`.

---

## 4. DevSecOps na prática — 6:00–10:00

Bloco mais importante do vídeo: é o requisito explícito
*"adicione uma dependência com vulnerabilidade conhecida e mostre o pipeline
falhando"*. Reserve os 4 minutos inteiros.

**TELA:** terminal + PR no GitHub.

**FALA (parte 1 — quebrando):**

> Agora a prova. Vou introduzir uma vulnerabilidade real, de propósito.
>
> *(no terminal)* Branch de demonstração saindo da `lab` — que é a branch de
> homologação — e no `requirements.txt` do `flags-service` eu adiciono
> `PyYAML==5.3.1`. Essa versão tem a **CVE-2020-14343**, severidade
> **CRITICAL**: execução remota de código via `yaml.full_load`, corrigida na
> 5.4. Escolhi de propósito uma CVE que tem correção disponível — é isso que o
> `trivy` classifica como `fixed` e o que um gate deve barrar.
>
> Push, e abro o PR para a `lab`. O filtro de path casa
> `fase-2/flags-service/**`, então dispara o `ci-flags.yml`, que chama o
> reusable.
>
> *(no PR, acompanhando os checks)* `build-and-test`: **passa** — a
> vulnerabilidade não quebra o código, e é exatamente por isso que teste
> unitário não substitui scan de segurança. `lint`: **passa** — a dependência
> está sintaticamente perfeita.
>
> E aí o `security-sast-sca` fica **vermelho**, no step `trivy fs`. *(abre o
> log)* Está tudo aqui: biblioteca `PyYAML`, `CVE-2020-14343`, severidade
> `CRITICAL`, status `fixed`, instalada `5.3.1`, corrigida em `5.4`. O step roda
> com `severity: CRITICAL` e `exit-code: 1` — vulnerabilidade crítica encerra o
> job com erro.
>
> E agora o efeito em cascata, que é o ponto todo: o `docker-build-scan-push`
> tem `needs: security-sast-sca`, então ele fica **skipped** — nenhuma imagem é
> construída. O `gitops-update` também: **skipped**. Resultado: nada vai para o
> ECR, nenhum bump de tag chega na `lab`, o ArgoCD não sincroniza nada. A
> vulnerabilidade morre no pipeline, a 3 estágios de distância do cluster.

**FALA (parte 2 — curando):**

> Para fechar o ciclo, o inverso. Troco para `PyYAML==5.4.1`, commit, push.
> *(mostra o novo run)* `trivy fs` volta a passar, e os estágios seguintes
> destravam: build, scan da imagem, push no ECR. O gate não é um bloqueio
> permanente — ele é um portão que abre quando a dependência está saudável.
>
> Essa branch de demonstração é descartada; ela nunca é mergeada.
>
> E o mesmo princípio está em cinco camadas, não só nessa: *(abre a tabela do
> `DEVSECOPS-DEMO.md`)* SCA de dependências com `trivy fs`; SAST de Go com
> `gosec`; SAST de Python com `bandit`; imagem de container com `trivy image`
> antes do push; e — isso o desafio **não** pedia — a camada de
> infraestrutura, com `checkov` em `soft-fail: false` e `trivy config` nos
> workflows de plan e apply. Estendemos o mesmo critério de "crítico barra o
> pipeline" para o IaC.

**AÇÕES:**
```bash
git checkout lab && git pull
git checkout -b demo/devsecops-vuln-block
# edita fase-2/flags-service/requirements.txt: + PyYAML==5.3.1
git commit -am "demo: dependencia com CVE critico (nao mergear)"
git push -u origin demo/devsecops-vuln-block
# abre PR -> lab
```
Depois: `sed -i 's/PyYAML==5.3.1/PyYAML==5.4.1/' ...`, commit, push.

> **Dica de gravação:** o run leva alguns minutos. Grave o push, **pause**,
> e retome com o resultado já pronto — ou deixe um run antigo já falhado numa
> aba para cortar a espera. Nunca fique 3 minutos olhando spinner.

---

## 5. GitOps — 10:00–13:00

**TELA:** job `gitops-update` no Actions + `fase-3/gitops/` no editor.

**FALA:**

> Com a imagem no ECR, quem faz o deploy? Ninguém digita `kubectl`. O último
> estágio do CI é o `gitops-update`.
>
> *(abre o job)* Ele só roda em `push` na branch `lab` — nunca em PR. Faz
> checkout da `lab`, instala o `kustomize` e roda `kustomize edit set image`,
> reescrevendo a tag da imagem no `kustomization.yaml` do serviço para o SHA do
> commit. Repare que ele casa a imagem **antiga**, da Fase 2 — outro registry,
> outra conta — e reaponta para o ECR desta conta: foi assim que a gente
> reaproveitou os overlays da Fase 2 sem duplicar manifesto.
>
> Depois ele commita e dá push direto na `lab`, com `[skip ci]` na mensagem
> para não entrar em loop, e um `concurrency group` chamado `gitops-bump-lab`
> para os cinco serviços não brigarem pelo mesmo branch ao mesmo tempo.
>
> *(abre `fase-3/gitops/.render/`)* E tem um cuidado extra aqui que vale
> mostrar. A gente mantém **golden files**: a saída renderizada do Kustomize,
> versionada. `make render-diff`, localmente, acusa qualquer drift silencioso
> entre commits. Como o bump muda o `kustomization.yaml`, o próprio job
> regenera esse baseline no mesmo commit — foi um bug que a gente teve e
> corrigiu. E o CI ainda roda `make lint-registry`, que valida que toda imagem
> no render vem do registry autorizado ou de uma allowlist explícita.
>
> Uma honestidade sobre esse desenho: o bump vai **direto** na `lab`, sem PR.
> O mercado hoje faria isso por PR automatizado — Flux Image Automation,
> Renovate — auditável e bloqueável antes do merge. A gente foi de push direto
> por prazo, e a `lab` não tem branch protection. Está registrado como
> trade-off consciente, não como descuido.

**AÇÕES:**
1. Actions → run recente na `lab` → job `gitops-update` → steps
   *"bump da imagem"*, *"regenera .render"*, *"validar registry"*,
   *"commit + push (lab)"*.
2. Commits da `lab` → mostrar os commits `cd(<svc>): bump -> <sha> [skip ci]`
   assinados pelo `github-actions[bot]`.
3. `git diff` de um desses commits: `kustomization.yaml` + `.render/`.

---

## 6. ArgoCD — 13:00–16:00

**TELA:** UI do ArgoCD.

**FALA:**

> E aqui o ciclo fecha. *(UI do ArgoCD)* O modelo é **app-of-apps**: um
> `root-app.yaml` que cria seis `Application` — os cinco microsserviços mais
> uma chamada `platform`. Todas com `syncPolicy.automated`, com `prune` e
> `selfHeal` ligados: se alguém mexer no cluster à mão, o ArgoCD desfaz.
>
> A `platform` tem `sync-wave: -1`, ou seja, sincroniza **antes** dos serviços.
> É ela que instala o **External Secrets Operator** e o `ClusterSecretStore`. E
> essa é a resposta ao `secrets.yaml` em texto claro da Fase 2: o segredo mora
> no AWS Secrets Manager, o ESO o sincroniza para dentro do cluster, e o
> repositório versiona só a *referência* — *(abre `externalsecret.yaml`)* aqui,
> um `ExternalSecret` que aponta para a chave `tc-rds-flag-credentials`. Nenhuma
> senha no Git.
>
> Dois problemas reais de operação que resolvemos nesse arquivo, e que valem
> mais que qualquer tutorial:
>
> **Primeiro, um deadlock de sincronização.** O Job de migration roda como hook
> `PreSync` e consome o Secret via `envFrom`. Mas o `ExternalSecret`, como
> recurso normal, só seria aplicado na fase `Sync` — depois. O pod do Job
> travava em `CreateContainerConfigError`, "secret not found", e isso bloqueava
> a fase `PreSync` inteira: a Application não saía do lugar. A correção foi
> tornar o `ExternalSecret` também um hook `PreSync`, na wave `-2`, uma antes
> do Job — e com `deletionPolicy: Retain`, porque o hook é recriado a cada sync
> e o Deployment depende do Secret em runtime.
>
> **Segundo, um falso `OutOfSync` permanente.** O webhook de admissão do ESO
> injeta defaults de schema no recurso, o diff do ArgoCD via o cluster
> diferente do Git, e a Application ficava eternamente `OutOfSync` sem nada
> estar errado. Resolvemos habilitando `ServerSideDiff`, que compara via API
> server e considera esses defaults.
>
> Agora o deploy ponta a ponta: *(clica Refresh)* o ArgoCD detecta o commit de
> bump que o CI acabou de empurrar na `lab`, entra em `OutOfSync`, sincroniza e
> volta para `Synced/Healthy`. *(abre a árvore de recursos e o novo
> ReplicaSet)* Rollout feito. Ninguém rodou `kubectl apply`. O Git é a única
> fonte de verdade, e o cluster converge para ele.

**AÇÕES:**
1. Tela das 6 Applications, todas `Synced/Healthy`.
2. Abrir `platform` → mostrar ESO e o `ClusterSecretStore`.
3. Abrir `fase-3/gitops/manifests/flags/externalsecret.yaml` no editor — ler os
   comentários das annotations `hook: PreSync` / `sync-wave: -2`.
4. Voltar ao ArgoCD, `Refresh` no `flags-service`, mostrar a transição
   `OutOfSync` → `Synced` e o novo pod subindo.

---

## 7. Decisões e dificuldades — 16:00–18:00

**TELA:** slide com 4 bullets grandes (não leia a Análise Técnica inteira).

**FALA:**

> Dois minutos sobre o que doeu — que é onde a gente aprendeu mais.
>
> **Restrições de IAM do Academy.** Não dá para criar Role nem Policy. Isso
> matou a prática padrão de mercado para pods acessarem serviços AWS, que é
> IRSA ou EKS Pod Identity. O ESO, o `TriggerAuthentication` do KEDA e os
> workers de SQS precisaram de credenciais estáticas de sessão, num `Secret`
> recriado a cada `terraform apply` via `null_resource` com trigger de
> timestamp. Expira em ~4 horas. **Não é o que faríamos em produção** — está
> documentado como débito e como limitação do ambiente educacional, não como
> escolha de design.
>
> **Um Access Entry errado que quase custou o cluster.** A `LabRole` estava
> registrada como `STANDARD` em vez de `EC2_LINUX`. O sintoma era
> `tls: internal error` no `metrics-server` e em qualquer
> `kubectl top`, `logs` ou `exec` — causa raiz nada óbvia de rastrear a partir
> daquela mensagem.
>
> **Capacidade dos nós.** Dois `t3.medium` suportam cerca de 34 pods. Cinco
> serviços vezes três réplicas, mais os mínimos dos HPAs, dava
> `FailedScheduling`. Reduzimos réplicas e removemos o HPA do `analytics`, que
> conflitava com o KEDA. E os probes de readiness ainda falhavam com "context
> deadline exceeded" porque 40 milicores não bastavam para o boot do gunicorn
> mais o pool de conexões mais o primeiro TLS contra um RDS `t3.micro` frio.
>
> **E uma CVE que o pipeline pegou de verdade** — não a plantada, uma real: a
> **CVE-2025-68121**, herdada da imagem base do Go. O runtime carregava o
> toolchain Go inteiro, com a stdlib 1.21 vulnerável. A correção foi
> multi-stage de verdade: builder `golang:1.25-alpine`, runtime `alpine:3.21`
> puro, usuário non-root, `-trimpath` e `-ldflags`. O toolchain não vai para a
> imagem final.
>
> Vale registrar: quase tudo isso só apareceu **depois** de o ambiente estar
> provisionado e sendo operado — não no `terraform validate`, não no CI. A
> gente passou por pelo menos um ciclo completo de destroy e reconstrução da
> infraestrutura para controlar custo e tempo de sessão.

**AÇÕES:** só slide. Se estiver estourando o tempo, corte a CVE-2025-68121
(ela já foi citada no bloco 4) e mantenha IAM/Academy + capacidade dos nós.

---

## 8. Custos e arquitetura — 18:00–19:00

**TELA:** print da estimativa de custos + diagrama.

**FALA:**

> Custos. *(mostra o print)* Esta é a estimativa do ambiente de `lab`: EKS,
> três instâncias RDS `t3.micro`, ElastiCache, DynamoDB, SQS e os
> repositórios ECR. E aqui também há uma decisão consciente: Multi-AZ e
> deletion protection estão **desligados**, e eles aparecem como exceções
> justificadas no `.checkov.yaml`. Multi-AZ dobraria o custo de RDS, e deletion
> protection atrapalharia o destroy rápido que as sessões do Academy exigem.
> Em produção, nenhum dos dois estaria desligado — e o `envs/prod`, que está
> versionado, já os tem ligados.
>
> Uma coisa a mais que fizemos e o desafio não pedia: o desafio pede um print
> de custo no relatório, um processo manual. A gente colocou **Infracost** no
> workflow de plan, comentando a estimativa automaticamente em cada PR de
> infraestrutura — governança de custo antes do `apply`, não depois da fatura.
>
> *(mostra o diagrama)* E a arquitetura final: VPC com subnets públicas e
> privadas, EKS com os cinco serviços, três RDS Postgres, ElastiCache, DynamoDB
> e SQS; o pipeline no GitHub Actions publicando no ECR e escrevendo na branch
> `lab`; e o ArgoCD dentro do cluster puxando dessa branch.

**AÇÕES:**
- Se o print novo não existir: mostre o **AWS Cost Explorer ao vivo** ou o
  Pricing Calculator, e diga que o print está no README. Não abra o README numa
  imagem quebrada.
- Se o diagrama da Fase 3 não existir: use
  `.github/toggle-master-arquitetura-distribuida.svg` e narre a diferença.

---

## 9. Encerramento — 19:00–20:00

**TELA:** slide final com a URL do repositório, o caminho `fase-3/` e os 5
integrantes.

**FALA:**

> Fechando: saímos de uma infraestrutura criada à mão, com segredo em texto
> claro e deploy manual, para um ambiente **inteiramente descrito em código**,
> com segurança bloqueando o pipeline em quatro camadas de aplicação e uma de
> infraestrutura, e deploy que acontece porque o Git mudou — não porque alguém
> rodou um comando.
>
> Toda a documentação está em `fase-3/`: o README com o passo a passo, o
> `DEVSECOPS-DEMO.md` com a demonstração que vocês acabaram de ver reproduzida
> passo a passo, e a Análise Técnica com as decisões, os débitos e as
> dificuldades — inclusive as que não conseguimos resolver dentro das
> restrições do AWS Academy.
>
> Grupo: [nomes]. Obrigado.

---

## 10. Mapa requisito → prova na tela

Use como conferência final antes de publicar: todo requisito do PDF tem um
momento do vídeo em que ele aparece na tela.

| Requisito do desafio | Onde aparece | Prova visual |
|---|---|---|
| Infraestrutura como código | Bloco 2 | `plan` limpo + recursos no console AWS |
| Estado remoto do Terraform | Bloco 2 | `backend.tf` + bucket S3 + tabela de lock |
| Workflow de CI por microsserviço | Bloco 3 | 5 `ci-*.yml` + grafo de jobs |
| Testes unitários (se houver) | Bloco 3 | job `build-and-test` com cobertura |
| Linting | Bloco 3 | job `lint` verde, sem `continue-on-error` |
| SAST + SCA | Blocos 3 e 4 | `gosec`, `bandit`, `trivy fs` |
| **Dependência vulnerável barrando o pipeline** | **Bloco 4** | run vermelho + jobs `skipped` |
| Build e scan de imagem | Bloco 4 | `trivy image` antes do push no ECR |
| Push no ECR | Blocos 2 e 4 | 5 repositórios com tags de SHA |
| Atualização da tag no repo de GitOps | Bloco 5 | commit `cd(<svc>): bump -> <sha>` |
| CD sem `kubectl apply` | Bloco 6 | ArgoCD sincronizando sozinho |
| Segredos fora do Git | Bloco 6 | ESO + `ExternalSecret` + Secrets Manager |
| Estimativa de custos | Bloco 8 | print / Cost Explorer |
| Diagrama de arquitetura | Bloco 8 | diagrama |
| Dificuldades encontradas | Bloco 7 | slide + seção 5 da Análise Técnica |

---

## 11. Planos B

| Se acontecer | Faça |
|---|---|
| Credencial do Academy expirou no meio da gravação | Pare, renove, regrave só o bloco afetado. Não tente contornar ao vivo. |
| `terraform plan` mostra diff inesperado | Narre o diff como drift real e explique a causa — é mais convincente que cortar. **Não** rode `apply` no vídeo. |
| O run do CI demora demais | Corte na edição: mostre o push, depois o resultado. Tenha um run antigo já falhado numa aba. |
| Cluster cheio / pods em `FailedScheduling` | É o problema real da seção 5 — narre como dificuldade encontrada e mostre `kubectl describe` do evento. Vira conteúdo. |
| ArgoCD `Degraded` | Provável credencial do ESO vencida. `terraform apply` recria o `Secret`; se não der tempo, use uma gravação prévia do sync e diga que é replay. |
| Estourou 20 minutos | Corte na ordem: bloco 8 → bloco 7 → segunda metade do bloco 2. Blocos 4, 5 e 6 são intocáveis. |
| O `trivy` não acusa mais a CVE (base de dados mudou) | Confirme antes de gravar com `trivy fs --severity CRITICAL fase-2/flags-service/`. Se mudou, troque a dependência plantada e atualize o `DEVSECOPS-DEMO.md`. |
