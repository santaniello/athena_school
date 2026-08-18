# Athena — Especificação de Produto e Arquitetura

> **Versão:** 2.0
> **Status:** Especificação inicial
> **Objetivo:** Definir a visão, arquitetura e roadmap do Athena como uma plataforma pessoal de aprendizado técnico baseada em conhecimento.

---

# 1. Visão do Produto

Athena é um **ambiente pessoal de aprendizado técnico baseado em conhecimento**, desenvolvido inicialmente para desenvolvedores.

O objetivo é transformar o processo de estudo em um ciclo contínuo:

```text
Knowledge
    ↓
Study
    ↓
Practice
    ↓
Evaluation
    ↓
Gap Detection
    ↓
Reinforcement
    ↓
Knowledge Update
    ↓
Knowledge
```

O Athena não deve ser apenas uma interface para conversar com um LLM.

O verdadeiro produto é uma **base de conhecimento pessoal que evolui conforme o usuário estuda**, totalmente personalizada a partir do perfil do usuário, combinando:

* conhecimento gerado pelo Athena;
* notas pessoais;
* documentos importados;
* conhecimento externo;
* exercícios;
* respostas do usuário;
* avaliações;
* histórico de aprendizado.

O LLM é um componente de inteligência utilizado quando necessário, e não o centro do produto.

---

# 2. Proposta de Valor

> **Athena é um ambiente de aprendizado técnico ativo, personalizado e mensurável, que constrói progressivamente uma base de conhecimento pessoal.**

Principais diferenciais:

* aprendizado ativo;
* conhecimento persistente;
* integração com notas pessoais;
* RAG;
* controle das fontes de conhecimento;
* avaliação automática;
* detecção de gaps;
* entrevistas técnicas;
* exercícios práticos;
* evolução mensurável;
* arquitetura visual;
* whiteboard para system design;
* múltiplos modelos através de um único gateway;
* possibilidade de funcionamento parcialmente offline.

---

# 3. Público-Alvo

Principalmente:

* desenvolvedores backend;
* desenvolvedores interessados em arquitetura;
* estudantes de System Design;
* candidatos a entrevistas técnicas;
* pessoas estudando algoritmos;
* usuários de Obsidian e Markdown;
* profissionais buscando evolução técnica contínua.

---

# 4. Princípio Arquitetural Fundamental

A interface não deve conter regras de negócio.

O Athena deve possuir um **Core independente da interface**.

```text
                ┌───────────────┐
                │ Athena Desktop│
                └───────┬───────┘
                        │
                ┌───────▼───────┐
                │   Athena Core │
                └───────┬───────┘
                        │
        ┌───────────────┼────────────────┐
        │               │                │
   Knowledge        Learning           LLM
    Engine           Engine           Service
        │               │                │
        ▼               ▼                ▼
   Vector DB        Tracking         OpenRouter
```

---

# 5. Plataforma

## 5.1 Interface principal

A interface principal será um **aplicativo desktop**.

A arquitetura deve permitir:

* desktop;
* futuramente outras interfaces.

## 5.2 Tecnologia sugerida

### Backend/Core

* Go

### Desktop

* Wails
* React
* TypeScript

### Persistência

* SQLite inicialmente

### LLM Gateway

* OpenRouter

### Vector Search

* solução local inicialmente;
* possibilidade de evolução para banco vetorial dedicado.

---

# 6. Por que Desktop

O Athena possui funcionalidades que vão além de uma CLI tradicional:

* Knowledge Explorer;
* dashboard;
* histórico;
* gráficos;
* mapas de conhecimento;
* editor de notas;
* entrevistas;
* diagramas;
* whiteboard;
* visualização de arquitetura;
* análise de diagramas.

Por isso, a interface desktop é a única interface do produto.

---

# 7. Arquitetura Geral

```text
┌───────────────────────────────────────────────┐
│                ATHENA DESKTOP                 │
│                                               │
│             React + TypeScript                │
│                                               │
│  Study │ Challenge │ Interview │ Knowledge    │
│                                               │
│            Whiteboard / Diagrams              │
└───────────────────────┬───────────────────────┘
                        │
                      Wails
                        │
┌───────────────────────▼───────────────────────┐
│                  ATHENA CORE                  │
│                     Go                        │
│                                               │
│ ┌──────────────┐ ┌──────────────┐ ┌─────────┐ │
│ │  Knowledge   │ │   Learning   │ │   LLM   │ │
│ │   Engine     │ │    Engine    │ │ Service │ │
│ └──────┬───────┘ └──────┬───────┘ └────┬────┘ │
│        │                 │              │      │
│        ▼                 ▼              ▼      │
│    SQLite            Tracking       OpenRouter │
│    Vector DB                               │   │
└───────────────────────────────────────────────┘
```

---

# 8. Knowledge Engine

O Knowledge Engine é uma das partes centrais do Athena.

Ele será responsável por:

* armazenar conhecimento;
* indexar conhecimento;
* realizar busca semântica;
* importar notas;
* controlar fontes;
* versionar conhecimento;
* promover conhecimento de draft para aprovado;
* detectar conteúdo duplicado;
* recuperar contexto para o LLM.

---

# 9. Knowledge Base

A Knowledge Base deve combinar diferentes fontes.

```text
Knowledge Base
│
├── Athena Knowledge
│
├── User Notes
│
├── Imported Documents
│
└── External Knowledge
```

## 9.1 Athena Knowledge

Conhecimento adquirido durante sessões de estudo.

Exemplo:

```text
Topic: System Design

Concept: Cache Aside

Definition:
A aplicação consulta o cache antes do banco...

Trade-offs:
- simplicidade;
- possibilidade de cache miss;
- risco de dados desatualizados.
```

## 9.2 User Notes

Notas pessoais importadas pelo usuário.

Exemplo:

```text
~/notes/system-design/

├── caching.md
├── redis.md
├── sharding.md
└── load-balancing.md
```

Importação:

```bash
athena ingest ./notes
```

## 9.3 Imported Documents

Possibilidade futura de importar:

* PDF;
* HTML;
* Markdown;
* documentos técnicos;
* livros;
* artigos.

---

# 10. Knowledge Item

O Athena deve trabalhar com unidades de conhecimento, e não apenas perguntas e respostas.

Exemplo:

```text
Knowledge Item
─────────────────────────

Topic:
system-design

Concept:
cache-aside

Definition:
...

Properties:
- lazy loading
- application controlled population

Trade-offs:
- stale data
- cache miss
- cache stampede

Related Concepts:
- redis
- ttl
- eviction
```

