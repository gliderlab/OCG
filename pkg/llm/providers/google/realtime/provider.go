package realtime

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/gliderlab/cogate/pkg/llm"
	"google.golang.org/genai"
)

type Provider struct {
	config    llm.RealtimeConfig
	client    *genai.Client
	session   *genai.Session
	connected bool
	mu        sync.RWMutex

	pcmBuf bytes.Buffer

	onAudioCb          func([]byte)
	onTextCb           func(string)
	onToolCallCb       func(llm.ToolCall)
	onTranscriptionCb  func(llm.TranscriptionResult)
	onVADCb            func(llm.VADSignal)
	onGoAwayCb         func(string)
	onSessionUpdateCb  func(bool)
	onUsageCb          func(int, int)
	onErrorCb         func(error)
	onDisconnectCb    func()
}

func New(cfg llm.RealtimeConfig) *Provider { return &Provider{config: cfg} }

func (p *Provider) Connect(ctx context.Context, cfg llm.RealtimeConfig) error {
	p.config = cfg
	if p.config.APIKey == "" {
		return fmt.Errorf("API key is required")
	}

	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  p.config.APIKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return fmt.Errorf("create client failed: %w", err)
	}
	p.client = client

	model := p.config.Model
	if model == "" {
		model = "models/gemini-2.5-flash-native-audio-preview-12-2025"
	}

	liveCfg := &genai.LiveConnectConfig{}

	// Native-audio model: use audio modality
	if strings.Contains(strings.ToLower(model), "native-audio") {
		voice := p.config.Voice
		if voice == "" {
			voice = "Kore"
		}
		liveCfg.ResponseModalities = []genai.Modality{genai.ModalityAudio}
		speechCfg := &genai.SpeechConfig{
			VoiceConfig: &genai.VoiceConfig{
				PrebuiltVoiceConfig: &genai.PrebuiltVoiceConfig{VoiceName: voice},
			},
		}
		if p.config.SpeechLanguageCode != "" {
			speechCfg.LanguageCode = p.config.SpeechLanguageCode
		} else if p.config.InputLanguage != "" {
			speechCfg.LanguageCode = p.config.InputLanguage
		}
		liveCfg.SpeechConfig = speechCfg
	} else {
		liveCfg.ResponseModalities = []genai.Modality{genai.ModalityText}
	}

	// Generation parameters
	if p.config.Temperature > 0 {
		liveCfg.Temperature = genai.Ptr[float32](p.config.Temperature)
	}
	if p.config.TopP > 0 {
		liveCfg.TopP = genai.Ptr[float32](p.config.TopP)
	}
	if p.config.TopK > 0 {
		liveCfg.TopK = genai.Ptr[float32](p.config.TopK)
	}
	if p.config.MaxTokens > 0 {
		liveCfg.MaxOutputTokens = p.config.MaxTokens
	}
	if p.config.Seed != 0 {
		liveCfg.Seed = genai.Ptr[int32](p.config.Seed)
	}

	// Thinking config
	if p.config.Thinking || p.config.IncludeThoughts || p.config.ThinkingBudget > 0 {
		thinkingCfg := &genai.ThinkingConfig{}
		if p.config.IncludeThoughts {
			thinkingCfg.IncludeThoughts = true
		}
		if p.config.ThinkingBudget > 0 {
			thinkingCfg.ThinkingBudget = genai.Ptr[int32](p.config.ThinkingBudget)
		} else if p.config.Thinking {
			thinkingCfg.ThinkingBudget = genai.Ptr[int32](1000)
		}
		liveCfg.ThinkingConfig = thinkingCfg
	}

	// Tools (full function declarations)
	if len(p.config.Tools) > 0 {
		decls := make([]*genai.FunctionDeclaration, len(p.config.Tools))
		for i, t := range p.config.Tools {
			decls[i] = &genai.FunctionDeclaration{
				Name:        t.Function.Name,
				Description: t.Function.Description,
				Parameters:  convertToSchema(t.Function.Parameters),
			}
		}
		liveCfg.Tools = []*genai.Tool{{
			FunctionDeclarations: decls,
		}}
	}

	// Affective dialog - TODO: not supported by Gemini Live API yet (server returns "Cannot find field")
	// if p.config.EnableAffectiveDialog != nil {
	// 	liveCfg.EnableAffectiveDialog = genai.Ptr[bool](*p.config.EnableAffectiveDialog)
	// }

	// Proactive audio
	if p.config.ProactiveAudio != nil {
		liveCfg.Proactivity = &genai.ProactivityConfig{ProactiveAudio: genai.Ptr[bool](*p.config.ProactiveAudio)}
	}

	// Input transcription
	if p.config.InputAudioTranscription {
		liveCfg.InputAudioTranscription = &genai.AudioTranscriptionConfig{}
	}

	// Output transcription
	if p.config.OutputAudioTranscription {
		liveCfg.OutputAudioTranscription = &genai.AudioTranscriptionConfig{}
	}

	// Session resumption
	if p.config.SessionResumption || p.config.SessionResumptionHandle != "" || p.config.SessionResumptionTransparent {
		liveCfg.SessionResumption = &genai.SessionResumptionConfig{
			Handle:      p.config.SessionResumptionHandle,
			Transparent: p.config.SessionResumptionTransparent,
		}
	}

	// Context window compression
	if p.config.ContextWindowCompression || p.config.ContextCompressionTriggerTokens > 0 || p.config.ContextCompressionTargetTokens > 0 {
		cwc := &genai.ContextWindowCompressionConfig{}
		if p.config.ContextCompressionTriggerTokens > 0 {
			cwc.TriggerTokens = genai.Ptr[int64](p.config.ContextCompressionTriggerTokens)
		}
		if p.config.ContextCompressionTargetTokens > 0 {
			cwc.SlidingWindow = &genai.SlidingWindow{TargetTokens: genai.Ptr[int64](p.config.ContextCompressionTargetTokens)}
		}
		liveCfg.ContextWindowCompression = cwc
	}

	// Realtime input / auto VAD configuration
	rtic := &genai.RealtimeInputConfig{}
	hasRTIC := false
	if p.config.AutoVADDisabled != nil || p.config.VADStartSensitivity != "" || p.config.VADEndSensitivity != "" || p.config.VADPrefixPaddingMs > 0 || p.config.VADSilenceDurationMs > 0 {
		aad := &genai.AutomaticActivityDetection{}
		if p.config.AutoVADDisabled != nil {
			aad.Disabled = *p.config.AutoVADDisabled
		}
		if s := parseStartSensitivity(p.config.VADStartSensitivity); s != "" {
			aad.StartOfSpeechSensitivity = s
		}
		if s := parseEndSensitivity(p.config.VADEndSensitivity); s != "" {
			aad.EndOfSpeechSensitivity = s
		}
		if p.config.VADPrefixPaddingMs > 0 {
			aad.PrefixPaddingMs = genai.Ptr[int32](p.config.VADPrefixPaddingMs)
		}
		if p.config.VADSilenceDurationMs > 0 {
			aad.SilenceDurationMs = genai.Ptr[int32](p.config.VADSilenceDurationMs)
		}
		rtic.AutomaticActivityDetection = aad
		hasRTIC = true
	}
	if h := parseActivityHandling(p.config.VADActivityHandling); h != "" {
		rtic.ActivityHandling = h
		hasRTIC = true
	}
	if c := parseTurnCoverage(p.config.VADTurnCoverage); c != "" {
		rtic.TurnCoverage = c
		hasRTIC = true
	}
	if hasRTIC {
		liveCfg.RealtimeInputConfig = rtic
	}

	// Explicit VAD signal
	if p.config.ExplicitVAD {
		liveCfg.ExplicitVADSignal = genai.Ptr[bool](true)
	}

	// Media resolution
	if mr := parseMediaResolution(p.config.MediaResolution); mr != "" {
		liveCfg.MediaResolution = mr
	}

	// System instruction
	if p.config.Instructions != "" {
		liveCfg.SystemInstruction = &genai.Content{Parts: []*genai.Part{{Text: p.config.Instructions}}}
	}

	session, err := p.client.Live.Connect(ctx, model, liveCfg)
	if err != nil {
		return fmt.Errorf("live connect failed: %w", err)
	}

	p.mu.Lock()
	p.session = session
	p.connected = true
	p.pcmBuf.Reset()
	p.mu.Unlock()

	go p.receiveLoop()
	return nil
}

