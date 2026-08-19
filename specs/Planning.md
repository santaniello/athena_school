# Athena — Planning de Implementação

> Este documento é o guia de implementação do Athena. Ele define as fases de desenvolvimento, dependências e critérios de conclusão de cada etapa.
>
> **Referência de produto:** `specs/Athena.md`
>
> **Specs detalhadas por fase:** `specs/phases/`

---

## Direção do Produto

| Decisão | Valor |
|---|---|
| Interface | Desktop apenas (Wails + React + TypeScript) |
| Backend/Core | Go |
| LLM Gateway | OpenRouter (único gateway inicial) |
| Persistência local | SQLite |
| Vector search | Local (inicial) |
| Distribuição inicial | Windows + Linux |
| Autenticação | Conta própria (e-mail + senha) |
| Modelo de negócio | Trial 7 dias → Planos Essencial / Pro / Expert |

---

## Stack Técnico

```text
Frontend         React + TypeScript
Desktop bridge   Wails v2
Core             Go (Clean/Hexagonal)
Banco local      SQLite (modernc.org/sqlite — pure Go, sem CGO)
Vector store     Local (fase inicial)
LLM              OpenRouter API
Auth backend     Go (HTTP API mínima — auth + licenças)
CI/CD            GitHub Actions
Pagamentos       Paddle
```

---

## Princípios de Implementação

- **Incremental** — cada fase entrega software funcionando e utilizável
- **Core-first** — regras de negócio nunca ficam no frontend
- **Knowledge-first** — buscar conhecimento local antes de chamar o LLM
- **Local-first** — dados do usuário ficam no dispositivo; servidor só gerencia auth e licença
- **Simple design** — sem abstrações prematuras; resolver o problema atual
- **TDD** — testes antes da implementação; cobertura mínima 80%

---

## Visão Geral das Fases

| Fase | Nome | Objetivo | Dependências |
|---|---|---|---|
| 0 | Foundation | Repositório, tooling, CI/CD | — |
| 1 | Desktop MVP | App funcional com login, onboarding e estudo | Fase 0 |
| 2 | Knowledge Engine | Knowledge Base + RAG + notas | Fase 1 |
| 3 | Learning Intelligence | Challenge + Gap Detection + Flashcards | Fase 2 |
| 4 | Interview Mode | Simulação de entrevista completa | Fase 3 |
| 5 | Comercialização | Planos, pagamento, macOS, feature gating | Fase 1 |
| 6 | Voice Interview | Entrevista por áudio (STT + TTS) | Fase 4 |
| 7 | Advanced Features | Whiteboard, Knowledge Graph, Algorithm Mode | Fase 4 |

---

## Fase 0 — Foundation

**Objetivo:** Repositório pronto para desenvolvimento. Zero funcionalidade, máximo de tooling.

### Tarefas

#### Repositório e estrutura
- [ ] `git init` + `.gitignore` (Go, Node, Wails, OS files)
- [ ] `CLAUDE.md` com regras de desenvolvimento (TDD, commits, padrões Go)
- [ ] `go.mod` com nome do módulo `github.com/<user>/athena`
- [ ] Estrutura de diretórios inicial:

```text
athena/
├── main.go                  # Wails entrypoint (Wails v2 exige main.go na raiz)
├── wails.json                # Config do projeto Wails
├── internal/
│   ├── domain/                # regras de negócio puras
│   ├── application/           # casos de uso
│   ├── infrastructure/        # SQLite, OpenRouter, filesystem
│   └── interfaces/
│       └── desktop/           # Wails bindings (App struct)
├── frontend/                 # React + TypeScript (gerado pelo Wails)
├── build/                    # Assets de empacotamento do Wails (ícones, manifests)
├── go.mod
├── go.sum
└── Makefile
```

> `cmd/athena/` foi descartado: o Wails v2 não suporta main package fora da raiz do projeto (a diretiva `//go:embed all:frontend/dist` também exige isso), então o entrypoint mora em `main.go` na raiz. Os bindings de verdade continuam em `internal/interfaces/desktop/`.

#### Wails
- [ ] `wails init` com template React + TypeScript
- [ ] Build funcionando: `wails build`
- [ ] Dev mode funcionando: `wails dev`

#### Quality gates (pre-commit hook)
- [ ] `go test ./...`
- [ ] Cobertura mínima 80%
- [ ] `golangci-lint run`
- [ ] `govulncheck ./...`
- [ ] `make install-hooks`

#### CI/CD (GitHub Actions)
- [ ] Workflow `ci.yml`: roda testes e lint em cada PR
- [ ] Workflow `release.yml`: build matrix [windows, linux] por tag `v*`
- [ ] Artifacts publicados no GitHub Releases automaticamente

#### Makefile
```makefile
build:         wails build
dev:           wails dev
test:          go test ./...
lint:          golangci-lint run
install-hooks: git config core.hooksPath .githooks && chmod +x .githooks/pre-commit
```

### Done when
- `wails dev` abre janela desktop em branco sem erros
- `go test ./...` passa
- Push de tag `v0.0.1` gera binários Windows e Linux no GitHub Releases

---

## Fase 1 — Desktop MVP

**Objetivo:** Produto mínimo enviável. Usuário cria uma conta local, conecta sua chave OpenRouter, faz onboarding e estuda. Sem planos pagos ou trial nesta fase — comercialização fica para a Fase 5, quando for retomada.