Isso evita transformar a Knowledge Base em um simples FAQ.

---

# 11. Knowledge Lifecycle

O Athena não deve salvar automaticamente toda resposta do LLM como conhecimento confiável.

Isso poderia persistir alucinações.

O conhecimento deverá possuir estados:

```text
draft
approved
deprecated
```

Fluxo:

```text
LLM Response
     ↓
Knowledge Extraction
     ↓
Draft
     ↓
Review
     ↓
Approved
```

O usuário poderá revisar, aprovar ou rejeitar conhecimento pela interface do desktop.

## 11.1 Proveniência Persistente

`source` identifica somente a categoria do conhecimento. Cada Knowledge Item também
deve apontar para evidências concretas — mensagens da sessão ou chunks importados —
com um snapshot imutável do trecho que o sustentou.

```text
Knowledge Item
      ↓
Item Evidence
      ↓
Session Message / Imported Chunk + immutable excerpt
```

Respostas assistidas por RAG preservam a lista exata de fontes que entrou no
contexto. Ao retomar uma sessão, as fontes reaparecem mesmo que a nota ou o item
original tenha mudado depois.

## 11.2 Detecção de Duplicidade e Reconciliação

Novo conhecimento não deve ser acumulado cegamente. Antes de criar um item, Athena
compara o conceito dentro do tópico por normalização determinística e similaridade
semântica.

```text
Extracted candidate
       ↓
Exact / semantic matches
       ↓
create | update | relate | conflict | no_change
       ↓
Human review
```

O LLM apenas propõe a classificação e um diff limitado. O backend valida o alvo, a
evidência e os campos alteráveis; somente o usuário pode aplicar a proposta. Um alvo
alterado depois da análise torna a proposta obsoleta e exige nova reconciliação.

## 11.3 Histórico de Revisões

Criação, edição, aprovação, depreciação e reconciliação aplicada geram snapshots
imutáveis, numerados e ligados às evidências da mudança. O histórico mostra o diff
entre revisões, mas a Fase 2 não restaura versões antigas automaticamente.

---

# 12. Knowledge Promotion

Após uma sessão:

```text
Athena:
"Cache Aside é uma estratégia..."

        ↓

💡 Novo conhecimento encontrado

Cache Aside

[1] Salvar como conhecimento
[2] Salvar como rascunho
[3] Ignorar
```

O modo automático poderá existir futuramente:

```yaml
knowledge:
  auto-save: true
```

Porém, mesmo nesse caso, o conteúdo poderá permanecer como `draft`.

---

# 13. RAG

RAG não deve ser tratado como uma funcionalidade isolada de "notas".

RAG é o mecanismo utilizado pelo Knowledge Engine para recuperar conhecimento relevante.

Fluxo:

```text
Question
    ↓
Embedding
    ↓
Semantic Search
    ↓
Relevant Knowledge
    ↓
Context Builder
    ↓
LLM
```

---

# 14. Knowledge-First Architecture

O Athena deve priorizar conhecimento existente antes de chamar o LLM.

```text
Question
    ↓
Knowledge Retrieval
    ↓
Knowledge sufficient?
    │
 ┌──┴─────┐
 │        │
YES       NO
 │        │
 ▼        ▼
Answer   OpenRouter
          │
          ▼
        Answer
          │
          ▼
     Knowledge Draft
```

Isso reduz custos e cria uma memória persistente para o Athena.

---

# 15. Cache vs Knowledge Base

A Knowledge Base não deve ser tratada simplesmente como cache.

### Cache

```text
Question → Answer
```

### Athena

```text
Question
   ↓
Knowledge Retrieval
   ↓
Concepts / Facts / Notes / Context
   ↓
Answer
```

A Knowledge Base representa conhecimento reutilizável.

O cache de respostas pode existir adicionalmente como otimização.

---

# 16. Response Cache

O Athena poderá possuir um cache de respostas.

Exemplo:

```text
Question Hash
+
Relevant Context Hash
+
Model
+
Prompt Version
```

Se todos forem iguais:

```text
Cache Hit
```

Isso evita chamadas desnecessárias ao OpenRouter.

Porém, o cache de respostas não substitui a Knowledge Base.

---

# 17. Source Modes

O usuário poderá controlar a origem do conhecimento pela interface do desktop.

## notes

Usa:

```text
User Notes
+
Athena Knowledge
```

## strict-notes

Usa somente:

```text
User Notes
```

O LLM não deve utilizar conhecimento externo fora do contexto permitido.

## web

Pode combinar:

```text
User Notes
+
Athena Knowledge
+
External/Web Knowledge
```

---

# 18. LLM Architecture

O Athena terá inicialmente apenas **um gateway de LLM**:

> OpenRouter

Não haverá integração direta inicial com:

* OpenAI;
* Anthropic;
* Google Gemini;
* outros providers.

O OpenRouter será responsável por disponibilizar os diferentes modelos.

```text
Athena
   ↓
LLMService
   ↓
OpenRouter
   ↓
Claude / Gemini / GPT / etc.
```

---

# 19. LLM Abstraction

O Core não deve depender diretamente do OpenRouter.

```go
type LLMProvider interface {
    Chat(
        ctx context.Context,
        req ChatRequest,
    ) (ChatResponse, error)

    ChatStream(
        ctx context.Context,
        req ChatRequest,
        handler func(chunk string) error,
    ) error

    Embeddings(
        ctx context.Context,
        req EmbeddingRequest,
    ) (EmbeddingResponse, error)
}
```

Implementação inicial:

```text
OpenRouterProvider
```

---

# 20. LLM Service

O `LLMService` será responsável por:

* selecionar modelo;
* construir requisições;
* controlar streaming;
* registrar consumo;
* tratar erros;
* executar fallback;
* aplicar políticas de custo.

O domínio não deve conhecer detalhes da API do OpenRouter.

---

# 21. Model Router

O Athena poderá selecionar modelos conforme a tarefa.

Exemplo:

```text
Study
  → cheap

Challenge
  → medium

Interview
  → premium

Evaluation
  → premium
```

Configuração:

```yaml
llm:
  provider: openrouter

  models:
    premium:
      - anthropic/...
      - google/...

    cheap:
      - google/...

    free:
      - openrouter/free
```

---

# 22. Budget Management

O Athena deverá acompanhar:

