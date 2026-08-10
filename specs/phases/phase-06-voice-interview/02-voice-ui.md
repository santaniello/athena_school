# Phase 6.2 — Voice Interview UI

## Goal

User conducts a full interview by speaking; the AI listens, evaluates, and responds in voice.

## Flow

```text
User presses "Start recording"
    ↓
Audio captured until user presses "Stop"
    ↓
STT → transcript displayed in real time
    ↓
Transcript sent to Evaluation Engine
    ↓
LLM generates next question
    ↓
TTS → AI speaks the question
    ↓
Repeat until interview ends
```

## Tasks

- [ ] Microphone controls: Start / Stop recording buttons
- [ ] Live transcript displayed as the user speaks (streaming where supported)
- [ ] Visual indicators:
  - "AI is speaking" — animated waveform or spinner
  - "Your turn" — microphone icon active
- [ ] Mute / unmute option during AI speech
- [ ] Voice interview mode selectable at session start (alongside text mode)
- [ ] Full transcript saved to SQLite alongside audio session

## Acceptance Criteria

- User can complete a 3-question interview entirely by voice
- Transcript is displayed in the UI in real time during recording
- AI responses are audible and clearly distinguished from user turns
- Transcript history is saved and accessible from interview history
- Microphone permission errors are surfaced with clear instructions