Esta fase combina o que o spec chama de "Fase 1 — Core MVP" e "Fase 2 — Desktop MVP" porque não faz sentido entregar um Core sem interface.

### 1.1 — Núcleo de Auth Local

Sem servidor remoto nesta fase. Conta criada e validada 100% localmente, desenhado como porta hexagonal para permitir trocar por um backend remoto no futuro sem reescrever os casos de uso.

**Entidades:**
```go
type Account struct {
    ID           string
    Email        string
    PasswordHash string // bcrypt
    CreatedAt    time.Time
}
```

**Porta (`internal/domain/auth/`):**
```go
type AccountRepository interface {
    Create(ctx context.Context, account Account) error
    FindByEmail(ctx context.Context, email string) (Account, error)
    UpdatePassword(ctx context.Context, id string, passwordHash string) error
    Delete(ctx context.Context, id string) error
}
```

Hoje a única implementação é local (`internal/infrastructure/sqlite`, tabela `accounts`). Uma implementação remota futura satisfaz a mesma interface sem alterar `internal/application/auth/`.

**Tarefas:**
- [x] `internal/domain/auth/` — `Account`, `AccountRepository`
- [x] `internal/application/auth/` — casos de uso `Register`, `Login`, `ResetLocalAccount`
- [x] `internal/infrastructure/sqlite/` — implementação local do `AccountRepository`
- [x] Sem JWT, sem SMTP, sem servidor HTTP — tudo roda no processo local

### 1.2 — Tela de Login e Criação de Conta (Desktop)

**Fluxo:**
```text
App abre → sem sessão local → tela de login
Login/registro bem-sucedido → sessão salva localmente → gate de chave OpenRouter (1.4) → próxima tela
```

**UI (React):**
- [ ] Tela de login (e-mail + senha + botão entrar)
- [ ] Tela de criação de conta (e-mail + senha + confirmar)
- [ ] Tela de reset de conta local (substitui "recuperação de senha" — sem e-mail/servidor, é um reset destrutivo, não recuperação real)
- [ ] Wails binding: `Login(email, password)`, `Register(email, password)`, `ResetLocalAccount(email)`

**Go (Core):**
- [ ] `internal/application/auth/` — casos de uso de autenticação (ver 1.1)
- [ ] Sessão armazenada localmente em `~/.athena/session.json`, sem expiração atrelada a servidor

### 1.4 — Onboarding Interview

Disparado após primeiro login bem-sucedido (sem perfil local).

**Fluxo:**
```text
Primeiro login → sem UserProfile local → tela de onboarding
Sem openrouter_key em ~/.athena/config.yaml → tela obrigatória "Conecte sua chave OpenRouter"
  (input mascarado, validado por chamada de teste — mesma validação da seção 1.8) → chave salva
LLM conduz entrevista conversacional (3–5 perguntas)
Usuário responde → perfil gerado → confirmação → salvo localmente
```

A entrevista depende de uma chamada LLM (ver 1.5), por isso não pode começar sem uma chave OpenRouter válida. O gate só aparece uma vez; a tela de configurações (1.8) permite trocar a chave depois.

**Perguntas coletadas:**
- Nome
- Área de atuação / o que estuda
- Foco específico
- Nível de experiência (beginner / intermediate / advanced)
- Objetivo principal
- Estilo de estudo preferido
- Como quer chamar o assistente

**Tarefas:**
- [ ] `internal/domain/profile/` — `UserProfile` struct
- [ ] `internal/application/onboarding/` — lógica de condução da entrevista
- [ ] `UserProfile` salvo em `~/.athena/profile.json`
- [ ] UI: gate "Conecte sua chave OpenRouter", exibido antes da entrevista quando `openrouter_key` está ausente
- [ ] UI: chat conversacional simples (não formulário)
- [ ] Tela de confirmação do perfil com opção de editar

```go
type UserProfile struct {
    Name            string   `json:"name"`
    AssistantName   string   `json:"assistant_name"`
    Area            string   `json:"area"`
    Specialty       string   `json:"specialty"`
    ExperienceLevel string   `json:"experience_level"` // beginner | intermediate | advanced
    Goals           []string `json:"goals"`
    StudyStyle      string   `json:"study_style"`
    CreatedAt       time.Time `json:"created_at"`
}
```

### 1.5 — LLM Service (OpenRouter)

- [ ] `internal/infrastructure/openrouter/` — implementação do `LLMProvider`
- [ ] Interface:

```go
type LLMProvider interface {
    Chat(ctx context.Context, req ChatRequest) (ChatResponse, error)
    ChatStream(ctx context.Context, req ChatRequest, handler func(chunk string) error) error
    Embeddings(ctx context.Context, req EmbeddingRequest) (EmbeddingResponse, error)
}
```

- [ ] Streaming via SSE (Server-Sent Events do OpenRouter)
- [ ] Model router: task → tier (cheap / medium / premium)
- [ ] Budget tracker: registra tokens e custo por sessão
- [ ] Configuração da chave OpenRouter: `~/.athena/config.yaml`

### 1.6 — Study Mode (Desktop)

**Tarefas:**
- [x] `internal/domain/study/` — `Session`, `Message`, regras de sessão
- [x] `internal/application/study/` — `Service.Start()`/`SendMessage()`/`End()`
- [x] Personalization: `UserProfile` injetado em todo prompt
- [x] UI: chat interface com streaming de resposta
- [x] Tópico selecionável via interface (sem CLI)
- [x] Perguntas geradas pelo LLM, resposta do usuário, feedback