* input tokens;
* output tokens;
* custo;
* modelo;
* sessão;
* tarefa.

Exemplo:

```go
type Usage struct {
    Model        string
    InputTokens  int
    OutputTokens int
    Cost         float64
}
```

Futuro:

```yaml
llm:
  policy:
    max-cost-per-request: 0.05
    monthly-budget: 10.00
```

---

# 23. Fallback

Existem dois níveis de fallback.

## 23.1 Fallback técnico

Responsabilidade do gateway/OpenRouter.

```text
Model A
   ↓ failure
Model B
   ↓ failure
Model C
```

## 23.2 Fallback econômico

Responsabilidade do Athena.

```text
Budget OK
   ↓
Premium Model

Budget low
   ↓
Cheap Model

Budget exhausted
   ↓
Free Model
```

O Athena poderá utilizar um modelo gratuito disponibilizado pelo OpenRouter como último nível.

---

# 24. Streaming

O desktop deve suportar streaming.

Exemplo:

```text
Athena > Explain caching

Caching is a technique...
```

O usuário não precisa esperar a resposta completa para começar a visualizar o conteúdo.

---

# 25. Study Mode

Funcionalidades:

* explicação guiada;
* perguntas;
* interação;
* feedback;
* identificação de gaps;
* integração com Knowledge Base;
* reforço de conhecimento.

---

# 26. Challenge Mode

Funcionalidades:

* problemas práticos;
* resposta do usuário;
* avaliação;
* feedback;
* score;
* sugestões de estudo.

---

# 27. Interview Mode

Funcionalidades:

* simulação de entrevista;
* perguntas progressivas;
* tempo controlado;
* avaliação;
* score final;
* histórico.

---

# 28. Evaluation Engine

As respostas poderão ser avaliadas por critérios estruturados:

```text
Clarity
Organization
Technical Depth
Scalability
Trade-offs
Correctness
```

Exemplo:

```text
Strengths:
- Boa decomposição

Improvements:
- Faltou discutir cache invalidation

Score:
7/10
```

---

# 29. Structured Evaluation

O LLM deve retornar estruturas tipadas sempre que possível.

Exemplo:

```go
type Evaluation struct {
    Score       int      `json:"score"`
    Strengths   []string `json:"strengths"`
    Weaknesses  []string `json:"weaknesses"`
    Missing     []string `json:"missing_topics"`
    Suggestions []string `json:"suggestions"`
}
```

Fluxo:

```text
LLM
 ↓
JSON
 ↓
Validation
 ↓
Evaluation
 ↓
UI
```

---

# 30. Gap Detection

O Athena deve identificar dificuldades recorrentes.

Exemplo:

```text
System Design

✅ Queues
⚠️ Caching
❌ Sharding
```

Sugestão:

```text
Você demonstra dificuldade em sharding.

Sugestão:
athena study system-design sharding
```

---

# 31. Learning Tracking

O Athena deverá armazenar:

* score;
* tempo de resposta;
* taxa de acerto;
* quantidade de sessões;
* temas estudados;
* dificuldades;
* progresso;
* conhecimento aprovado;
* conhecimento revisado.

---

# 32. Topics

Estrutura:

```text
system-design
├── caching
├── load-balancing
├── sharding
├── queues
└── databases
```

---

# 33. Sugestão de Subtopics

Quando o usuário estudar um tema:

```text
System Design

Subtopics:

[1] Caching
[2] Load Balancing
[3] Sharding
[4] Queues
```

O Athena poderá gerar e persistir a estrutura de tópicos.

---

# 34. Notes Integration

A importação é feita pela interface do desktop.

Pipeline:

```text
Files
 ↓
Parser
 ↓
Chunking
 ↓
Metadata
 ↓
Embeddings
 ↓
Vector Store
```

Metadata recomendada:

```text
source
file
topic
heading
created_at
updated_at
```

---

# 35. Vector Search

A busca deverá considerar:

* similaridade semântica;
* topic;
* source;
* metadata;
* estado do conhecimento.

Exemplo:

```text
Query:
"Como evitar cache stampede?"

↓

Top Results:

1. caching.md
2. redis.md
3. cache-aside.md
```

---

# 36. Whiteboard / Architecture Mode

Funcionalidade futura.

Objetivo:

Permitir que o usuário desenhe arquiteturas de sistemas e receba avaliação automática.

Exemplo:

```text
Client
   ↓
Load Balancer
   ↓
API
  / \
Redis DB
```

---

# 37. Whiteboard Technology

O Wails não precisa fornecer o editor visual.

O frontend poderá utilizar tecnologias especializadas.

Possíveis opções:

* React Flow;
* tldraw;
* Konva;
* Fabric.js;
* Excalidraw;
* Mermaid.

A escolha deverá ser feita quando a funcionalidade for implementada.

---

# 38. Diagram Model

O backend não deve receber apenas coordenadas gráficas.

Evitar:

```json
{
  "x": 300,
  "y": 200,
  "width": 100
}
```

Preferir um modelo semântico:

```json
{
  "nodes": [
    {
      "id": "api-1",
      "type": "api-server"
    },
    {
      "id": "redis-1",
      "type": "redis"
    }
  ],
  "edges": [
    {
      "source": "api-1",
      "target": "redis-1"
    }
  ]
}
```

Isso permite trocar a biblioteca visual sem modificar o Core.

---

# 39. Architecture Evaluation

A avaliação do whiteboard deverá ser híbrida.

## Deterministic Rules

Exemplos:

```text
❌ Single Point of Failure
⚠️ Database without replication
⚠️ Missing load balancing
```

## LLM Evaluation

Exemplos:

```text
✅ Boa separação de responsabilidades
⚠️ Redis pode não ser necessário
⚠️ Faltou discutir idempotência
```

---

# 40. Architecture Score

Exemplo:

```text
Architecture Score: 8.2/10

Scalability:       9/10
Reliability:       7/10
Cost:              8/10
Trade-offs:        7/10
Technical Design:  9/10
```

O score deverá considerar tanto regras determinísticas quanto avaliação semântica.

---

# 41. Algorithm Mode

Funcionalidade futura.

Avaliação:

* correctness;
* tests;
* complexity;
* code quality;
* edge cases.

Exemplo:

```text
✅ Passed tests

Complexity:
O(n)

Score:
9/10
```

---

# 42. Coding Interview

Possibilidades:

* perguntas;
* coding challenges;
* timer;
* execução;
* testes;
* avaliação;
* score.

---

