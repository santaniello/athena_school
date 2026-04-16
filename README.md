# athena_school

Athena is an active learning CLI for developers. It guides you through technical topics with explanations, questions, and feedback.

## Installation

```bash
go install github.com/fsantaniello/athena_school/cmd/athena@latest
```

## Running Ollama with Docker

The easiest way to run Ollama locally is via Docker Compose:

```bash
# Start Ollama in the background
docker compose up -d

# Pull the default model (only needed once — data is persisted in a Docker volume)
docker compose exec ollama ollama pull llama3
```

Ollama will be available at `http://localhost:11434`, which is the default host used by `athena`.

To point `athena` at a different Ollama instance, set the `OLLAMA_HOST` environment variable:

```bash
OLLAMA_HOST=http://my-server:11434 athena study system-design
```

Or persist it permanently:

```bash
athena config set ollama.host http://my-server:11434
```

## Configuration

```bash
# View current configuration
athena config get

# Set LLM provider and model
athena config set provider ollama
athena config set model llama3
athena config set ollama.host http://localhost:11434
```

Configuration is stored at `~/.config/athena/config.yaml`.

## Commands

### `athena study <topic> [subtopic]`

Start an interactive guided study session. Athena explains the topic, asks a question, and gives structured feedback on your answer.

```bash
# Study a top-level topic
athena study system-design

# Focus on a subtopic
athena study system-design caching

# Override provider and model for this session
athena study system-design caching --provider ollama --model llama3
```

**Session flow:**
1. Athena presents an explanation of the topic
2. Athena asks one question to test your understanding
3. You type your answer (press Enter on a blank line to submit)
4. Athena evaluates your answer with structured feedback (Strengths, Improvements, Score)
5. Choose to continue with another question or exit

Type `exit` at the answer prompt to end the session early.

### `athena config get`

Print the current configuration.

### `athena config set <key> <value>`

Set a configuration value. Valid keys: `provider`, `model`, `ollama.host`.

### `athena version`

Print the current version.