**Prompts base:**

`{Specialty}` foi removido: `UserProfile` não tem esse campo (removido em 1.4).

```text
System: Você é {AssistantName}, assistente de aprendizado de {Name}.
        Área: {Area}. Nível: {ExperienceLevel}.
        Estilo: {StudyStyle}. Objetivo: {Goals}.
        Adapte todas as explicações ao contexto do usuário.
```

### 1.7 — Persistência Local (SQLite)

- [ ] `internal/infrastructure/sqlite/` — repositórios
- [ ] Schema inicial:

```sql
CREATE TABLE accounts (
    id TEXT PRIMARY KEY,
    email TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    created_at DATETIME
);

CREATE TABLE sessions (
    id TEXT PRIMARY KEY,
    topic TEXT,
    mode TEXT, -- study | challenge | interview
    started_at DATETIME,
    ended_at DATETIME
);

CREATE TABLE messages (
    id TEXT PRIMARY KEY,
    session_id TEXT REFERENCES sessions(id),
    role TEXT, -- user | assistant
    content TEXT,
    created_at DATETIME
);

CREATE TABLE usage (
    id TEXT PRIMARY KEY,
    session_id TEXT REFERENCES sessions(id),
    model TEXT,
    input_tokens INTEGER,
    output_tokens INTEGER,
    cost REAL,
    created_at DATETIME
);
```

### 1.8 — Configurações

- [ ] Tela de configurações no desktop
- [ ] Campos: chave OpenRouter, nome do assistente, área/foco, nível
- [ ] Salvo em `~/.athena/config.yaml`

### 1.9 — Auto-Update

- [ ] App verifica GitHub Releases API na inicialização
- [ ] Notificação silenciosa: "Nova versão disponível"
- [ ] Download + instalação com confirmação do usuário

### Done when (Fase 1)

- Usuário instala no Windows ou Linux
- Cria uma conta local e conecta sua chave OpenRouter
- Passa pelo onboarding conversacional
- Abre a tela principal e faz uma sessão de estudo completa
- Resposta chega em streaming com personalização baseada no perfil

---

## Fase 2 — Knowledge Engine

**Objetivo:** A Knowledge Base pessoal começa a existir. O usuário importa notas e o Athena constrói conhecimento a partir das sessões.

**Dependência:** Fase 1 completa.

**Entrega em três incrementos**, cada um demonstrável sozinho:

1. **2.1 → 2.2 → 2.3 → 2.7** — modelo, extração, import de notas + explorer, fila de revisão
2. **2.4 → 2.5 → 2.8** — vector search, RAG, indexação de todos os estados
3. **2.9 → 2.10 → 2.11 → 2.12** — proveniência, duplicidade, reconciliação e histórico de revisões

> A antiga 2.6 (Knowledge Explorer) foi absorvida pela 2.3: toda nota importada
> também vira um `knowledge.Item` "sombra" (sem LLM), então a tela que
> lista/gerencia Items precisa existir para a importação ser utilizável. Isso
> quebra a independência original das duas trilhas — a 2.3 agora depende da 2.1
> (o modelo `Item`) — mas evita que o Explorer mantenha duas listagens
> paralelas. Ver a spec da fase para a justificativa completa.

> As specs detalhadas estão em `specs/phases/phase-02-knowledge-engine/`. Esta seção é o resumo; em caso de divergência, a spec da fase é a mais específica.

### 2.1 — Knowledge Item Model

```go
type KnowledgeItem struct {
    ID             string
    Topic          string
    Concept        string
    Definition     string
    Properties     []string
    TradeOffs      []string
    RelatedConcepts []string
    Source         string // athena | user_note | imported_doc
    Status         string // draft | approved | deprecated
    CreatedAt      time.Time
    UpdatedAt      time.Time
}
```

**Tarefas:**
- [ ] `internal/domain/knowledge/` — `KnowledgeItem` + `TransitionTo`: só `draft → approved` e `approved → deprecated` são válidas
- [ ] `Repository` sem `UpdateStatus` — aprovar/deprecar é sempre load → transição → `Update`, para que a regra não seja contornável
- [ ] `internal/infrastructure/sqlite/jsonlist.go` — codificação JSON-array-como-TEXT (padrão inédito no projeto)
- [ ] Schema SQLite (+ índices por `status, created_at` e por `topic`):

```sql
CREATE TABLE knowledge_items (
    id TEXT PRIMARY KEY,
    topic TEXT,
    concept TEXT,
    definition TEXT,
    properties TEXT, -- JSON array
    trade_offs TEXT, -- JSON array
    related_concepts TEXT, -- JSON array
    source TEXT,
    status TEXT DEFAULT 'draft',
    created_at DATETIME,
    updated_at DATETIME
);
```

### 2.2 — Knowledge Extraction

Sob demanda, o LLM extrai conceitos da sessão e os **propõe** como Knowledge Items. Nada é gravado até o usuário confirmar.

O domínio `study` não tem conceito de fim de sessão (`Session` não tem `EndedAt`, não existe caso de uso `End`), e reintroduzi-lo mexeria numa área já estável. O gatilho é portanto uma **ação explícita**: botão "Extract knowledge" no composer do chat.