# 43. Segurança na Execução de Código

A execução de código do usuário deverá ser isolada.

Nunca executar código arbitrário diretamente no processo principal do Athena.

Possível arquitetura futura:

```text
Athena
  ↓
Sandbox
  ↓
Container / isolated process
  ↓
Tests
```

Limitações:

* CPU;
* memória;
* filesystem;
* rede;
* tempo de execução.

---

# 44. Desktop UI

A aplicação poderá possuir:

```text
┌──────────────────────────────────────────────┐
│ Athena                                       │
├──────────────┬───────────────────────────────┤
│ Knowledge    │                               │
│              │       Main Content            │
│ System       │                               │
│ Design       │                               │
│              │                               │
│ Caching      │                               │
│ Sharding     │                               │
│              │                               │
├──────────────┴───────────────────────────────┤
│ Progress / Session / Status                   │
└──────────────────────────────────────────────┘
```

---

# 45. Dashboard

Futuro:

* progresso;
* score;
* tempo estudado;
* temas dominados;
* gaps;
* histórico;
* evolução.

Exemplo:

```text
System Design

Caching           ████████░░ 80%
Load Balancing    ██████░░░░ 60%
Sharding          ███░░░░░░░ 30%
Queues            █████████░ 90%
```

---

# 46. Knowledge Explorer

Interface para navegar pela base:

```text
Knowledge
│
├── System Design
│   ├── Caching
│   ├── Sharding
│   └── Load Balancing
│
├── Algorithms
│   ├── Arrays
│   ├── Graphs
│   └── Dynamic Programming
│
└── Distributed Systems
    ├── Consensus
    ├── Replication
    └── Partitioning
```

---

# 47. Knowledge Graph

Funcionalidade futura.

As relações começam na reconciliação da Fase 2 e usam IDs de Knowledge Items. O
grafo futuro reutiliza essas relações e acrescenta semânticas direcionais; nomes de
conceitos não são chaves de integridade.

Relacionamentos:

```text
Caching
   │
   ├── Redis
   ├── TTL
   ├── Eviction
   └── Cache Stampede
```

O objetivo é permitir visualização das relações entre conceitos.

---

# 48. Local-First

Sempre que possível, o Athena deve priorizar dados locais.

Dados locais:

* Knowledge Base;
* notas;
* histórico;
* progresso;
* cache;
* configurações.

Internet necessária principalmente para:

* LLM;
* embeddings remotos;
* web search;
* sincronização futura.

Isso permite funcionalidades parcialmente offline.

---

# 49. Persistência

Inicialmente:

```text
SQLite
```

Possíveis entidades:

```text
topics
knowledge_items
knowledge_chunks
knowledge_evidence
knowledge_item_evidence
knowledge_reconciliation_proposals
knowledge_item_relations
knowledge_item_revisions
message_sources
notes
sessions
questions
answers
evaluations
progress
usage
cached_responses
```

---

# 50. Diretório Local

Exemplo:

```text
~/.athena/

├── config.yaml
├── athena.db
├── vectors/
├── cache/
├── knowledge/
└── logs/
```

A estrutura poderá mudar conforme a implementação.

---

# 51. Core Package Structure

Sugestão:

```text
athena/
│
├── cmd/
│   └── athena/
│
├── internal/
│   │
│   ├── domain/
│   │   ├── study/
│   │   ├── challenge/
│   │   ├── interview/
│   │   ├── evaluation/
│   │   ├── knowledge/
│   │   └── architecture/
│   │
│   ├── application/
│   │   ├── study/
│   │   ├── challenge/
│   │   ├── interview/
│   │   ├── ingest/
│   │   └── knowledge/
│   │
│   ├── llm/
│   │   ├── provider.go
│   │   ├── service.go
│   │   ├── router.go
│   │   ├── budget.go
│   │   └── models.go
│   │
│   ├── knowledge/
│   │   ├── retrieval.go
│   │   ├── ingestion.go
│   │   ├── promotion.go
│   │   └── embeddings.go
│   │
│   ├── infrastructure/
│   │   ├── openrouter/
│   │   ├── sqlite/
│   │   ├── vectorstore/
│   │   └── filesystem/
│   │
│   └── interfaces/
│       └── desktop/
│
└── frontend/
    ├── src/
    └── ...
```

---

# 52. Interfaces

O Core deve expor casos de uso.

Exemplos:

```go
type StudyService interface {
    Start(ctx context.Context, req StudyRequest) (StudySession, error)
}

type KnowledgeService interface {
    Search(ctx context.Context, req SearchRequest) ([]KnowledgeItem, error)
    Ingest(ctx context.Context, req IngestRequest) error
}

type InterviewService interface {
    Start(ctx context.Context, req InterviewRequest) (InterviewSession, error)
}
```

---

# 53. Desktop ↔ Go

A UI deverá chamar o Core.

```text
React
  ↓
Wails Binding
  ↓
Go Application Service
  ↓
Domain
```

Nunca:

```text
React
  ↓
OpenRouter
```

ou:

```text
React
  ↓
SQLite
```

A UI deve permanecer desacoplada da infraestrutura.

---

# 54. Configuração

Exemplo inicial:

```yaml
llm:
  provider: openrouter

  models:
    premium:
      - anthropic/...
    cheap:
      - google/...
    free:
      - openrouter/free

knowledge:
  auto-save: false

storage:
  database: ~/.athena/athena.db
```

---

# 56. Princípios de Design

## Core-first

O Core é independente da interface.

## Knowledge-first

Buscar conhecimento existente antes de chamar o LLM.

## Local-first

Dados do usuário devem permanecer locais inicialmente.

## Provider abstraction

O Core não conhece detalhes do OpenRouter.

## Semantic model

Diagramas devem possuir representação semântica.

## Structured output

Avaliações devem ser estruturadas.

## Human approval

Conhecimento gerado pelo LLM não deve automaticamente ser tratado como verdade.

## Extensibility

Novas interfaces e providers devem poder ser adicionados sem alterar o domínio.

---

# 57. Roadmap

## Fase 1 — Core MVP

* Go;
* SQLite;
* onboarding interview;
* user profile;
* personalização básica;
* Study;
* Challenge;
* LLMService;
* OpenRouter;
* streaming;
* tracking básico.

---

## Fase 2 — Desktop MVP

* Wails;
* React;
* TypeScript;
* Study UI;
* Challenge UI;
* Knowledge Explorer;
* histórico.

---

