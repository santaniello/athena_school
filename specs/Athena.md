Criar uma **CLI em Go** para estudo ativo com IA, focada em:
- aprender qualquer tema (ex: system design)
- praticar com desafios
- receber feedback automático
- usar suas próprias notas como base.

# Especificação Completa de Produto

---

# 🎯 Visão do Produto

Athena é uma ferramenta **CLI-first** focada em aprendizado ativo com IA para desenvolvedores.

Ela permite:
- Aprender conceitos técnicos (ex: system design, algoritmos)    
- Praticar com desafios reais    
- Simular entrevistas técnicas    
- Receber feedback estruturado    
- Utilizar suas próprias notas como base de conhecimento    

---

# 🚀 Proposta de Valor

Athena é um:

> 🧠 **ambiente de aprendizado técnico ativo, personalizado e mensurável**

### Diferenciais:
- CLI-first (dev-native)    
- Aprendizado ativo (não passivo)    
- Simulação de entrevistas reais    
- Uso de notas pessoais (RAG)    
- Controle de fonte de conhecimento    
- Arquitetura multi-provider    
- Evolução para GUI    

---

# 👤 Público-Alvo

- Desenvolvedores backend (junior → sênior)    
- Pessoas estudando system design    
- Candidatos a entrevistas técnicas    
- Usuários de Obsidian / notas estruturadas    

---

# 🧩 Funcionalidades Principais

## 📚 Study Mode

```bash
athena study system-design
athena study system-design caching
```

- Explicação guiada    
- Perguntas interativas    
- Feedback sobre entendimento    

---

## 🧪 Challenge Mode

```bash
athena challenge system-design
athena challenge system-design caching
```

- Problemas práticos    
- Avaliação estruturada    

---

## 🎤 Interview Mode

```bash
athena interview system-design
athena interview system-design caching --source strict-notes
```

- Simulação de entrevista    
- Tempo controlado    
- Score final    

---

## 📝 Notes Integration (RAG)

```bash
athena ingest ./notes
```

- Indexa notas    
- Busca semântica    
- Base de conhecimento personalizada    

---

# 🧠 Source Modes (Controle de Conhecimento)

```bash
athena study caching --source notes
athena study caching --source web
athena interview system-design --source strict-notes
```

|Mode|Descrição|
|---|---|
|notes|Apenas notas do usuário|
|web|Notas + conhecimento geral|
|strict-notes|Apenas notas (modo restrito)|

---

# 🧩 Temas e Subtemas

## Estrutura

```text
tema
 ├── subtema
```

## Exemplo

```text
system-design
 ├── caching
 ├── load-balancing
 ├── sharding
```

---

## Uso

```bash
athena study system-design
athena study system-design caching
```

---

## Comportamento

- Tema → visão geral    
- Subtema → profundidade   

---

## Sugestão automática

```text
Subtopics:
[1] caching
[2] load-balancing
```

---

## Detecção de gaps

```text
⚠️ Dificuldade em:
- caching

Sugestão:
athena study system-design caching
```

---

# ⚙️ Providers de IA

## Inicial

- Ollama    

## Futuro

- OpenAI    
- Claude    
- Gemini    

---

## Configuração

```bash
athena config set provider ollama
athena config set model llama3
```

---

## Uso por comando

```bash
athena study caching --provider ollama
athena interview system-design --provider openai
```

---

# 🧠 Avaliação de Respostas

Critérios:

- Clareza    
- Organização    
- Escalabilidade    
- Trade-offs    
- Profundidade técnica    

---

## Exemplo

```text
✅ Strengths:
- Boa decomposição

⚠️ Improvements:
- Faltou cache

⭐ Score: 7/10
```

---

# 🖥️ Interfaces

## 🟢 CLI (MVP)

- Interface principal    
- Scriptável    
- Rápida    

---

## 🟡 TUI

- Interface interativa no terminal    
- Melhor UX    

---

## 🔵 GUI (Futuro)

### Objetivo

- Melhor visualização    
- Maior acessibilidade    

---

## Funcionalidades da GUI

### 📊 Dashboard

- Progresso    
- Scores    
- Histórico   

---

### 🧠 Mapa de conhecimento

```text
system-design
 ├── caching        ✅
 ├── load-balancing ⚠️
 ├── sharding       ❌
```

---

### 🎤 Entrevistas visuais

- Timer    
- Chat em tempo real    

---

### 🧩 Whiteboard

- Diagramas (Mermaid / visual)    
- Análise de arquitetura    

---

# 🏗️ Arquitetura

## Princípio

> Interface desacoplada do core

---

## Estrutura

```text
CLI / GUI
   ↓
Use Cases
   ↓
LLM + RAG
```

---

## Regras

- Sem lógica na UI    
- Core reutilizável    
- Providers plugáveis    

---

# 🔌 LLM Abstraction

## Interface

```go
type LLMProvider interface {
    Chat(ctx context.Context, req ChatRequest) (ChatResponse, error)
    Embeddings(ctx context.Context, input []string) ([]Embedding, error)
}
```

---

## Benefícios

- Troca de provider sem impacto    
- Extensível    
- Testável    

---

# 🧪 Algorithm Mode (Último Nível)

## Comando

```bash
athena algo two-sum
```

---

## Funcionalidades

### 📌 Problemas

- Estilo entrevista    
- Constraints definidas    

---

### 💻 Execução

```bash
athena algo two-sum --run solution.go
```

---

### 🧠 Avaliação

- Correção    
- Complexidade    
- Qualidade   

---

### 📊 Feedback

```text
✅ Passed tests

⚠️ Melhorar para O(n)

⭐ Score: 7/10
```

---

### 🧪 Testes

```text
❌ Failed:
Input: [3,3]
Expected: [0,1]
```

---

## 📈 Dificuldade

```bash
athena algo lru-cache --difficulty medium
```

---

## 🎤 Coding Interview

```bash
athena interview algorithms
```

---

# 📊 Tracking

- Score por tema    
- Evolução    
- Tempo de resposta    
- Taxa de acerto   

---

# 🚀 Roadmap

## 🟢 Fase 1

- CLI básica    
- Study    
- Challenge    
- Ollama   

---

## 🟡 Fase 2

- Interview    
- Source Modes    
- Subtemas    

---

## 🔴 Fase 3

- RAG (notas)    
- TUI    

---

## 🔵 Fase 4

- GUI    
- Whiteboard    
- Analytics    

---

## 🟣 Fase Final

- Algorithm Mode    
- Execução de código    
- Coding interviews    

---

# ✅ Critérios de Sucesso

- Estudo sem sair do terminal    
- Feedback útil para entrevistas    
- Uso real de notas    
- Evolução mensurável    
- Arquitetura extensível    

---

# 💬 Resumo Final

Athena evolui de:

> ⚡ CLI de estudo

Para:

> 🧠 Plataforma completa de aprendizado técnico

Unificando:
- System Design    
- Entrevistas    
- Algoritmos    

Com um diferencial claro:

> **aprendizado ativo, personalizado e controlado**