- [ ] `internal/application/knowledge/extraction.go` — `ExtractFromSession` devolve candidatos **não persistidos**; `SaveDrafts` grava
- [ ] Prompt de extração estruturado → envelope `{"items":[...]}`, não array nu
- [ ] Validação em Go antes de persistir: tetos de tamanho, item inválido é pulado sem descartar os válidos, `SaveDrafts` regenera o ID e retorna os índices exatos persistidos para um retry sem duplicação
- [ ] JSON malformado → `ErrMalformedExtraction`; quem loga é o binding desktop, mantendo `internal/application` sem logging
- [ ] UI: modal "New knowledge found" com [Save as drafts / Dismiss] — o terceiro botão [Save and approve] entra na 2.3, quando `Approve` passa a existir

### 2.3 — Notes Import & Knowledge Explorer

> Substitui as antigas 2.3 (Notes Import) e 2.6 (Knowledge Explorer), tratadas
> como uma entrega conjunta — ver a nota de trilhas acima e a spec da fase para
> a justificativa completa.

- [ ] `internal/application/ingest/` — pipeline de ingestão sobre `fs.FS` (`os.OpenRoot` dá confinamento contra symlink escape; testes usam `fstest.MapFS`)
- [ ] UI: botão "Import notes" no toolbar do Knowledge Explorer + seletor de pasta (`PickNotesFolder` e `ImportNotes` como bindings separados; o diálogo Wails precisa ser injetável, como o `emit`) + `Dialog` de progresso (shadcn `progress`), terminando num resumo com falhas por arquivo
- [ ] Pipeline:

```text
Arquivos Markdown
    ↓
Parser (goldmark)
    ↓
Chunking por heading, com orçamento de ~2000 chars (4 chars ≈ 1 token)
    ↓
Metadata (source, file_path, heading, topic, status, created_at)
    ↓
Embeddings (llm.Provider.Embeddings — já implementado, sem chamadores hoje)
    ↓
knowledge_chunks (embedding como BLOB float32 little-endian)
    +
knowledge_items: um Item "sombra" por arquivo (Concept = H1/nome do arquivo,
Definition = preview de 300 chars, Status = approved direto, sem LLM)
```

- [ ] Suporte inicial: `.md` e `.txt`; diretórios ocultos (`.git`, `.obsidian`) são pulados na varredura
- [ ] Deduplicação por `file_path` + `mtime` + modelo de embedding; arquivo alterado **substitui** seus chunks numa transação, não duplica — e trocar o modelo re-embeda automaticamente. A mesma transação grava/atualiza o Item-sombra
- [ ] Conteúdo sem heading (texto corrido, front matter, só H4+) cai no split por parágrafo; nada é descartado por falta de heading
- [ ] Apagar uma nota pela tela do Explorer remove o Item e seus chunks, mas preserva o registro em `ingested_files` — reimportar a mesma pasta sem editar o arquivo não traz o conteúdo de volta; editar o arquivo e reimportar traz
- [ ] Sidebar esquerda com árvore de tópicos e conceitos, filtros por status (draft / approved / deprecated); a lista de Items mostra um selo de proveniência (Athena / Imported note) — sem listagem paralela, é o mesmo `ListItems`/`ListTopics` de sempre
- [ ] Ações gated pelo ciclo de vida: aprovar (só draft — nunca oferecido a nota importada, que já nasce approved), editar, deprecar (só approved), deletar (com aviso extra no `AlertDialog` para itens de origem `imported_doc`)
- [ ] Reusar `components/tag-input.tsx` no editor inline das três listas
- [ ] Schema:

```sql
CREATE TABLE knowledge_chunks (
    id TEXT PRIMARY KEY,
    source TEXT,
    topic TEXT,      -- necessário para os filtros da 2.4
    status TEXT,     -- idem
    item_id TEXT,    -- sempre preenchido: Item extraído (athena) ou Item-sombra (imported_doc)
    file_path TEXT,
    heading TEXT,
    content TEXT,
    embedding BLOB, -- float32 empacotado, little-endian
    embedding_model TEXT NOT NULL,
    item_updated_at DATETIME, -- NULL para imported_doc; usa ingested_files.mtime, não este campo
    created_at DATETIME
);

-- onde vive o mtime da deduplicação; registra também arquivos com zero chunks
CREATE TABLE ingested_files (
    file_path       TEXT PRIMARY KEY,
    mtime           INTEGER NOT NULL,
    embedding_model TEXT NOT NULL,
    chunk_count     INTEGER NOT NULL,
    item_id         TEXT NOT NULL, -- ID estável do Item-sombra entre reimportações
    ingested_at     DATETIME
);
```

### 2.4 — Vector Search

- [ ] `internal/infrastructure/vectorstore/` — busca por similaridade cosseno
- [ ] Pure Go, sem dependência externa de vector DB
- [ ] Retorna top-K `ScoredChunk` por similaridade + filtros (topic, source, status)
- [ ] `Remove` e `Len` além de `Add`/`Search`: sem `Remove`, item deletado continua respondendo até reiniciar; `Len` evita gastar embedding com base vazia
- [ ] Normalização dos vetores no insert **e da query no `Search`** — produto escalar só é cosseno com os dois lados unitários; filtra antes de pontuar
- [ ] Reimportar arquivo alterado remove os vetores antigos da memória, não só do SQLite
- [ ] Carga em memória no startup, a partir de `knowledge_chunks`
- [ ] Startup carrega somente chunks do modelo atual e, para Knowledge Items, com status/`item_updated_at` iguais ao registro fonte; índice obsoleto nunca responde
- [ ] `ADR-004` registrando a escolha do vector store e do modelo de embedding (trocar o modelo exige re-ingest)
- [ ] `internal/infrastructure/vectorstore` entra no escopo do `make mutation-go` (emenda à ADR-002)