## Fase 3 — Knowledge Engine

* ingestão Markdown;
* chunking;
* embeddings;
* vector search;
* RAG;
* Knowledge Items;
* Knowledge lifecycle;
* response cache.

---

## Fase 4 — Learning Intelligence

* avaliação estruturada;
* gap detection;
* progress tracking;
* Knowledge Promotion;
* flashcards com geração automática via LLM;
* spaced repetition (SM-2);
* revisão;
* recomendações.

---

## Fase 5 — Interview

* System Design Interview;
* timer;
* avaliação;
* score;
* histórico.

---

## Fase 6 — Advanced Knowledge

* Knowledge Graph;
* relacionamentos entre conceitos;
* visualização;
* múltiplas fontes;
* web knowledge.

---

## Fase 7 — Whiteboard

* editor visual;
* arquitetura;
* nodes;
* edges;
* semantic model;
* diagram evaluation;
* architecture score.

---

## Fase 8 — Algorithm Mode

* problemas;
* editor;
* execução;
* testes;
* sandbox;
* análise de complexidade;
* coding interviews.

---

# 58. Futuro

O Athena poderá evoluir para:

```text
                    ATHENA
                       │
        ┌──────────────┼──────────────┐
        │              │              │
     Knowledge      Learning       Practice
        │              │              │
        │              │              │
        └──────────────┼──────────────┘
                       │
                  AI Engine
                       │
        ┌──────────────┼──────────────┐
        │              │              │
       RAG          Evaluation      LLM
        │              │              │
        │              │         OpenRouter
        │              │
        └──────────────┼──────────────┘
                       │
                 User Progress
```

O produto final deverá permitir que o usuário:

1. importe suas notas;
2. construa uma base de conhecimento;
3. estude novos conceitos;
4. pratique;
5. responda perguntas;
6. faça entrevistas;
7. desenhe arquiteturas;
8. receba avaliações;
9. identifique gaps;
10. reforce conceitos;
11. acompanhe sua evolução.

---

# 59. Critério de Sucesso

O Athena será bem-sucedido se conseguir criar um ciclo de aprendizado em que:

```text
O usuário aprende
       ↓
O Athena registra conhecimento
       ↓
O usuário pratica
       ↓
O Athena avalia
       ↓
O Athena identifica gaps
       ↓
O usuário reforça os gaps
       ↓
A Knowledge Base evolui
       ↓
O próximo estudo fica mais personalizado
```

O objetivo final não é simplesmente fornecer respostas de IA.

> **O objetivo é construir uma memória técnica pessoal e utilizá-la para melhorar continuamente a capacidade do usuário.**

---

# 60. Visão Final

Athena começa como:

> **uma ferramenta de aprendizado técnico**

e evolui para:

> **um ambiente pessoal de conhecimento, prática e avaliação para desenvolvedores.**

Sua arquitetura deve permitir a evolução de:

```text
Study
  ↓
Knowledge
  ↓
RAG
  ↓
Practice
  ↓
Evaluation
  ↓
Interviews
  ↓
System Design
  ↓
Whiteboard
  ↓
Algorithms
  ↓
Personal Technical Learning Environment
```

O diferencial central do produto é:

> **Knowledge → Practice → Evaluation → Gaps → Reinforcement → Knowledge**

---

# 61. Stack Inicial Recomendada

```text
Language:
Go

Desktop:
Wails

Frontend:
React + TypeScript

Database:
SQLite

Vector Search:
Local vector storage

LLM Gateway:
OpenRouter

Architecture:
Clean / Hexagonal principles

Communication:
Wails bindings

Future:
Whiteboard
Knowledge Graph
Algorithm Sandbox
```

O Athena deve evitar dependência de infraestrutura própria de IA no início.

Não utilizar Ollama como requisito de infraestrutura.

A inferência será realizada remotamente através do OpenRouter.

A Knowledge Base, histórico, notas e cache devem permanecer localmente no dispositivo do usuário sempre que possível.

---

# 62. Regra de Ouro

> **O Athena Core deve saber sobre conhecimento, aprendizado e avaliação.**
>
> **Ele não deve saber se está sendo executado por um Desktop ou outra interface futura.**
>
> **Ele também não deve saber se a inteligência vem de Claude, Gemini ou outro modelo.**
>
> **Para o Core, existe apenas um LLM Provider. A implementação inicial desse provider é o OpenRouter.**

Essa separação permitirá que o Athena evolua sem precisar ser reescrito a cada nova interface, modelo ou tecnologia visual.

---

# 63. User Profile

O perfil do usuário é a base da personalização do Athena.

Ele é criado durante o onboarding e pode ser editado nas configurações.

```go
type UserProfile struct {
    Name           string   `json:"name"`
    AssistantName  string   `json:"assistant_name"`
    Area           string   `json:"area"`
    Specialty      string   `json:"specialty"`
    ExperienceLevel string  `json:"experience_level"`
    Goals          []string `json:"goals"`
    StudyStyle     string   `json:"study_style"`
}
```

Campos:

```text
name
  Nome do usuário

assistant_name
  Apelido configurável do assistente
  Default: "Athena"

area
  Área de conhecimento principal
  Exemplos: direito, medicina veterinária, tecnologia, concursos públicos

specialty
  Foco específico dentro da área
  Exemplos: OAB, clínica de pequenos animais, system design, INSS

experience_level
  beginner | intermediate | advanced

goals
  Exemplos: certificação, entrevista de emprego, aprendizado contínuo, aprovação em concurso

study_style
  Exemplos: socrático, direto, baseado em desafios
```

---

# 64. Onboarding Interview

Na primeira execução, o Athena conduz uma entrevista conversacional para construir o perfil do usuário.

A entrevista é guiada pelo LLM e adapta as perguntas conforme as respostas anteriores.

Fluxo:

```text
Primeira execução
       ↓
Tela de boas-vindas
       ↓
Entrevista conversacional (LLM)
       ↓
Confirmação do perfil
       ↓
Perfil salvo
       ↓
Experiência personalizada
```

Exemplo de interação:

```text
Assistente:
"Olá! Sou o Athena, seu ambiente pessoal de aprendizado.
Antes de começarmos, quero entender um pouco sobre você.

Como você se chama?"

Usuário:
"Felipe"

Assistente:
"Olá, Felipe! Com o que você trabalha ou estuda atualmente?"

Usuário:
"Sou advogado e estou me preparando para a OAB"

Assistente:
"Entendido. Você já tem experiência com a prova ou é sua primeira tentativa?"

...

Assistente:
"Perfeito. Como você prefere estudar?
[1] Explicações guiadas
[2] Desafios práticos
[3] Simulação de provas"
```