func (p *Provider) Disconnect() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.session != nil {
		p.session.Close()
		p.session = nil
	}
	p.connected = false
	p.pcmBuf.Reset()
	return nil
}

func (p *Provider) IsConnected() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.connected
}

func (p *Provider) SendAudio(ctx context.Context, audioData []byte) error {
	p.mu.RLock()
	s := p.session
	ok := p.connected
	p.mu.RUnlock()
	if !ok || s == nil {
		return fmt.Errorf("not connected")
	}
	if len(audioData) == 0 {
		return nil
	}
	return s.SendRealtimeInput(genai.LiveRealtimeInput{
		Audio: &genai.Blob{
			MIMEType: "audio/pcm",
			Data:     audioData,
		},
	})
}

func (p *Provider) EndAudio(ctx context.Context) error {
	p.mu.RLock()
	s := p.session
	ok := p.connected
	p.mu.RUnlock()
	if !ok || s == nil {
		return fmt.Errorf("not connected")
	}
	return s.SendRealtimeInput(genai.LiveRealtimeInput{
		AudioStreamEnd: true,
	})
}

func (p *Provider) SendText(ctx context.Context, text string) error {
	p.mu.RLock()
	s := p.session
	ok := p.connected
	p.mu.RUnlock()
	if !ok || s == nil {
		return fmt.Errorf("not connected")
	}
	p.mu.Lock()
	p.pcmBuf.Reset()
	p.mu.Unlock()
	turn := genai.NewContentFromText(text, genai.RoleUser)
	return s.SendClientContent(genai.LiveClientContentInput{
		Turns:        []*genai.Content{turn},
		TurnComplete: genai.Ptr(true),
	})
}