### 2.5 — RAG Integration

- [ ] `internal/application/knowledge/retrieval.go`
- [ ] Fluxo Knowledge-First:

```text
Pergunta do usuário
    ↓
Embedding da pergunta
    ↓
Vector search → top-K chunks relevantes
    ↓
Conhecimento suficiente?
    YES → responder com conhecimento local
    NO  → chamar OpenRouter com chunks como contexto
```

- [ ] Source modes: `notes` / `strict-notes` / `web` — transiente, passado por chamada em `SendStudyMessage`, sem migração
- [ ] Em `strict-notes` sem chunks: responde `NoLocalKnowledgeMessage` **sem chamar o LLM** — inclusive com o store vazio, que é só a forma mais barata de não achar chunk e não pode curto-circuitar a checagem de modo
- [ ] `DefaultTopK` explícito, junto com os thresholds
- [ ] Thresholds de similaridade injetados no construtor (defaults 0.35 / 0.55, calibrados para `text-embedding-3-small`)
- [ ] Contexto injetado como **segunda mensagem `system`**, preservando `buildSystemPrompt` e seus testes
- [ ] Teto de contexto descartando chunks inteiros do menor score para cima — nunca truncar no meio, para que as fontes citadas batam com o que o modelo viu
- [ ] `study.Service` recebe uma **porta** `knowledge.Retriever` definida no domínio; orquestrar isso no binding violaria a ADR-001
- [ ] Evento `study:sources` com as fontes usadas (determinístico, em vez de pedir citação inline ao modelo)

### 2.7 — Knowledge Review

- [ ] Lista de itens em `draft` aguardando revisão, mais antigos primeiro
- [ ] Usuário aprova ou rejeita; `ApproveAllDrafts` itera por `TransitionTo`, `RejectAllDrafts` itera `Delete` — rejeitar não é mudança de status
- [ ] Badge com contador de itens pendentes, com estado no `AppShell` + callback — sem Context nem store

### 2.8 — Knowledge Item Indexing

Todos os Knowledge Items entram no vector store para permitir detecção de duplicidade em qualquer estado. Somente items aprovados são recuperáveis pelo RAG (`source = athena`, `status = approved`).

- [ ] Hook em `SaveDrafts` (indexa), `Approve`/`Deprecate`/`UpdateItem` (substituem status/conteúdo), `DeleteItem` (remove)
- [ ] Aprovar/deprecar atualiza só o status do chunk, sem novo embedding; editar conteúdo re-embeda
- [ ] Editar persiste o item e remove imediatamente o chunk antigo da memória; falha ao re-embeddar deixa o item temporariamente fora da busca, nunca servindo conteúdo obsoleto
- [ ] Falha de indexação **nunca** reverte uma escrita bem-sucedida — devolve o item persistido com `ErrIndexingFailed`, o binding loga e trata como sucesso, e o backfill recolhe depois
- [ ] Backfill consentido de todos os items criados antes da 2.8: alerta no Explorer com [Indexar agora], nunca silencioso no startup
- [ ] Backfill compara `item_updated_at`, status e modelo de embedding para encontrar chunks obsoletos, não apenas ausentes

### 2.9 — Persistent Provenance

- [ ] Extração marca mensagens por ID e exige ao menos uma citação literal `{message_id, quote}` válida por candidato (máximo 5 × 1000 caracteres)
- [ ] `knowledge_evidence` + `knowledge_item_evidence` guardam snapshots imutáveis; `Source` continua sendo apenas a categoria
- [ ] `message_sources` persiste, em ordem, exatamente os chunks que sobreviveram ao threshold e ao teto de contexto
- [ ] Mensagem final do assistente e fontes são gravadas atomicamente; sessão retomada restaura as fontes
- [ ] Explorer mostra evidências do item

### 2.10 — Knowledge Duplicate Detection

- [ ] Match exato por conceito normalizado dentro do tópico, sem embedding
- [ ] Sem match exato, busca semântica top-5 em items de todos os estados, threshold injetável default `0.90`
- [ ] Match exato bloqueia criação direta; match semântico alerta e permite criação separada somente por escolha explícita
- [ ] Backend repete a checagem no save para impedir bypass por chamada forjada ou UI stale

### 2.11 — Knowledge Reconciliation

- [ ] Propostas `create | update | relate | conflict | no_change`; LLM propõe, Go valida e usuário decide
- [ ] Sem candidatos duplicados, `create` é determinístico e não faz uma segunda chamada LLM
- [ ] Propostas salvas para revisão são persistidas separadamente de drafts e usam `TargetUpdatedAt` como controle otimista
- [ ] `knowledge_item_relations` liga IDs; `related` é simétrico e idempotente
- [ ] Target removido ou alterado torna a proposta `stale`; nenhuma alteração existente é automática

### 2.12 — Knowledge Revision History