Ao final:

```text
Assistente:
"Ótimo, Felipe! Seu perfil está pronto.

Área:       Direito
Foco:       OAB
Nível:      Intermediário
Objetivo:   Aprovação no exame da OAB

Posso te chamar de algo diferente de Athena?
Ou podemos começar assim mesmo."
```

O perfil pode ser editado posteriormente nas configurações do desktop.

---

# 65. Personalization Engine

O perfil do usuário é injetado como contexto em todas as interações com o LLM.

Isso permite que o Athena adapte:

* tom e profundidade das explicações ao `experience_level`;
* exemplos contextualizados à `area` e `specialty` do usuário;
* sugestões de tópicos alinhadas aos `goals`;
* critérios de avaliação ajustados ao domínio;
* recomendações de estudo personalizadas.

Exemplo de contexto injetado no prompt:

```text
User Profile:
- Name: Felipe
- Area: Direito
- Specialty: OAB
- Experience Level: Intermediate
- Goals: Aprovação no exame da OAB
- Study Style: Direto
```

Exemplo de diferença na resposta:

```text
[Sem perfil]
"Cache é uma técnica de armazenamento temporário..."

[Com perfil — advogado]
"Pense no cache como a jurisprudência que você já conhece:
antes de pesquisar do zero, você consulta o que já foi decidido..."
```

O Personalization Engine não é um módulo separado.

Ele é a consequência de o Core sempre incluir o `UserProfile` como contexto ao construir requisições ao LLM.

---

# 66. Assistant Name

O usuário pode configurar como quer chamar o assistente.

O nome padrão é:

```text
Athena
```

O nome configurado aparece em:

* saudações na UI;
* mensagens do assistente;
* notificações;
* tela de progresso.

Exemplo com nome customizado:

```text
[Nome padrão]
Athena > "Olá, Felipe! Vamos estudar hoje?"

[Nome customizado: "Mentor"]
Mentor > "Olá, Felipe! Vamos estudar hoje?"
```

A configuração é feita:

* durante o onboarding;
* nas configurações do desktop a qualquer momento.

O `AssistantName` faz parte do `UserProfile` e é persistido localmente.

---

# 67. Distribuição

O Athena é um aplicativo desktop cross-platform.

O foco inicial de distribuição é **Windows e Linux**. macOS será suportado posteriormente.

---

## 67.1 Artefatos por sistema operacional

```text
Windows  → .exe (installer) ou .msi
Linux    → .AppImage / .deb / .rpm
macOS    → .dmg + .app (futuro)
```

O Wails compila para cada plataforma a partir do mesmo código Go + React.

A webview utilizada é nativa de cada OS — sem Chromium embutido, sem Electron. O binário final é leve (tipicamente 20–50 MB).

---

## 67.2 Windows

### Build

O Wails gera um instalador `.exe` ou `.msi` nativamente.

### Code Signing

Sem assinatura, o SmartScreen do Windows exibe um aviso de "app desconhecido" na primeira execução.

Não impede a instalação, mas impacta a experiência do usuário.

Opções de certificado:

```text
OV (Organization Validation)  → ~$200–300/ano
EV (Extended Validation)       → ~$400–500/ano — elimina o aviso do SmartScreen imediatamente
```

O EV é o recomendado quando o produto estiver em distribuição pública.

### Canais de distribuição

```text
Site próprio      → download direto do .exe / .msi
GitHub Releases   → release público, gratuito
winget            → Windows Package Manager (Microsoft)
                    comando: winget install athena
Chocolatey        → gerenciador de pacotes da comunidade Windows
```

---

## 67.3 Linux

### Formatos

```text
.AppImage   → universal, sem instalação, roda em qualquer distro
.deb        → Debian, Ubuntu, Pop!_OS e derivados
.rpm        → Fedora, RHEL, openSUSE
```

O `.AppImage` é o formato recomendado inicialmente por não depender de distro específica.

### Code Signing

Não obrigatório. GPG signing dos pacotes é boa prática para garantir integridade.

### Canais de distribuição

```text
Site próprio      → download direto
GitHub Releases   → release público
Flathub           → loja universal Flatpak, amplo alcance
Snap Store        → loja da Canonical
AUR               → Arch Linux, mantido pela comunidade
```

---

## 67.4 macOS (futuro)

### Requisitos

* Apple Developer account — $99/ano
* Code signing obrigatório — sem isso, o Gatekeeper bloqueia o app
* Notarização via `xcrun notarytool` — Apple valida o binário antes da distribuição

Sem esses passos, o usuário recebe:

```text
"Não é possível abrir porque não pode ser verificado"
```

### Canais de distribuição

```text
Site próprio      → .dmg com download direto
GitHub Releases   → release público
Homebrew Cask     → brew install --cask athena
Mac App Store     → possível, mas sandboxing pode conflitar com acesso local ao filesystem
```

---

## 67.5 Pipeline de build (GitHub Actions)

O processo de build e publicação é automatizado via CI.

A cada nova tag de release (ex: `v1.0.0`):

```text
Push da tag
    ↓
GitHub Actions dispara
    ↓
Build matrix: [windows, linux, macos]
    ↓
Assinar binários (Windows + macOS)
    ↓
Empacotar artefatos
    ↓
Publicar no GitHub Releases automaticamente
```

Estrutura do workflow:

```yaml
strategy:
  matrix:
    os: [ubuntu-latest, windows-latest, macos-latest]
```

---

## 67.6 Auto-Update

O app deve ser capaz de se atualizar automaticamente.

O usuário não pode depender de baixar manualmente uma nova versão.

Fluxo:

```text
App inicia
    ↓
Verifica versão disponível (GitHub Releases API)
    ↓
Nova versão disponível?
    ↓
Notifica o usuário
    ↓
Usuário confirma → Download → Instala → Reinicia
```

A verificação deve ser silenciosa e não bloquear a inicialização do app.

---

## 67.7 Canais de pagamento e download

O Athena pode utilizar um serviço de pagamento que atua como **merchant of record**, simplificando impostos internacionais.

```text
Paddle   → recomendado para apps desktop pagos
           lida com impostos de cada país automaticamente
           emite licença após pagamento

Gumroad  → alternativa simples para início
           menor burocracia, mas sem merchant of record
```