func (p *Provider) SendToolResponse(ctx context.Context, resp llm.ToolResponse) error {
	p.mu.RLock()
	s := p.session
	ok := p.connected
	p.mu.RUnlock()
	if !ok || s == nil {
		return fmt.Errorf("not connected")
	}

	return s.SendToolResponse(genai.LiveToolResponseInput{
		FunctionResponses: []*genai.FunctionResponse{{
			ID:   resp.ID,
			Name: resp.Name,
			Response: map[string]any{
				"output": resp.Result,
			},
		}},
	})
}

// Callback setters
func (p *Provider) OnAudio(fn func([]byte))                  { p.mu.Lock(); p.onAudioCb = fn; p.mu.Unlock() }
func (p *Provider) OnText(fn func(string))                   { p.mu.Lock(); p.onTextCb = fn; p.mu.Unlock() }
func (p *Provider) OnToolCall(fn func(llm.ToolCall))         { p.mu.Lock(); p.onToolCallCb = fn; p.mu.Unlock() }
func (p *Provider) OnTranscription(fn func(llm.TranscriptionResult)) { p.mu.Lock(); p.onTranscriptionCb = fn; p.mu.Unlock() }
func (p *Provider) OnVAD(fn func(llm.VADSignal))            { p.mu.Lock(); p.onVADCb = fn; p.mu.Unlock() }
func (p *Provider) OnGoAway(fn func(string))                { p.mu.Lock(); p.onGoAwayCb = fn; p.mu.Unlock() }
func (p *Provider) OnSessionUpdate(fn func(bool))             { p.mu.Lock(); p.onSessionUpdateCb = fn; p.mu.Unlock() }
func (p *Provider) OnUsage(fn func(int, int))                { p.mu.Lock(); p.onUsageCb = fn; p.mu.Unlock() }
func (p *Provider) OnError(fn func(error))                  { p.mu.Lock(); p.onErrorCb = fn; p.mu.Unlock() }
func (p *Provider) OnDisconnect(fn func())                  { p.mu.Lock(); p.onDisconnectCb = fn; p.mu.Unlock() }

func (p *Provider) receiveLoop() {
	for {
		p.mu.RLock()
		s := p.session
		p.mu.RUnlock()
		if s == nil {
			return
		}
		msg, err := s.Receive()
		if err != nil {
			p.emitError(fmt.Errorf("receive error: %w", err))
			p.emitDisconnect()
			return
		}
		p.processMessage(msg)
	}
}

