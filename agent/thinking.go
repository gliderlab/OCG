package agent

import (
	"bufio"
	"strings"
	"sync"
)

// ThinkingMode Thinking mode
type ThinkingMode string

const (
	ThinkingModeOff    ThinkingMode = "off"     // off
	ThinkingModeOn     ThinkingMode = "on"      // on but not streaming
	ThinkingModeStream ThinkingMode = "stream" // streaming thinking
)

// ThinkingConfig Thinking config
type ThinkingConfig struct {
	Mode          ThinkingMode
	MaxTokens     int
	IncludeReason bool
}

// DefaultThinkingConfig default config
var DefaultThinkingConfig = ThinkingConfig{
	Mode:      ThinkingModeOff,
	MaxTokens: 4000,
}

// ParseThinkingMode Parse thinking mode
func ParseThinkingMode(s string) ThinkingMode {
	s = strings.ToLower(strings.TrimSpace(s))
	switch s {
	case "on":
		return ThinkingModeOn
	case "stream":
		return ThinkingModeStream
	default:
		return ThinkingModeOff
	}
}

// ThinkingProcessor Process thinking content
type ThinkingProcessor struct {
	config    ThinkingConfig
	buf       strings.Builder
	mu        sync.RWMutex
	isActive  bool
	finalText string
}

// NewThinkingProcessor Create thinking processor
func NewThinkingProcessor(cfg ThinkingConfig) *ThinkingProcessor {
	if cfg.MaxTokens == 0 {
		cfg = DefaultThinkingConfig
	}
	return &ThinkingProcessor{
		config: cfg,
		buf:    strings.Builder{},
	}
}

// IsEnabled Is enabled
func (p *ThinkingProcessor) IsEnabled() bool {
	return p.config.Mode != ThinkingModeOff
}

// IsStream Is streaming
func (p *ThinkingProcessor) IsStream() bool {
	return p.config.Mode == ThinkingModeStream
}

// Start Start thinking
func (p *ThinkingProcessor) Start() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.isActive = true
	p.buf.Reset()
}

// Append Append thinking
func (p *ThinkingProcessor) Append(text string) {
	if !p.IsEnabled() {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.buf.WriteString(text)
}

// End End thinking
func (p *ThinkingProcessor) End() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.isActive = false
	p.finalText = p.buf.String()
}

// GetThinking Get thinking content
func (p *ThinkingProcessor) GetThinking() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.finalText
}

// ExtractThinkingFromStream Extract thinking from stream
func ExtractThinkingFromStream(data string) (thinking, remaining string, hasThinking bool) {
	// Detect thinking tag
	thinkingStart := strings.Index(data, "<think>")
	thinkEnd := strings.Index(data, "</think>")
	
	if thinkingStart == -1 && thinkEnd == -1 {
		return "", data, false
	}
	
	// Priority detect end tag
	if thinkEnd != -1 && thinkingStart != -1 && thinkEnd > thinkingStart {
		thinking = data[thinkingStart+len("<think>") : thinkEnd]
		remaining = data[:thinkingStart] + data[thinkEnd+len("</think>"):]
		return thinking, remaining, true
	}
	
	// Only start tag
	if thinkingStart != -1 {
		remaining = data[:thinkingStart]
		return "", remaining, true
	}
	
	return "", data, false
}

// ParseThinkingBlocks Parse thinking blocks
func ParseThinkingBlocks(content string) ([]string, string) {
	var blocks []string
	remainder := content
	
	for {
		start := strings.Index(remainder, "<think>")
		if start == -1 {
			break
		}
		
		// Find content start
		contentStart := start + len("<think>")
		
		// Find end tag
		end := strings.Index(remainder[contentStart:], "</think>")
		if end == -1 {
			// No end tag, rest is thinking
			blocks = append(blocks, remainder[contentStart:])
			remainder = remainder[:start]
			break
		}
		
		// Extract thinking block
		blocks = append(blocks, remainder[contentStart:contentStart+end])
		
		// Continue processing
		remainder = remainder[:start] + remainder[contentStart+end+len("</think>"):]
	}
	
	return blocks, remainder
}

// FormatThinkingForMarkdown Format thinking for markdown
func FormatThinkingForMarkdown(thinking string) string {
	var sb strings.Builder
	sb.WriteString("<thinking>\n")
	
	scanner := bufio.NewScanner(strings.NewReader(thinking))
	for scanner.Scan() {
		sb.WriteString("  ")
		sb.WriteString(scanner.Text())
		sb.WriteString("\n")
	}
	
	sb.WriteString("</thinking>\n")
	return sb.String()
}

// HasThinkingTag Check if contains thinking tag
func HasThinkingTag(content string) bool {
	return strings.Contains(content, "<think>") || 
		   strings.Contains(content, "<thinking>") ||
		   strings.Contains(content, "<thought>")
}

// StripThinkingTags Strip thinking tags
func StripThinkingTags(content string) string {
	result := content
	result = strings.ReplaceAll(result, "<think>", "")
	result = strings.ReplaceAll(result, "</think>", "")
	result = strings.ReplaceAll(result, "<thinking>", "")
	result = strings.ReplaceAll(result, "</thinking>", "")
	result = strings.ReplaceAll(result, "<thought>", "")
	result = strings.ReplaceAll(result, "</thought>", "")
	
	// Clean up extra whitespace
	lines := strings.Split(result, "\n")
	var cleaned []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			cleaned = append(cleaned, trimmed)
		}
	}
	
	return strings.Join(cleaned, "\n")
}

// ExtractFinalAnswer Extract final answer
func ExtractFinalAnswer(response string) string {
	_, remainder := ParseThinkingBlocks(response)
	return strings.TrimSpace(remainder)
}