- [ ] Snapshot imutável por criação, edição, aprovação, depreciação e reconciliação aplicada
- [ ] Revisão e mutação do item ficam na mesma transação; falha de indexação posterior não apaga a revisão
- [ ] Backfill `baseline` idempotente para items preexistentes
- [ ] Histórico read-only no Explorer com diff por campo e evidências; restore fica fora da Fase 2

### Done when (Fase 2)

- Usuário importa uma pasta de notas Markdown
- Faz uma sessão de estudo que usa as notas como contexto
- Extrai, reconcilia, revisa e aprova Knowledge Items com evidência persistente
- Respostas retomadas preservam as fontes originalmente usadas
- Duplicatas exatas não são criadas silenciosamente; conflitos sempre exigem decisão humana
- Knowledge Explorer mostra os itens por tópico, suas evidências e o histórico de revisões
- Items aprovados voltam como contexto em sessões seguintes

---

## Fase 3 — Learning Intelligence

**Objetivo:** O Athena detecta onde o usuário tem dificuldade, pratica com desafios e reforça com flashcards.

**Dependência:** Fase 2 completa.

### 3.1 — Challenge Mode

- [ ] `internal/domain/challenge/` — `ChallengeSession`
- [ ] UI: tela de desafio com problema, área de resposta, submissão
- [ ] LLM gera problemas baseados no perfil e histórico do usuário
- [ ] Avaliação estruturada:

```go
type Evaluation struct {
    Score       int      `json:"score"`
    Strengths   []string `json:"strengths"`
    Weaknesses  []string `json:"weaknesses"`
    Missing     []string `json:"missing_topics"`
    Suggestions []string `json:"suggestions"`
}
```

- [ ] Critérios de avaliação configuráveis por domínio (injeta via UserProfile)

### 3.2 — Evaluation Engine

- [ ] `internal/domain/evaluation/` — lógica de avaliação
- [ ] LLM retorna JSON estruturado (schema validado no Core)
- [ ] UI de resultado: strengths, improvements, score, sugestões
- [ ] Persistido em SQLite:

```sql
CREATE TABLE evaluations (
    id TEXT PRIMARY KEY,
    session_id TEXT,
    score INTEGER,
    strengths TEXT,
    weaknesses TEXT,
    missing TEXT,
    suggestions TEXT,
    created_at DATETIME
);
```

### 3.3 — Progress Tracking

- [ ] `internal/domain/progress/` — agregação de métricas
- [ ] Métricas por tópico: score médio, sessões, taxa de acerto, tempo
- [ ] Schema:

```sql
CREATE TABLE progress (
    id TEXT PRIMARY KEY,
    topic TEXT,
    subtopic TEXT,
    mode TEXT,
    score INTEGER,
    session_id TEXT,
    recorded_at DATETIME
);
```

- [ ] UI: tela de progresso com barras por tópico

### 3.4 — Gap Detection

- [ ] `internal/application/gaps/` — análise de padrões de dificuldade
- [ ] Algoritmo: tópicos com score médio abaixo de threshold → gap
- [ ] UI: dashboard de gaps com indicadores ✅ ⚠️ ❌ por tópico
- [ ] Sugestões automáticas de sessões de reforço

### 3.5 — Flashcards

#### Modelo

```go
type Flashcard struct {
    ID            string
    KnowledgeItem string
    Topic         string
    Type          string // front_back | cloze | multiple_choice
    Front         string
    Back          string
    Options       []string
    CorrectOption int
    Status        string // draft | active | suspended
    CreatedAt     time.Time
}

type FlashcardReview struct {
    FlashcardID string
    ReviewedAt  time.Time
    Quality     int     // 0–5 (SM-2)
    Interval    int     // dias
    EaseFactor  float64
}
```

#### Geração automática

- [ ] LLM extrai conceitos de Knowledge Items aprovados
- [ ] Gera 3 tipos: Front/Back, Cloze, Múltipla Escolha
- [ ] Cartões criados como `draft` → usuário aprova

#### Spaced Repetition (SM-2)

- [ ] `internal/application/flashcards/sm2.go` — algoritmo SM-2
- [ ] Cálculo de próximo intervalo baseado na qualidade da resposta (0–5)
- [ ] Fila diária de revisão priorizando cartões vencidos

#### UI de revisão

- [ ] Tela: cartão frente → botão "Ver resposta" → verso → avaliação (❌ / ⚠️ / ✅)
- [ ] Resumo ao final: acertos, erros, próxima revisão

#### Integração com Gap Detection

- [ ] Taxa de erro nos flashcards alimenta o gap detector
- [ ] Cartões com erros recorrentes → recomendação de sessão de estudo

### 3.6 — Knowledge Promotion

- [ ] Após sessões, modal de promoção de Knowledge Items de `draft` para `approved`
- [ ] Ao aprovar: flashcards são gerados automaticamente (com confirmação)

### Done when (Fase 3)

- Usuário faz um challenge e recebe avaliação estruturada
- Dashboard de gaps mostra tópicos com dificuldade
- Flashcards são gerados a partir de Knowledge Items aprovados
- Sessão de revisão diária funciona com SM-2

---

## Fase 4 — Interview Mode

**Objetivo:** Simulação completa de entrevista com timer, avaliação e histórico.

**Dependência:** Fase 3 completa.

### 4.1 — Interview Session