Fluxo do usuário:

```text
Site do Athena
    ↓
Escolhe o plano
    ↓
Pagamento via Paddle / Gumroad
    ↓
Recebe link de download + chave de licença
    ↓
Instala o app
    ↓
Ativa com a chave
```

---

## 67.8 Licença

A ativação da licença deve funcionar **offline** após a primeira validação.

Isso é consistente com o princípio local-first do produto.

```text
Primeira execução
    ↓
Valida chave online (uma única vez)
    ↓
Armazena validação localmente (assinatura criptográfica)
    ↓
Execuções seguintes → validação local, sem internet necessária
```

---

# 68. Licença e Comercialização

---

## 68.1 Modelo de Licença

O Athena adota um modelo de **trial gratuito de 7 dias** com acesso completo, seguido de planos pagos baseados em funcionalidades.

Não há plano gratuito permanente. O trial é a porta de entrada.

```text
Trial (7 dias)
└── acesso completo a todas as funcionalidades disponíveis

Após o trial → escolha obrigatória de plano
```

### Planos disponíveis

Cada plano está disponível nas modalidades **mensal** e **anual**.

```text
┌─────────────────────────────────────────────────────────────┐
│  ESSENCIAL                                                  │
│                                                             │
│  ✅ Onboarding e perfil personalizado                       │
│  ✅ Study Mode                                              │
│  ✅ Knowledge Base básica                                   │
│  ✅ Importação de notas (Markdown)                          │
│  ✅ Modelos LLM cheap e free                                │
│  ❌ Challenge Mode                                          │
│  ❌ Interview Mode                                          │
│  ❌ Gap Detection                                           │
│  ❌ Progress Tracking avançado                              │
└─────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────┐
│  PRO                                                        │
│                                                             │
│  ✅ Tudo do Essencial                                       │
│  ✅ Challenge Mode                                          │
│  ✅ Interview Mode                                          │
│  ✅ Knowledge Base ilimitada                                │
│  ✅ Gap Detection                                           │
│  ✅ Progress Tracking completo                              │
│  ✅ Acesso a modelos premium                                │
│  ✅ Knowledge Lifecycle (draft / approved / deprecated)     │
└─────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────┐
│  EXPERT                                                     │
│                                                             │
│  ✅ Tudo do Pro                                             │
│  ✅ Whiteboard / Architecture Mode (quando disponível)      │
│  ✅ Algorithm Mode (quando disponível)                      │
│  ✅ Knowledge Graph (quando disponível)                     │
│  ✅ Acesso antecipado a novas funcionalidades               │
│  ✅ Suporte prioritário                                     │
└─────────────────────────────────────────────────────────────┘
```

---

## 68.2 Login e Criação de Conta

O Athena exige uma conta para funcionar.

A conta serve para:

* vincular a licença ao usuário;
* permitir recuperação de acesso em caso de reinstalação;
* preparar o produto para futuras funcionalidades de sync.

O perfil de aprendizado (UserProfile, Knowledge Base, histórico) permanece **local**. A conta gerencia apenas identidade e licença no servidor.

### Tela de entrada

Na primeira execução, o usuário vê a tela de login:

```text
┌────────────────────────────────────────┐
│                Athena                  │
│                                        │
│  Bem-vindo de volta                    │
│                                        │
│  E-mail  [                          ]  │
│  Senha   [                          ]  │
│                                        │
│         [ Entrar ]                     │
│                                        │
│  Não tem uma conta? Criar conta        │
│  Esqueceu a senha? Recuperar acesso    │
└────────────────────────────────────────┘
```

### Criação de conta

```text
┌────────────────────────────────────────┐
│            Criar sua conta             │
│                                        │
│  Nome     [                          ] │
│  E-mail   [                          ] │
│  Senha    [                          ] │
│  Confirmar[                          ] │
│                                        │
│         [ Criar conta ]                │
│                                        │
│  Já tem uma conta? Entrar              │
└────────────────────────────────────────┘
```

Após criar a conta, o usuário recebe um e-mail de confirmação.

Com a conta confirmada, o app inicia o onboarding interview (Seção 64).

---

## 68.3 Fluxo completo do novo usuário

```text
Instala o Athena
       ↓
Tela de login
       ↓
Cria conta (e-mail + senha)
       ↓
Confirma e-mail
       ↓
Plano Free ativado automaticamente
       ↓
Onboarding Interview
       ↓
Perfil salvo localmente
       ↓
Experiência personalizada
```

Para usuário que já tem conta:

```text
Instala o Athena (novo dispositivo ou reinstalação)
       ↓
Tela de login
       ↓
E-mail + senha
       ↓
Licença validada
       ↓
Onboarding (se perfil local não existir)
       ↓
Experiência restaurada
```

---

## 68.4 Ativação do Plano Pro

O upgrade para o plano Pro é feito dentro do próprio app ou pelo site.

```text
App → Configurações → Plano → Fazer upgrade
       ↓
Redireciona para checkout (Paddle)
       ↓
Pagamento realizado
       ↓
Paddle notifica o servidor do Athena (webhook)
       ↓
Licença Pro vinculada à conta
       ↓
App detecta upgrade no próximo check
       ↓
Funcionalidades Pro desbloqueadas
```

---

## 68.5 Validação de Licença

A validação ocorre na inicialização do app.

```text
App inicia
    ↓
Verifica token de sessão local
    ↓
Token válido? → usa cache local
    ↓
Token expirado? → valida online silenciosamente
    ↓
Sem internet? → usa última validação cacheada (grace period de 7 dias)
```

O app nunca bloqueia o usuário imediatamente por falta de conexão.

---

## 68.6 Backend mínimo necessário

O Athena é local-first, mas precisa de um backend leve para gerenciar contas e licenças.

Responsabilidades do servidor:

```text
Autenticação     → registro, login, recuperação de senha
Licenças         → plano do usuário, status, validade
Webhooks         → recebe eventos do Paddle (pagamento, cancelamento)
```

O servidor **não** armazena:

```text
Knowledge Base
Notas do usuário
Histórico de estudo
Perfil de aprendizado
```

Esses dados permanecem exclusivamente no dispositivo do usuário.

---

## 68.7 Preços sugeridos

Cada plano oferece desconto na modalidade anual (equivalente a 2 meses grátis).

