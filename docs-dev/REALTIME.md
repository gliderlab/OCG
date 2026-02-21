# OCG Realtime Documentation

> Real-time Audio, Voice, and Live Mode Features

## Overview

OCG supports multiple real-time communication modes:

1. **WebSocket Chat** - Real-time text messaging
2. **Live/Audio/Voice Mode** - Gemini Live Realtime audio conversations
3. **Audio Streaming** - PCM audio input/output via gRPC

---

## Mode Commands

### Switch Modes

| Command | Description |
|---------|-------------|
| `/live` | Switch to live audio mode (Gemini Realtime) |
| `/voice` | Switch to voice input mode |
| `/audio` | Switch to audio mode |
| `/text` | Switch back to text mode |
| `/live-audio-file <path>` | Process audio file in live mode |

### Usage Examples

```
/live                    # Start live audio conversation
/voice                   # Switch to voice input
/audio                   # Switch to audio mode
/text                    # Return to text mode
/live-audio-file /path/to/audio.wav  # Process audio file
```

---

## Gemini Live Realtime

### Configuration

```bash
# Environment
export GOOGLE_API_KEY=AI...
export GEMINI_API_KEY=AI...

# Config
google.model=gemini-2.0-flash-exp
google.realtime=true
google.realtime_voice=verse  # voice options: nova, alloy, echo, fable, onyx, shimmer
```

### Features

- **Full-duplex audio** - Simultaneous input/output
- **PCM audio** - 16-bit, 16kHz, mono
- **Function calling** - Tools work in live mode
- **VAD detection** - Voice activity detection
- **Usage tracking** - Token/usage callbacks

### Audio Format

```
Input:  PCM 16-bit, 16kHz, mono
Output: WAV format (16-bit, 16kHz, mono)
```

### API Reference

#### gRPC Audio Streaming

```protobuf
// Send audio chunk
rpc SendAudioChunk(AudioChunkArgs) returns (AudioReply);

// End audio stream
rpc EndAudioStream(AudioArgs) returns (AudioReply);

message AudioChunkArgs {
    string session_key = 1;
    bytes audio_data = 2;  // PCM data
}

message AudioArgs {
    string session_key = 1;
}

message AudioReply {
    string error = 1;
    bytes audio_data = 2;  // WAV response
}
```

#### WebSocket Audio

```javascript
// Connect with live mode
const ws = new WebSocket(
  'ws://localhost:55003/ws/chat?token=<TOKEN>&mode=live'
);

// Send PCM audio
const audioData = getPCM16Bit16KHzMono();
ws.send(JSON.stringify({
  type: 'audio',
  data: arrayBufferToBase64(audioData)
}));

// Receive audio response
ws.onmessage = (event) => {
  const data = JSON.parse(event.data);
  if (data.type === 'audio') {
    playAudio(base64ToArrayBuffer(data.data)); // WAV audio
  }
};
```

---

## WebSocket Realtime Chat

### Connection

```javascript
const ws = new WebSocket(
  'ws://localhost:55003/ws/chat?token=<TOKEN>'
);
```

### Message Types

#### Text Message

```javascript
// Send
ws.send(JSON.stringify({
  type: 'message',
  content: 'Hello!'
}));

// Receive
{
  type: 'message',
  content: 'Hello! How can I help?'
}
```

#### Tool Event

```javascript
// Receive
{
  type: 'tool_event',
  tool: 'exec',
  result: {
    success: true,
    content: '...'
  }
}
```

#### Typing Indicator

```javascript
// Receive
{
  type: 'typing',
  enabled: true
}
```

#### Audio (Live Mode)

```javascript
// Send audio frame
ws.send(JSON.stringify({
  type: 'audio_frame',
  data: base64AudioData
}));

// Receive audio response
{
  type: 'audio',
  data: base64WAVData
}
```

---

## Voice Routing

### Telegram Voice → Live Audio

OCG can route voice messages directly to live audio:

```
Telegram Voice → (no transcription) → Live Audio → Gemini Realtime → Response Audio → Telegram
```

### Configuration

```bash
# Enable voice routing
voice_routing.enabled=true
voice_routing.voice_to_live=true
```

### Flow

1. User sends voice message on Telegram
2. OCG captures raw audio (no STT)
3. Audio sent to Gemini Live Realtime
4. Response audio streamed back
5. Sent as voice message to user

---

## Live Mode Debug

### Debug Commands

```
/debug live          # Show live session debug info
```

### Debug Output

```
=== Live Session Debug ===
Session: live:telegram:123456
Provider: google
Model: gemini-2.0-flash-exp
Status: active
Audio chunks: 150
Duration: 45s
```

---

## Implementation Details

### Session Management

```go
// Per-session realtime sessions
realtimeSessions map[string]llm.RealtimeProvider

// Per-session locks (prevent concurrent requests)
realtimeSessionMu map[string]*sync.Mutex
```

### Provider Interface

```go
type RealtimeProvider interface {
    // Audio
    SendAudio(ctx context.Context, pcmData []byte) error
    EndAudio(ctx context.Context) error
    
    // Callbacks
    OnAudio(func([]byte))      // Audio output (WAV)
    OnText(func(string))       // Text output
    OnTool(func(ToolCall))     // Function calls
    OnVAD(func(bool))          // Voice activity
    OnUsage(func(Usage))      // Usage stats
}
```

### Modality Switching

```go
func modalityDirective(msg string) string {
    switch {
    case strings.HasPrefix(msg, "/live-audio-file "):
        return "force_live"
    case strings.HasPrefix(msg, "/live "):
        return "force_live"
    case strings.HasPrefix(msg, "/voice "):
        return "force_live"
    case strings.HasPrefix(msg, "/audio "):
        return "force_live"
    }
    return ""
}
```

---

## Error Handling

### Common Errors

| Error | Description | Solution |
|-------|-------------|----------|
| `no active live session` | No live session for session key | Start with `/live` |
| `google live API key not configured` | Missing API key | Set `GOOGLE_API_KEY` |
| `session already active` | Concurrent live request | Wait or use separate session |

### Fallback

If live mode fails, OCG automatically falls back to HTTP:

```
Live Error → HTTP Request → Continue conversation
```

---

## Use Cases

### Voice Conversation

```bash
# Start voice conversation
User: /live
Bot: 🎙️ Live mode activated. Speak now...

User: (sends voice)
Bot: (processes audio) What can you help me with?
```

### Audio File Processing

```bash
# Process audio file
User: /live-audio-file /path/to/meeting.wav
Bot: Processing audio...
Bot: Summary: The meeting discussed Q1 roadmap...
```

### Continuous Audio

```bash
# Multi-turn audio conversation
User: /live
Bot: 🎙️ Live mode activated.

User: (audio) What's the weather?
Bot: (audio) It's sunny and 72°F...

User: (audio) Thanks!
Bot: (audio) You're welcome!
```

---

## Performance

| Metric | Value |
|--------|-------|
| Audio latency | < 500ms |
| First audio response | < 2s |
| Concurrent sessions | Per-session locks |
| Audio format | PCM 16kHz mono |

---

## Limitations

- **Single provider** - Currently only Google Gemini supports native audio
- **Session isolation** - Each live session is isolated
- **No transcription** - Raw audio (no STT intermediate step)
- **Token limits** - Subject to Gemini usage limits