```go
type InterviewSession struct {
    ID        string
    Topic     string
    Mode      string // system_design | behavioral | domain_specific
    Questions []Question
    Answers   []Answer
    Score     int
    StartedAt time.Time
    EndedAt   time.Time
}
```

- [ ] `internal/domain/interview/` — lógica de sessão
- [ ] LLM conduz entrevista progressiva (perguntas que aprofundam conforme as respostas)
- [ ] Contexto do UserProfile determina domínio das perguntas:
  - Desenvolvedor → System Design
  - Advogado → Questões jurídicas
  - Veterinário → Casos clínicos

### 4.2 — Timer

- [ ] Tempo configurável por questão (30s / 1min / 2min / sem limite)
- [ ] Contador visível na UI durante a resposta
- [ ] Ao expirar: salva resposta parcial e avança

### 4.3 — Avaliação de Entrevista

- [ ] Evaluation Engine avalia cada resposta individualmente
- [ ] Score final agregado da sessão
- [ ] Relatório: por questão (strengths, improvements) + score geral
- [ ] Persistência: `evaluations` linkadas à `interview_session`

### 4.4 — Histórico de Entrevistas

- [ ] Lista de entrevistas passadas na UI
- [ ] Detalhe: questões, respostas, scores, feedbacks
- [ ] Evolução ao longo do tempo por tópico

### 4.5 — Domain-Aware Evaluation

- [ ] Critérios de avaliação variam por área (via UserProfile):

```text
TI/System Design:  Escalabilidade, Trade-offs, Correctness, Design
Direito:           Fundamentação, Argumentação, Legislação, Precisão
Medicina Vet:      Diagnóstico diferencial, Protocolo, Raciocínio clínico
Concursos:         Completude, Precisão factual, Organização
```

### Done when (Fase 4)

- Usuário faz uma entrevista completa com 3+ questões e timer
- Recebe relatório de avaliação por questão e score final
- Histórico de entrevistas acessível na UI
- Domínio da entrevista reflete o perfil do usuário

---

## Fase 5 — Comercialização

**Objetivo:** Receita. Planos ativados, pagamento integrado, feature gating na UI.

**Dependência:** Fase 1 completa (pode ser desenvolvida em paralelo com fases 2–4).

### 5.1 — Planos e Feature Gating

```text
Essencial: Study Mode + Knowledge Base básica + notas
Pro:       + Challenge + Interview + Gap Detection + flashcards + modelos premium
Expert:    + funcionalidades futuras + acesso antecipado + suporte prioritário
```

- [ ] `Plan` retornado pela API de conta
- [ ] `internal/application/licensing/` — verifica permissões por plano
- [ ] Wails binding: `GetCurrentPlan()`, `CanAccess(feature string)`
- [ ] UI: features bloqueadas exibem lock icon + "Disponível no plano Pro"

### 5.2 — Paddle Integration

- [ ] Conta Paddle configurada
- [ ] Produtos criados no Paddle: Essencial Mensal, Essencial Anual, Pro Mensal, Pro Anual, Expert Mensal, Expert Anual
- [ ] Webhook no backend: `POST /webhooks/paddle` → atualiza plano na conta
- [ ] UI: botão "Fazer upgrade" → abre checkout Paddle (browser externo ou overlay)

### 5.3 — Tela de Planos

- [ ] Exibida ao fim do trial
- [ ] Tabela comparativa: Essencial / Pro / Expert
- [ ] Toggle Mensal / Anual (mostra desconto)
- [ ] Preços: R$ 19/39/69 mensal | R$ 152/312/552 anual

### 5.4 — macOS Distribution

- [ ] Apple Developer account
- [ ] Code signing + notarização (`xcrun notarytool`)
- [ ] Build matrix CI: adicionar `macos-latest`
- [ ] Artifact: `.dmg`
- [ ] Homebrew Cask (opcional, pós-lançamento)

### 5.5 — Canais de Distribuição

**Windows:**
- [ ] `.exe` installer (Wails gera nativamente)
- [ ] Code signing EV certificate
- [ ] winget package publicado

**Linux:**
- [ ] `.AppImage` (prioritário)
- [ ] `.deb` para Ubuntu/Debian
- [ ] Flathub submission

### Done when (Fase 5)

- Trial expira e usuário consegue fazer upgrade com pagamento real
- Funcionalidades bloqueadas são claramente indicadas por plano
- Build macOS funcionando na CI
- App publicado nos canais de distribuição principais

---

## Fase 6 — Voice Interview

**Objetivo:** Entrevista por áudio — a IA faz perguntas em voz, escuta a resposta e avalia.

**Dependência:** Fase 4 completa.

> Esta fase requer integração direta com APIs de áudio (não disponíveis via OpenRouter).
> É uma exceção justificada à regra de "OpenRouter only".

### Arquitetura

```text
Microfone do usuário
    ↓
STT (Whisper API ou browser Web Speech API)
    ↓
Texto → LLM (processa e gera próxima pergunta)
    ↓
TTS (OpenAI TTS ou ElevenLabs)
    ↓
Áudio reproduzido para o usuário
```

### Tarefas

- [ ] Integração com Whisper API (OpenAI) para STT
- [ ] Integração com TTS (OpenAI TTS como primeira opção)
- [ ] `internal/infrastructure/audio/` — STT e TTS providers
- [ ] UI: tela de entrevista com controles de microfone (iniciar/parar gravação)
- [ ] Indicador visual de "IA falando" e "aguardando resposta"
- [ ] Transcrição exibida em tempo real na UI durante a fala do usuário