```text
                  Mensal        Anual           Anual (por mês)
                  ──────        ─────           ───────────────
Trial             grátis        —               —
Essencial         R$ 19/mês     R$ 152/ano      R$ 12,67/mês
Pro               R$ 39/mês     R$ 312/ano      R$ 26/mês
Expert            R$ 69/mês     R$ 552/ano      R$ 46/mês
```

Equivalência aproximada em dólar:

```text
Essencial   ~$3,50/mês   ~$28/ano
Pro         ~$7/mês      ~$56/ano
Expert      ~$12/mês     ~$99/ano
```

O usuário traz sua própria chave do OpenRouter — os custos de LLM não estão incluídos em nenhum plano.

Isso mantém o custo operacional do Athena baixo e previsível, independente do volume de uso de cada usuário.

### Trial para plano pago

Ao fim do trial de 7 dias, o usuário escolhe o plano e informa os dados de pagamento.

```text
Trial encerra
    ↓
Notificação: "Seu trial acabou. Escolha um plano para continuar."
    ↓
Tela de planos (Essencial / Pro / Expert)
    ↓
Mensal ou Anual
    ↓
Checkout via Paddle
    ↓
Acesso liberado conforme o plano escolhido
```

Funcionalidades além do plano contratado ficam bloqueadas na UI, com indicação clara do plano necessário para desbloqueá-las.

---

# 69. Flashcards

---

## 69.1 Visão

Flashcards são uma funcionalidade nativa do Athena, integrada diretamente à Knowledge Base.

Diferente de ferramentas como Anki ou Quizlet, os flashcards do Athena não são criados isoladamente — eles são **derivados do conhecimento que o usuário já construiu**.

Isso evita o trabalho duplo de estudar e depois criar os cartões manualmente.

O ciclo é:

```text
Sessão de estudo
       ↓
Knowledge Item aprovado
       ↓
Flashcard gerado automaticamente
       ↓
Review com spaced repetition
       ↓
Performance registrada
       ↓
Gap Detection
       ↓
Reforço de estudo
```

---

## 69.2 Tipos de Cartão

O Athena suportará três tipos de flashcard:

```text
Front / Back
─────────────────────────
Frente: "O que é Cache Aside?"
Verso:  "Estratégia onde a aplicação consulta o cache antes do banco..."

Cloze (lacuna)
─────────────────────────
"Cache Aside é uma estratégia onde a ________ consulta o cache antes do banco."

Múltipla Escolha
─────────────────────────
"Qual estratégia de cache a aplicação controla diretamente?"

(A) Write Through
(B) Cache Aside ✓
(C) Read Through
(D) Write Behind
```

---

## 69.3 Geração Automática

O LLM gera flashcards a partir de Knowledge Items aprovados.

Fluxo:

```text
Knowledge Item aprovado
       ↓
LLM extrai conceitos-chave
       ↓
Gera cartões (Front/Back, Cloze, Múltipla Escolha)
       ↓
Cartões em estado draft
       ↓
Usuário revisa e aprova
       ↓
Cartões entram na fila de review
```

O usuário também pode criar cartões manualmente ou editar os gerados.

Modelo do cartão:

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
```

---

## 69.4 Spaced Repetition

O Athena utiliza o algoritmo **SM-2** (base do Anki) para calcular o intervalo de revisão de cada cartão.

Após cada revisão, o usuário avalia sua resposta:

```text
❌ Errei completamente   → intervalo reinicia
⚠️  Lembrei com esforço  → intervalo reduzido
✅  Lembrei facilmente   → intervalo aumenta
```

O sistema calcula automaticamente quando cada cartão deve ser revisado novamente.

```text
Review 1  → próximo em 1 dia
Review 2  → próximo em 3 dias
Review 3  → próximo em 7 dias
Review 4  → próximo em 16 dias
...
```

Os dados de performance são armazenados localmente.

```go
type FlashcardReview struct {
    FlashcardID string
    ReviewedAt  time.Time
    Quality     int // 0 (errou) a 5 (perfeito)
    Interval    int // dias até próxima revisão
    EaseFactor  float64
}
```

---

## 69.5 Sessão de Revisão

O usuário pode iniciar uma sessão de revisão a qualquer momento.

O Athena prioriza automaticamente os cartões com revisão pendente.

```text
┌──────────────────────────────────────────────┐
│  Revisão de hoje                             │
│                                              │
│  12 cartões pendentes                        │
│                                              │
│  ┌──────────────────────────────────────┐    │
│  │                                      │    │
│  │   O que é Cache Aside?               │    │
│  │                                      │    │
│  └──────────────────────────────────────┘    │
│                                              │
│            [ Ver resposta ]                  │
│                                              │
│  Tópico: System Design · Cache               │
└──────────────────────────────────────────────┘
```

Após ver a resposta:

```text
┌──────────────────────────────────────────────┐
│  [ ❌ Errei ]  [ ⚠️ Com esforço ]  [ ✅ Fácil ] │
└──────────────────────────────────────────────┘
```

A sessão exibe ao final um resumo de performance:

```text
Revisão concluída

Total revisado:   12
Acertos:          9 (75%)
Com dificuldade:  2
Erros:            1

Próxima revisão agendada: amanhã (5 cartões)
```

---

## 69.6 Integração com a Knowledge Base

Os flashcards são cidadãos da Knowledge Base — não entidades separadas.

```text
Knowledge Base
│
├── Knowledge Items
│   └── Flashcards derivados
│
├── User Notes
│   └── Flashcards gerados de notas importadas
│
└── Imported Documents
    └── Flashcards gerados de documentos
```

Um flashcard sempre referencia o Knowledge Item ou a nota de origem.

Isso permite navegar do cartão para o conceito completo durante a revisão.

---

## 69.7 Integração com Gap Detection

A performance nos flashcards alimenta diretamente o Gap Detection.

```text
Taxa de erros alta em cartões de "Sharding"
       ↓
Gap Detection registra dificuldade recorrente
       ↓
Athena sugere sessão de estudo focada
```

```text
Direito Civil

✅ Contratos          (92% de acerto)
⚠️  Responsabilidade  (61% de acerto)
❌  Posse             (38% de acerto)

Sugestão: revisar Posse e Propriedade
```

---

## 69.8 Flashcards no Roadmap

Flashcards serão introduzidos na **Fase 4 — Learning Intelligence**, após a Knowledge Base estar estruturada.

```text
Fase 4
├── avaliação estruturada
├── gap detection
├── progress tracking
├── Knowledge Promotion
├── flashcards (geração automática + SM-2)
└── recomendações
```
