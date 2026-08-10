# Phase 6.1 — Audio Providers (STT + TTS)

## Goal

Infrastructure layer for speech-to-text and text-to-speech, enabling voice-driven interviews.

> This phase is an intentional exception to the "OpenRouter only" rule — audio APIs are not available via OpenRouter.

## Interface

```go
type AudioProvider interface {
    Transcribe(ctx context.Context, audio []byte) (string, error)
    Speak(ctx context.Context, text string) ([]byte, error)
}
```

## Providers

| Role | Primary | Fallback |
|---|---|---|
| STT | OpenAI Whisper API | Browser Web Speech API |
| TTS | OpenAI TTS | ElevenLabs (higher quality, paid) |

## Tasks

- [ ] `internal/infrastructure/audio/` — `AudioProvider` implementations
- [ ] `WhisperProvider`: POST audio bytes to Whisper API → return transcript
- [ ] `OpenAITTSProvider`: POST text → return MP3 bytes
- [ ] Provider selected via `config.yaml` (`audio.stt_provider`, `audio.tts_provider`)
- [ ] API keys for audio providers stored in `config.yaml` (separate from OpenRouter key)

## Acceptance Criteria

- `AudioProvider.Transcribe` returns accurate text for a short English or Portuguese audio clip
- `AudioProvider.Speak` returns playable audio bytes for a given string
- Switching provider in config takes effect without restarting the app
- Missing audio API key returns a descriptive error surfaced in the UI