### Providers a avaliar

```text
STT:  OpenAI Whisper API (cloud) | Web Speech API (browser, gratuito)
TTS:  OpenAI TTS | ElevenLabs (mais realista, pago)
```

### LLMProvider extension

```go
type AudioProvider interface {
    Transcribe(ctx context.Context, audio []byte) (string, error)
    Speak(ctx context.Context, text string) ([]byte, error)
}
```

### Done when (Fase 6)

- Usuário faz entrevista completa falando e ouvindo
- IA transcreve a resposta, avalia e faz a próxima pergunta em voz
- Histórico salvo com transcrição textual

---

## Fase 7 — Advanced Features

**Objetivo:** Funcionalidades de diferenciação que completam a visão do produto.

**Dependência:** Fase 4 completa.

### 7.1 — Knowledge Graph

- [ ] `internal/domain/knowledge/graph.go` — relacionamentos entre conceitos
- [ ] Reutilizar `knowledge_item_relations` da 2.11 sem nova tabela: `related` é simétrica; `prerequisite` e `extends` usam direção `from_item_id → to_item_id`; nomes não são chaves
- [ ] UI: visualização de grafo com biblioteca (React Flow ou similar)
- [ ] Navegação: clicar em conceito abre Knowledge Item

### 7.2 — Whiteboard / Architecture Mode

- [ ] Modelo semântico de diagrama (não coordenadas gráficas):

```go
type Diagram struct {
    Nodes []Node `json:"nodes"`
    Edges []Edge `json:"edges"`
}

type Node struct {
    ID   string `json:"id"`
    Type string `json:"type"` // api-server | database | cache | queue | etc.
}

type Edge struct {
    Source string `json:"source"`
    Target string `json:"target"`
    Label  string `json:"label"`
}
```

- [ ] `internal/domain/architecture/` — avaliação de diagramas
- [ ] Frontend: biblioteca visual (React Flow ou tldraw — decisão na fase)
- [ ] Regras determinísticas: single point of failure, missing replication
- [ ] LLM evaluation: trade-offs, missing concerns
- [ ] Architecture Score: Scalability / Reliability / Cost / Trade-offs / Design

### 7.3 — Algorithm Mode (IT-specific)

> Módulo opcional, visível apenas para usuários com perfil de área TI.

- [ ] Problema de algoritmo → editor de código no desktop
- [ ] Execução em sandbox isolado (container ou processo restrito)
- [ ] Avaliação: correctness, complexity, code quality, edge cases
- [ ] Segurança: CPU + memória + tempo de execução limitados

### Done when (Fase 7)

- Knowledge Graph mostra relações entre conceitos estudados
- Usuário desenha uma arquitetura e recebe avaliação com score
- Algorithm Mode funciona com execução segura de código (IT users)

---

## Concerns Transversais

Estes itens devem ser considerados em todas as fases:

### Segurança
- Fase 1: auth 100% local (senha com hash bcrypt em SQLite), sem JWT nem chamada de rede
- Tokens JWT com expiração curta + refresh — só se aplica quando um backend remoto de auth existir (Fase 5+); a porta `AccountRepository` (1.1) já é desenhada para essa troca
- Chave OpenRouter nunca enviada para o servidor próprio (fica local)
- Execução de código em sandbox (Fase 7)
- HTTPS obrigatório nas chamadas à API de auth, quando essa API existir

### Observabilidade
- Logs estruturados (JSON) em `~/.athena/logs/`
- Rastreamento de custo por sessão (tokens + R$)
- Erros do LLM reportados claramente na UI

### Offline
- App funciona offline para funcionalidades locais (Knowledge Base, flashcards, auth — tudo local na Fase 1)
- Somente o LLM requer internet
- Grace period de validação de licença offline — só se aplica quando houver licenciamento remoto (Fase 5+); não existe licença para validar na Fase 1

### Internacionalização
- App inicialmente em Português (BR)
- Prompts do LLM: Language do UserProfile (futuro)

---

## Diretório Local do Usuário

```text
~/.athena/
├── config.yaml        # chave OpenRouter, modelo padrão
├── profile.json       # UserProfile
├── session.json       # token de autenticação
├── athena.db          # SQLite (knowledge, sessions, flashcards, progress)
├── vectors/           # embeddings locais
├── cache/             # response cache
└── logs/              # logs de execução
```

---

## Ordem de Implementação Recomendada

```text
Fase 0 — Foundation (tooling, CI, Wails setup)
    ↓
Fase 1 — Desktop MVP (login, onboarding, study, OpenRouter)
    ↓
Fase 2 — Knowledge Engine (knowledge items, RAG, notas)       ←─┐
    ↓                                                            │
Fase 3 — Learning Intelligence (challenge, gaps, flashcards)   │ em paralelo
    ↓                                                            │
Fase 5 — Comercialização (planos, Paddle, macOS) ───────────────┘
    ↓
Fase 4 — Interview Mode (entrevista completa)
    ↓
Fase 6 — Voice Interview (áudio)
    ↓
Fase 7 — Advanced Features (whiteboard, graph, algorithms)
```

> **Fase 5 pode iniciar após a Fase 1** e ser desenvolvida em paralelo com as fases 2, 3 e 4, pois depende apenas da estrutura de contas e licenças da Fase 1.