func (p *Provider) processMessage(msg *genai.LiveServerMessage) {
	if msg == nil {
		return
	}

	// Tool call
	if msg.ToolCall != nil && len(msg.ToolCall.FunctionCalls) > 0 {
		for _, fc := range msg.ToolCall.FunctionCalls {
			if fc.ID != "" && fc.Name != "" {
				var args map[string]interface{}
				argsJSON, _ := json.Marshal(fc.Args)
				_ = json.Unmarshal(argsJSON, &args)

				p.mu.RLock()
				cb := p.onToolCallCb
				p.mu.RUnlock()
				if cb != nil {
					cb(llm.ToolCall{
						ID:   fc.ID,
						Type: "function",
						Function: &llm.ToolFunction{
							Name:        fc.Name,
							Parameters:  args,
						},
					})
				}
			}
		}
	}

	// Input transcription
	if msg.ServerContent != nil && msg.ServerContent.InputTranscription != nil {
		p.mu.RLock()
		cb := p.onTranscriptionCb
		p.mu.RUnlock()
		if cb != nil && msg.ServerContent.InputTranscription.Text != "" {
			cb(llm.TranscriptionResult{
				Text: msg.ServerContent.InputTranscription.Text,
				Type: "input",
			})
		}
	}

	// VAD
	if msg.VoiceActivity != nil {
		p.mu.RLock()
		cb := p.onVADCb
		p.mu.RUnlock()
		if cb != nil {
			cb(llm.VADSignal{
				Active: msg.VoiceActivity.VoiceActivityType != "",
				Type:   string(msg.VoiceActivity.VoiceActivityType),
			})
		}
	}

	// Go away
	if msg.GoAway != nil {
		p.mu.RLock()
		cb := p.onGoAwayCb
		p.mu.RUnlock()
		if cb != nil {
			cb("server will disconnect soon")
		}
	}

	// Session update
	if msg.SessionResumptionUpdate != nil {
		p.mu.RLock()
		cb := p.onSessionUpdateCb
		p.mu.RUnlock()
		if cb != nil {
			cb(msg.SessionResumptionUpdate.Resumable)
		}
	}

	// Usage
	if msg.UsageMetadata != nil {
		p.mu.RLock()
		cb := p.onUsageCb
		p.mu.RUnlock()
		if cb != nil {
			cb(int(msg.UsageMetadata.PromptTokenCount), int(msg.UsageMetadata.ResponseTokenCount))
		}
	}

	// Server content (text + audio)
	if msg.ServerContent != nil && msg.ServerContent.ModelTurn != nil {
		for _, part := range msg.ServerContent.ModelTurn.Parts {
			// Text
			if part.Text != "" {
				p.mu.RLock()
				cb := p.onTextCb
				p.mu.RUnlock()
				if cb != nil {
					cb(part.Text)
				}
			}
			// Audio
			if part.InlineData != nil && len(part.InlineData.Data) > 0 {
				p.mu.Lock()
				_, _ = p.pcmBuf.Write(part.InlineData.Data)
				p.mu.Unlock()
			}
		}

		// Output transcription
		if msg.ServerContent.OutputTranscription != nil && msg.ServerContent.OutputTranscription.Text != "" {
			p.mu.RLock()
			cb := p.onTranscriptionCb
			p.mu.RUnlock()
			if cb != nil {
				cb(llm.TranscriptionResult{
					Text: msg.ServerContent.OutputTranscription.Text,
					Type: "output",
				})
			}
		}
	}

	// Turn complete - emit full WAV
	if msg.ServerContent != nil && (msg.ServerContent.TurnComplete || msg.ServerContent.GenerationComplete) {
		p.mu.Lock()
		pcm := append([]byte(nil), p.pcmBuf.Bytes()...)
		p.pcmBuf.Reset()
		cb := p.onAudioCb
		p.mu.Unlock()
		if cb != nil && len(pcm) > 0 {
			cb(pcmToWav(pcm, 24000, 1, 16))
		}
	}
}

func pcmToWav(pcm []byte, sampleRate, channels, bitsPerSample int) []byte {
	byteRate := sampleRate * channels * bitsPerSample / 8
	blockAlign := channels * bitsPerSample / 8
	dataLen := uint32(len(pcm))
	riffLen := 36 + dataLen

	buf := &bytes.Buffer{}
	buf.WriteString("RIFF")
	_ = binary.Write(buf, binary.LittleEndian, uint32(riffLen))
	buf.WriteString("WAVE")
	buf.WriteString("fmt ")
	_ = binary.Write(buf, binary.LittleEndian, uint32(16))
	_ = binary.Write(buf, binary.LittleEndian, uint16(1))
	_ = binary.Write(buf, binary.LittleEndian, uint16(channels))
	_ = binary.Write(buf, binary.LittleEndian, uint32(sampleRate))
	_ = binary.Write(buf, binary.LittleEndian, uint32(byteRate))
	_ = binary.Write(buf, binary.LittleEndian, uint16(blockAlign))
	_ = binary.Write(buf, binary.LittleEndian, uint16(bitsPerSample))
	buf.WriteString("data")
	_ = binary.Write(buf, binary.LittleEndian, dataLen)
	buf.Write(pcm)
	return buf.Bytes()
}

