# Athena — Progress
## Acompanhamento de Aprendizagem por Folder

**Status:** Refinada (2026-11-23). Detalhamento técnico em [`specs/phases/phase-05-progress/`](specs/phases/phase-05-progress/).
**Módulos envolvidos:** Folders, Study Sessions (estado pedagógico e evidências do tutor), Quiz, Architecture Lab.
**Referência visual:** [mockup aprovado](specs/phases/phase-05-progress/mockup/) ([versão interativa](https://claude.ai/artifact/8ADzEMmFVyFDVbUozFNmtZ)).

---

## 1. Objetivo

Criar a aba **Progress**, que mostra, por Folder, como o aluno está em cada assunto. A tela reúne os resultados dos quizzes, do Architecture Lab e as evidências observadas pelo tutor, e informa explicitamente quando não há evidência suficiente.

Ao abrir a Progress, o aluno deve conseguir responder:

- Quais assuntos estou estudando?
- O que já demonstrei entender?
- Em quais conceitos ainda tenho dificuldade?
- Como evoluí nas avaliações e nos laboratórios?
- O que estudar ou praticar em seguida?

## 2. Modelo: assunto = sessão

```text
System Design (Folder)
├── Cache (sessão = assunto)
│   ├── conceitos e evidências do tutor
│   ├── quizzes da sessão
│   └── Arch Labs da sessão
└── Paralelismo (sessão = assunto)
```

- Cada assunto corresponde a **exatamente uma sessão de estudo**. O aluno mantém uma sessão por assunto: uma sessão "Cache" na Folder System Design, nunca "Cache 1" e "Cache 2". Uma validação de unicidade pode vir no futuro.
- O nome do assunto é o tema da sessão.
- Não há entidade Topic, nem classificação por IA, nem atividade em mais de um assunto.
- **Quizzes do assunto:** todos os quizzes da sessão, sejam da oferta no chat ou gerados pelo menu.
- **Arch Labs do assunto:** os desafios gerados a partir da sessão e os vinculados a ela no chat, cada um contado uma única vez.
- **Tudo é calculado na leitura**, sem cópias:
  - mover uma sessão de Folder leva todo o histórico junto;
  - apagar a sessão remove o assunto da Progress (quizzes e labs continuam nas telas próprias);
  - nada é contado duas vezes.

## 3. Regras de avaliação

### 3.1 Nota de Quiz no card
Média, entre os quizzes da sessão que têm tentativa, da **última tentativa** de cada um (acertos ÷ total, em %). Cada quiz tem o mesmo peso. Sem nenhuma tentativa, a nota aparece como "—".

### 3.2 Nota de Arch Lab no card
Cada tentativa do lab tem duas notas:
- o **diagrama**, que mede o que o aluno sabe montar;
- a **defesa**, que mede se ele sabe justificar o que montou.

Na Progress, a nota de uma tentativa é a **média das duas**. Só contam tentativas com as duas avaliações; uma tentativa interrompida antes da defesa fica de fora. O card mostra a média, entre os desafios do assunto, da última tentativa completa de cada um.

A média é só um resumo da Progress. A regra de aprovação do Arch Lab não muda: as duas notas precisam ser ≥ a nota mínima, sem média, e os critérios obrigatórios precisam ser atendidos. Uma reprovação continua levando o assunto a "Precisa reforço".

Exemplo: diagrama 85 e defesa 40 dão **63%** e status **Precisa reforço**.

A tela indica "última tentativa", porque o catálogo do Arch Lab mostra a melhor nota.

### 3.3 Status do assunto
Verificado nesta ordem; vale o primeiro que se aplica:

| Status | Quando |
|---|---|
| **Evidências insuficientes** | Nenhum quiz respondido, nenhum lab avaliado e nenhum conceito demonstrado pelo próprio aluno na sessão. |
| **Precisa reforço** | A nota de quiz está abaixo da nota mínima; **ou** a última tentativa de algum desafio não passou; **ou** algum conceito está "para revisar". |
| **Melhorando** | Algum quiz ou desafio refeito teve a última tentativa **≥ 10 pontos** acima da anterior *do mesmo item*. |
| **Estável** | Qualquer outro caso. |

- A **nota mínima** é a configuração que já existe no Arch Lab (padrão 80), usada também para quizzes.
- "Precisa reforço" vence "Melhorando". A melhora continua visível no gráfico.
- Uma explicação dada pelo tutor nunca conta como compreensão.

## 4. Tela

**Cabeçalho**
- Título "Progress" e subtítulo.
- Três contadores:
  - **Folders** (com sessões);
  - **Assuntos**;
  - **Assuntos para revisar** (em "Precisa reforço").
- Sem filtro de período, horas estudadas ou percentual global.

**Área esquerda**
- **Folders** com sessões, na ordem da tela Folders.
- Logo abaixo, o **Athena Insights da Folder** (seção 5), recolhido na primeira frase.
- **Assuntos** da Folder selecionada, do mais recente para o mais antigo. Cada card mostra:
  - nome;
  - Quiz %;
  - Arch Lab %;
  - status.

**Área direita: assunto selecionado**
- **Atividades relacionadas:** quantidade de quizzes, Arch Labs e conceitos trabalhados. O contador de "sessões" do mockup sai, porque é sempre 1.
- **Resultados:**
  - por quiz: última nota e número de tentativas;
  - por desafio: diagrama, defesa, resultado e número de tentativas.

  Cada linha abre o item.
- **Aprendizado identificado:**
  - **Conceitos compreendidos:** conceitos demonstrados pelo aluno.
  - **Dificuldades:**
    - conceitos parcialmente entendidos ou para revisar;
    - critérios da rubrica do lab abaixo do nível 2.

    Cada dificuldade indica a origem (Tutor ou Arch Lab: desafio).
  - **Explicados, ainda não avaliados.**
  - Cada conceito pode ser expandido para mostrar as evidências que o sustentam.
- **Athena Insights do assunto** (seção 5).
- **Evolução:**
  - duas linhas: Quiz e Arch Lab (média de diagrama e defesa);
  - um ponto por tentativa, em ordem cronológica;
  - uma linha da nota mínima;
  - só aparece a partir de 2 pontos.
- Seções vazias dizem isso explicitamente ("Nenhuma avaliação ainda").

**Idioma:** a tela segue o idioma do assistente no perfil (pt/en). O item "Progress" do menu fica em inglês.

## 5. Athena Insights

Os insights existem em dois níveis: **assunto** e **Folder**.

- **Sob demanda:** a tela tem um botão "Gerar insights". Abrir a Progress nunca chama o LLM e nada roda em segundo plano.
- **Salvos:** reabrir a tela mostra o último insight na hora.
- **Desatualizado:** se surgir atividade nova, mudar o idioma ou, no caso da Folder, sessões entrarem ou saírem dela, o card mostra "Há atividades novas desde esta análise — Atualizar". A detecção não usa IA.
- **Entrada:** só evidências salvas (notas, resultados por critério, pontos fortes e fracos do lab, conceitos e evidências do tutor). A conversa nunca é relida.
  - No nível da Folder, a entrada é o resumo de cada assunto.
- **Saída:** análise e de 1 a 3 recomendações, cada uma citando as atividades (ou assuntos) que a sustentam.
- **Falha:** a tela mostra o erro e oferece "Tentar novamente"; o resto da tela continua funcionando. Uma resposta inválida mantém o insight anterior.
- **Sem evidência, sem insight:** em vez do botão, a tela orienta a estudar, fazer um quiz ou um lab.

## 6. Arquitetura

A antiga "Learning Intelligence Engine" fica reduzida ao necessário, respeitando a arquitetura hexagonal (ADR-001):

- `internal/domain/progress`: regras puras (notas, status, evolução, impressão digital e validação do insight). Não importa `study`, `quiz` nem `exercise`.
- `internal/application/progress`: lê os dados por uma porta própria, implementada por um adaptador SQLite de leitura.
- Persistência nova:
  - coluna `display_name` em `session_concepts`, para exibir "Cache-aside" em vez de "cache-aside";
  - tabelas `topic_insights` e `folder_insights`, apagadas junto com a sessão ou a Folder.
- Novas modalidades, como o Coding Lab, entram como mais uma fonte de entrada do domínio `progress`.

## 7. Incrementos

| # | Spec | Entrega |
|---|---|---|
| 5.1 | [Progress Dashboard](specs/phases/phase-05-progress/01-progress-dashboard.md) | Folders, assuntos, notas, status, contadores. Sem IA. |
| 5.2 | [Topic Detail](specs/phases/phase-05-progress/02-topic-detail.md) | Atividades, resultados, conceitos e dificuldades. |
| 5.3 | [Evolution Chart](specs/phases/phase-05-progress/03-evolution-chart.md) | Gráfico de evolução. |
| 5.4 | [Topic Insights](specs/phases/phase-05-progress/04-topic-insights.md) | Insights do assunto. |
| 5.5 | [Folder Insights](specs/phases/phase-05-progress/05-folder-insights.md) | Insights da Folder. |

## 8. Fora do escopo

- Gamificação, rankings e comparação entre alunos.
- Filtro de período e horas como métrica.
- Percentual global de domínio.
- Classificação de assuntos por IA e atividades com múltiplos assuntos.
- Geração automática de insights e eventos em segundo plano.
- Previsão de esquecimento e planos de estudo automáticos.
- Alterações no funcionamento do tutor (além do `display_name`).
