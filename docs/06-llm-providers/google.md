# Google Gemini

Configure Google Gemini as LLM provider, including native-audio realtime.

---

## Configuration

### Environment Variables

```bash
export GOOGLE_API_KEY="..."
export GOOGLE_MODEL="gemini-2.5-flash"
export GOOGLE_BASE_URL="https://generativelanguage.googleapis.com/v1"
```

### Configuration File

```json
{
  "llm": {
    "provider": "google",
    "model": "gemini-2.5-flash",
    "temperature": 0.7
  }
}
```

---

## Realtime (Native Audio)

OCG Google realtime adapter now supports:

- text + audio streaming
- PCM audio uplink (`SendAudio`)
- audio stream end (`EndAudio`)
- finalized WAV output callback
- function calling + tool response roundtrip
- input/output transcription callbacks
- VAD/session/usage callbacks

Recommended native-audio model example:

- `models/gemini-2.5-flash-native-audio-preview-12-2025`

---

## Function Calling (Realtime)

Tool schemas are converted to Gemini function declarations (with parameters).

Flow:

1. model emits tool call
2. callback handles business logic
3. send result via `SendToolResponse`
4. model continues generation (including audio response)

---

## Available Models (Common)

| Model | Context | Description |
|-------|---------|-------------|
| gemini-2.5-flash | 1M | Fast, large context |
| gemini-2.5-pro | 2M | High capability |
| gemini-1.5-flash | 1M | Stable option |
| gemini-1.5-pro | 2M | High quality |

Realtime-native models may use different naming under `models/...`.

---

## Notes

- Gemini native-audio models should run in audio modality.
- For non-native-audio models, text modality is used.
- If websocket payload errors occur, verify model + modality match.

---

## Realtime Configurable Parameters

The following parameters can be set in `RealtimeConfig` for fine-tuning Live behavior:

### Generation
| Parameter | Type | Description |
|-----------|------|-------------|
| `temperature` | float | Randomness (0-2) |
| `topP` | float | Nucleus sampling |
| `topK` | float | Top-k sampling |
| `maxTokens` | int32 | Max output tokens |
| `seed` | int32 | Reproducibility seed |
| `includeThoughts` | bool | Include thinking in response |
| `thinkingBudget` | int32 | Thinking token budget |

### Audio / Speech
| Parameter | Type | Description |
|-----------|------|-------------|
| `voice` | string | Prebuilt voice name (e.g., "Kore") |
| `speechLanguageCode` | string | Output audio language (e.g., "en-US") |
| `inputAudioTranscription` | bool | Enable input transcription |
| `outputAudioTranscription` | bool | Enable output transcription |

### VAD (Voice Activity Detection)
| Parameter | Type | Description |
|-----------|------|-------------|
| `explicitVAD` | bool | Use explicit VAD signal |
| `autoVADDisabled` | *bool | Disable automatic VAD |
| `vadStartSensitivity` | string | Start sensitivity ("low"/"high") |
| `vadEndSensitivity` | string | End sensitivity ("low"/"high") |
| `vadPrefixPaddingMs` | int32 | Prefix padding in ms |
| `vadSilenceDurationMs` | int32 | Silence threshold in ms |
| `vadActivityHandling` | string | "start_of_activity_interrupts" / "no_interruption" |
| `vadTurnCoverage` | string | "turn_includes_only_activity" / "turn_includes_all_input" |

### Session / Context
| Parameter | Type | Description |
|-----------|------|-------------|
| `sessionResumption` | bool | Enable session resumption |
| `sessionResumptionHandle` | string | Resume handle token |
| `sessionResumptionTransparent` | bool | Transparent resumption |
| `contextWindowCompression` | bool | Enable compression |
| `contextCompressionTriggerTokens` | int64 | Trigger threshold |
| `contextCompressionTargetTokens` | int64 | Target after compression |
| `mediaResolution` | string | "low"/"medium"/"high" |

### Proactivity
| Parameter | Type | Description |
|-----------|------|-------------|
| `proactiveAudio` | *bool | Enable proactive audio |

---

## Usage Examples

### Via Code
```go
affective := true
proactive := false
cfg := llm.RealtimeConfig{
    Model:                      "models/gemini-2.5-flash-native-audio-preview-12-2025",
    Voice:                      "Kore",
    EnableAffectiveDialog:       &affective,
    ProactiveAudio:             &proactive,
    InputAudioTranscription:    true,
    OutputAudioTranscription:   true,
    VADStartSensitivity:        "low",
    VADEndSensitivity:         "low",
    VADSilenceDurationMs:       120,
    MediaResolution:            "medium",
}
```

### Via Commands (Modality Switching)
| Command | Behavior |
|--------|----------|
| `/live <text>` | Force Google Live (text input) |
| `/voice <text>` | Force Google Live (text input) |
| `/audio <text>` | Force Google Live (text input) |
| `/text <text>` | Force HTTP LLM (text input) |
| `/http <text>` | Force HTTP LLM (text input) |
| `/live-audio-file <path>` | Send audio file to Live |

### Debug Commands
- `/debug live` - Show active Live sessions and connection status
- `/debug live <sessionKey>` - Show specific session details
