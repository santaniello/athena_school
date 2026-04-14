# Athena — Implementation Phases

Cada fase entrega software funcionando. Não inicie a próxima fase antes de a atual estar completa e testada.

---

## Estrutura de Diretórios (Package-Oriented Design)

```
athena/
├── cmd/
│   └── athena/              # CLI wiring: Cobra setup, flags, output — imports internal/
│       ├── main.go
│       ├── root.go
│       ├── study.go
│       └── ...
├── internal/
│   ├── study/               # domain: study session logic
│   ├── challenge/           # domain: challenge logic
│   ├── interview/           # domain: interview logic
│   ├── progress/            # domain: progress tracking
│   ├── rag/                 # domain: RAG / semantic search
│   └── platform/            # foundational packages (no app policy)
│       ├── config/          # config load/save (YAML)
│       ├── llm/             # LLMProvider interface + factory
│       │   └── ollama/      # Ollama HTTP implementation
│       └── storage/         # persistence layer (future)
├── go.mod
└── Makefile
```

**Import rules:**
- `cmd/athena/` → may import `internal/` (domain) and `internal/platform/`
- `internal/<domain>/` → may import `internal/platform/`; never imports `cmd/`
- `internal/platform/<pkg>/` → may NOT import siblings at the same level; sets no app policy

---

## Princípios (XP)

- **Incrementos pequenos** — cada spec é implementável em 1-3 dias
- **Working software first** — nada de scaffolding sem funcionalidade real
- **Simple design** — o mínimo necessário para o requisito, sem abstrações prematuras
- **Test as you go** — cada spec tem critérios de aceitação verificáveis

---

## Visão Geral

| Fase | Foco | Entrega |
|---|---|---|
| [Phase 01](phase-01-foundation/) | Fundação | CLI funcional + Ollama + `study` |
| [Phase 02](phase-02-challenge/) | Prática ativa | `challenge` + tracking de progresso |
| [Phase 03](phase-03-interview/) | Simulação | `interview` + source modes + subtópicos |
| [Phase 04](phase-04-rag/) | Notas pessoais | `ingest` + busca semântica |
| [Phase 05](phase-05-tui/) | UX do terminal | Interface TUI com Bubble Tea |
| [Phase 06](phase-06-gui/) | Expansão | Dashboard web + multi-provider |
| [Phase 07](phase-07-algorithms/) | Código | `algo` + execução + coding interview |

---

## Phase 01 — Foundation

> **Objetivo:** Ter um `athena study system-design caching` funcionando end-to-end com Ollama.

| Spec | Descrição |
|---|---|
| [01-project-setup.md](phase-01-foundation/01-project-setup.md) | Go module, Cobra CLI, Makefile |
| [02-config-system.md](phase-01-foundation/02-config-system.md) | Config persistente em YAML |
| [03-llm-provider.md](phase-01-foundation/03-llm-provider.md) | Interface `LLMProvider` + Ollama |
| [04-study-command.md](phase-01-foundation/04-study-command.md) | Sessão de estudo interativa |

**Done when:** `athena study system-design caching` explica o tópico, pergunta algo, e dá feedback.

---

## Phase 02 — Challenge & Progress

> **Objetivo:** Prática com problemas reais e histórico de evolução.

| Spec | Descrição |
|---|---|
| [01-challenge-command.md](phase-02-challenge/01-challenge-command.md) | Desafio com avaliação por critérios |
| [02-progress-tracking.md](phase-02-challenge/02-progress-tracking.md) | Score persistido + `athena progress` |

**Done when:** `athena challenge system-design` e `athena progress` mostram evolução.

---

## Phase 03 — Interview & Navigation

> **Objetivo:** Simulação de entrevista com timer, controle de fonte e navegação por subtópicos.

| Spec | Descrição |
|---|---|
| [01-interview-command.md](phase-03-interview/01-interview-command.md) | Entrevista cronometrada multi-questão |
| [02-source-modes.md](phase-03-interview/02-source-modes.md) | `--source notes/web/strict-notes` |
| [03-subtopics.md](phase-03-interview/03-subtopics.md) | Sugestão e seleção de subtópicos |

**Done when:** `athena interview system-design --source web` roda 3 questões com timer e score final.

---

## Phase 04 — RAG (Notes Integration)

> **Objetivo:** Notas pessoais do usuário como base de conhecimento.

| Spec | Descrição |
|---|---|
| [01-ingest-command.md](phase-04-rag/01-ingest-command.md) | Indexação de arquivos Markdown |
| [02-semantic-search.md](phase-04-rag/02-semantic-search.md) | Recuperação por similaridade + injeção no prompt |

**Done when:** `athena ingest ./notes && athena study caching --source notes` usa as notas do usuário.

---

## Phase 05 — TUI

> **Objetivo:** Interface de terminal rica e fluida.

| Spec | Descrição |
|---|---|
| [01-tui-interface.md](phase-05-tui/01-tui-interface.md) | Bubble Tea: layout, viewport, timer |

**Done when:** `athena tui` permite rodar qualquer sessão com layout visual completo.

---

## Phase 06 — GUI & Multi-Provider

> **Objetivo:** Dashboard web de progresso e suporte a provedores cloud.

| Spec | Descrição |
|---|---|
| [01-dashboard.md](phase-06-gui/01-dashboard.md) | Web server + mapa de conhecimento |
| [02-multi-provider.md](phase-06-gui/02-multi-provider.md) | OpenAI, Claude, Gemini |

**Done when:** `athena serve` abre dashboard; `--provider openai` funciona.

---

## Phase 07 — Algorithm Mode

> **Objetivo:** Prática de algoritmos com execução real de código.

| Spec | Descrição |
|---|---|
| [01-algo-command.md](phase-07-algorithms/01-algo-command.md) | Problema → editor → testes → feedback |

**Done when:** `athena algo two-sum --run solution.go` roda testes e avalia a solução.

---

## Dependências entre Fases

```
Phase 01 (Foundation)
    └── Phase 02 (Challenge + Progress)
            └── Phase 03 (Interview + Source Modes + Subtopics)
                    └── Phase 04 (RAG)        ← depende de Phase 03 (source modes)
                    └── Phase 05 (TUI)        ← depende de Phase 03 (sessions completas)
                            └── Phase 06 (GUI + Multi-Provider)
                                    └── Phase 07 (Algorithms)
```