func (p *Provider) emitError(err error) {
	p.mu.RLock()
	cb := p.onErrorCb
	p.mu.RUnlock()
	if cb != nil {
		cb(err)
	}
}

func (p *Provider) emitDisconnect() {
	p.mu.Lock()
	p.connected = false
	p.mu.Unlock()
	p.mu.RLock()
	cb := p.onDisconnectCb
	p.mu.RUnlock()
	if cb != nil {
		cb()
	}
}

func parseStartSensitivity(v string) genai.StartSensitivity {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "", "default", "unspecified", "start_sensitivity_unspecified":
		return ""
	case "low", "start_sensitivity_low":
		return genai.StartSensitivityLow
	case "high", "start_sensitivity_high":
		return genai.StartSensitivityHigh
	default:
		return ""
	}
}

func parseEndSensitivity(v string) genai.EndSensitivity {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "", "default", "unspecified", "end_sensitivity_unspecified":
		return ""
	case "low", "end_sensitivity_low":
		return genai.EndSensitivityLow
	case "high", "end_sensitivity_high":
		return genai.EndSensitivityHigh
	default:
		return ""
	}
}

func parseActivityHandling(v string) genai.ActivityHandling {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "", "default", "unspecified", "activity_handling_unspecified":
		return ""
	case "start_of_activity_interrupts", "interrupts", "barge_in":
		return genai.ActivityHandlingStartOfActivityInterrupts
	case "no_interruption", "no_interrupt":
		return genai.ActivityHandlingNoInterruption
	default:
		return ""
	}
}

func parseTurnCoverage(v string) genai.TurnCoverage {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "", "default", "unspecified", "turn_coverage_unspecified":
		return ""
	case "turn_includes_only_activity", "only_activity":
		return genai.TurnCoverageTurnIncludesOnlyActivity
	case "turn_includes_all_input", "all_input":
		return genai.TurnCoverageTurnIncludesAllInput
	default:
		return ""
	}
}

func parseMediaResolution(v string) genai.MediaResolution {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "", "default", "unspecified", "media_resolution_unspecified":
		return ""
	case "low", "media_resolution_low":
		return genai.MediaResolutionLow
	case "medium", "media_resolution_medium":
		return genai.MediaResolutionMedium
	case "high", "media_resolution_high":
		return genai.MediaResolutionHigh
	default:
		return ""
	}
}

// convertToSchema converts interface{} parameters to *genai.Schema
func convertToSchema(params interface{}) *genai.Schema {
	if params == nil {
		return nil
	}

	// Try to convert from map[string]interface{} directly
	if m, ok := params.(map[string]interface{}); ok {
		return mapToSchema(m)
	}

	// If it's already a JSON string, try to unmarshal
	if s, ok := params.(string); ok {
		var m map[string]interface{}
		if err := json.Unmarshal([]byte(s), &m); err == nil {
			return mapToSchema(m)
		}
	}

	return nil
}

func mapToSchema(m map[string]interface{}) *genai.Schema {
	if m == nil {
		return nil
	}

	schema := &genai.Schema{}

	if t, ok := m["type"].(string); ok {
		schema.Type = genai.Type(t)
	}

	if desc, ok := m["description"].(string); ok {
		schema.Description = desc
	}

	if props, ok := m["properties"].(map[string]interface{}); ok {
		schema.Properties = make(map[string]*genai.Schema)
		for k, v := range props {
			if propMap, ok := v.(map[string]interface{}); ok {
				schema.Properties[k] = mapToSchema(propMap)
			}
		}
	}

	if required, ok := m["required"].([]interface{}); ok {
		schema.Required = make([]string, len(required))
		for i, r := range required {
			if s, ok := r.(string); ok {
				schema.Required[i] = s
			}
		}
	}

	return schema
}
