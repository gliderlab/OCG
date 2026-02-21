// Package telegram provides Telegram bot channel implementation
package telegram

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gliderlab/cogate/gateway/channels/types"
)

// TelegramBot implements the types.ChannelLoader interface
type TelegramBot struct {
	token            string
	baseURL          string
	client           *http.Client
	agentRPC         types.AgentRPCInterface
	running          bool
	stopCh           chan struct{}
	greetedUsers     map[int64]bool
	muGreeted        sync.RWMutex

	// Long Polling mode
	mode          string // "webhook" or "long_polling"
	offset        int    // Last processed update ID
	muOffset      sync.Mutex
	pollInterval  time.Duration
	allowedChats  map[int64]bool // Whitelist for allowed chats

	// Worker pool for bounded concurrency
	msgCh     chan TelegramUpdate
	workerCnt int
	wg        sync.WaitGroup
}

const maxGreetedUsers = 10000

// NewTelegramBot creates a new Telegram bot
// mode: "long_polling" (default) or "webhook"
func NewTelegramBot(token string, agentRPC types.AgentRPCInterface) *TelegramBot {
	mode := os.Getenv("TELEGRAM_MODE")
	if mode == "" {
		mode = "long_polling" // Default to Long Polling
	}

	return &TelegramBot{
		token:           token,
		baseURL:         fmt.Sprintf("https://api.telegram.org/bot%s", token),
		client:          &http.Client{Timeout: 35 * time.Second},
		agentRPC:        agentRPC,
		stopCh:          make(chan struct{}),
		greetedUsers:    make(map[int64]bool),
		mode:            mode,
		offset:          0,
		pollInterval:    1 * time.Second,
		allowedChats:    make(map[int64]bool),
		msgCh:           make(chan TelegramUpdate, 100), // Bounded queue
		workerCnt:       8,                               // Max concurrent workers
	}
}

func (b *TelegramBot) ChannelInfo() types.ChannelInfo {
	modeDesc := b.mode
	if b.mode == "long_polling" {
		modeDesc = "Long Polling (no webhook needed)"
	}
	return types.ChannelInfo{
		Name:        "Telegram",
		Type:        types.ChannelTelegram,
		Version:     "1.0.0",
		Description: fmt.Sprintf("Telegram Bot API integration with %s mode", modeDesc),
	}
}

func (b *TelegramBot) Initialize(config map[string]interface{}) error {
	return nil
}

func (b *TelegramBot) Start() error {
	if b.running {
		return nil
	}
	log.Printf("[START] Starting Telegram bot (mode: %s)...", b.mode)
	b.running = true

	if b.mode == "long_polling" {
		// Delete webhook first to enable getUpdates
		b.deleteWebhook()

		// Start Long Polling loop in background
		go b.startLongPolling()
	} else {
		log.Printf("📡 Telegram bot running in Webhook mode")
	}

	return nil
}

func (b *TelegramBot) Stop() error {
	if !b.running {
		return nil
	}

	// Stop Long Polling
	if b.mode == "long_polling" {
		close(b.stopCh)
		b.stopCh = make(chan struct{}) // Recreate for potential restart
	}

	b.running = false
	log.Printf("🛑 Telegram bot stopped")
	return nil
}

func (b *TelegramBot) SendMessage(req *types.SendMessageRequest) (*types.SendMessageResponse, error) {
	text := req.Text
	if len(text) > 4096 {
		text = text[:4096]
	}

	apiReq := map[string]interface{}{
		"chat_id":    req.ChatID,
		"text":       text,
		"parse_mode": "Markdown",
	}

	if len(req.Buttons) > 0 {
		inlineKeyboard := make([][]map[string]string, 0, len(req.Buttons))
		for _, row := range req.Buttons {
			buttonRow := make([]map[string]string, 0, len(row))
			for _, btn := range row {
				buttonRow = append(buttonRow, map[string]string{
					"text":          btn.Text,
					"callback_data": btn.CallbackData,
				})
			}
			inlineKeyboard = append(inlineKeyboard, buttonRow)
		}
		apiReq["reply_markup"] = map[string]interface{}{
			"inline_keyboard": inlineKeyboard,
		}
	}

	payload, _ := json.Marshal(apiReq)
	resp, err := b.client.Post(b.baseURL+"/sendMessage", "application/json", strings.NewReader(string(payload)))
	if err != nil {
		return &types.SendMessageResponse{OK: false, Error: err.Error()}, err
	}
	defer resp.Body.Close()

	var sendResp struct {
		OK          bool   `json:"ok"`
		ErrorCode   int    `json:"error_code,omitempty"`
		Description string `json:"description,omitempty"`
		Result      struct {
			MessageID int `json:"message_id"`
			Chat      struct {
				ID int64 `json:"id"`
			} `json:"chat"`
			Date int `json:"date"`
		} `json:"result"`
	}

	json.NewDecoder(resp.Body).Decode(&sendResp)

	if !sendResp.OK {
		return &types.SendMessageResponse{OK: false, Error: sendResp.Description}, nil
	}

	return &types.SendMessageResponse{
		OK:        true,
		MessageID: int64(sendResp.Result.MessageID),
		ChatID:    sendResp.Result.Chat.ID,
		Timestamp: int64(sendResp.Result.Date),
	}, nil
}

func (b *TelegramBot) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	types.LimitBody(w, r)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("Error reading webhook body: %v", err)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	var update TelegramUpdate
	if err := json.Unmarshal(body, &update); err != nil {
		log.Printf("Error parsing webhook JSON: %v", err)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	if update.Message.Text != "" {
		go b.processMessage(update.Message)
	}

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprint(w, `{"ok": true}`)
}

func (b *TelegramBot) HealthCheck() error {
	resp, err := b.client.Get(b.baseURL + "/getMe")
	if err != nil {
		return fmt.Errorf("API connection failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	return nil
}

func (b *TelegramBot) processMessage(msg TelegramMessage) {
	chatID := int64(msg.Chat.ID)
	username := msg.From.Username
	userID := int64(msg.From.ID)

	text := strings.TrimSpace(msg.Text)
	if text == "" {
		text = strings.TrimSpace(msg.Caption)
	}
	hasVoice := msg.Voice.FileID != ""
	if text == "" && !hasVoice {
		return
	}

	log.Printf("📨 Received message from %s (@%s): text=%q voice=%v", msg.From.FirstName, username, text, hasVoice)

	// Handle commands first
	if strings.HasPrefix(text, "/start") {
		b.muGreeted.Lock()
		b.greetedUsers[userID] = true
		b.muGreeted.Unlock()
		b.sendSimpleMessage(chatID, fmt.Sprintf("Hello %s! I'm OCG-Go Telegram Bot. Send me a message!", msg.From.FirstName))
		return
	}

	if strings.HasPrefix(text, "/help") {
		b.sendSimpleMessage(chatID, "Commands:\n/start - Start bot\n/help - Help")
		return
	}

	// Generate session key from chat ID (unique per user/chat)
	sessionKey := fmt.Sprintf("telegram_%d", chatID)

	userContent := text
	if hasVoice {
		if text != "" {
			// Voice + text: transcription then force normal HTTP path
			transcript, err := b.transcribeTelegramVoice(msg.Voice.FileID)
			if err != nil {
				log.Printf("[Telegram] voice transcription failed: %v", err)
				b.sendSimpleMessage(chatID, "Sorry, voice transcription failed.")
				return
			}
			userContent = "/text " + strings.TrimSpace(transcript+"\n"+text)
		} else {
			// Voice only: do NOT transcribe first. Convert to PCM and force realtime audio path.
			pcmPath, err := b.downloadTelegramVoiceAsPCM(msg.Voice.FileID)
			if err != nil {
				log.Printf("[Telegram] voice download/convert failed: %v", err)
				b.sendSimpleMessage(chatID, "Sorry, voice processing failed.")
				return
			}
			userContent = "/live-audio-file " + pcmPath
		}
	}

	// Send to agent with session context
	messages := []types.Message{
		{Role: "system", Content: fmt.Sprintf("You are an AI assistant. User @%s (ID: %d) sent a message in Telegram chat %d.", username, userID, chatID)},
		{Role: "user", Content: userContent},
	}

	// Use ChatWithSession to load conversation history
	response, err := b.agentRPC.ChatWithSession(sessionKey, messages)
	if err != nil {
		log.Printf("Agent error: %v", err)
		b.sendSimpleMessage(chatID, "Sorry, I encountered an error.")
		return
	}

	b.sendSimpleMessage(chatID, response)
}

func (b *TelegramBot) sendSimpleMessage(chatID int64, text string) {
	// Send typing indicator before sending message
	b.sendChatAction(chatID, "typing")

	if len(text) > 4096 {
		text = text[:4096]
	}

	apiReq := map[string]interface{}{
		"chat_id":    chatID,
		"text":       text,
		"parse_mode": "Markdown",
	}

	payload, _ := json.Marshal(apiReq)
	b.client.Post(b.baseURL+"/sendMessage", "application/json", strings.NewReader(string(payload)))
}

// sendChatAction sends a chat action (typing, uploading_photo, etc.)
func (b *TelegramBot) sendChatAction(chatID int64, action string) {
	apiReq := map[string]interface{}{
		"chat_id": chatID,
		"action":  action,
	}
	payload, _ := json.Marshal(apiReq)
	b.client.Post(b.baseURL+"/sendChatAction", "application/json", strings.NewReader(string(payload)))
}

func (b *TelegramBot) transcribeTelegramVoice(fileID string) (string, error) {
	filePath, err := b.getTelegramFilePath(fileID)
	if err != nil {
		return "", err
	}

	resp, err := b.client.Get(fmt.Sprintf("https://api.telegram.org/file/bot%s/%s", b.token, filePath))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("download voice failed: status=%d body=%s", resp.StatusCode, string(body))
	}

	tmpDir := os.TempDir()
	inPath := filepath.Join(tmpDir, fmt.Sprintf("ocg-tg-voice-%d.ogg", time.Now().UnixNano()))
	outDir := tmpDir
	if data, err := io.ReadAll(resp.Body); err != nil {
		return "", err
	} else if err := os.WriteFile(inPath, data, 0644); err != nil {
		return "", err
	}
	defer os.Remove(inPath)

	cmd := exec.Command("whisper", inPath, "--model", "tiny", "--output_format", "txt", "--output_dir", outDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("whisper failed: %v (%s)", err, string(out))
	}

	txtPath := strings.TrimSuffix(inPath, filepath.Ext(inPath)) + ".txt"
	defer os.Remove(txtPath)
	bytes, err := os.ReadFile(txtPath)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(bytes)), nil
}

func (b *TelegramBot) downloadTelegramVoiceAsPCM(fileID string) (string, error) {
	filePath, err := b.getTelegramFilePath(fileID)
	if err != nil {
		return "", err
	}

	resp, err := b.client.Get(fmt.Sprintf("https://api.telegram.org/file/bot%s/%s", b.token, filePath))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("download voice failed: status=%d body=%s", resp.StatusCode, string(body))
	}

	tmpDir := os.TempDir()
	inPath := filepath.Join(tmpDir, fmt.Sprintf("ocg-tg-voice-%d.ogg", time.Now().UnixNano()))
	pcmPath := strings.TrimSuffix(inPath, filepath.Ext(inPath)) + ".pcm"
	if data, err := io.ReadAll(resp.Body); err != nil {
		return "", err
	} else if err := os.WriteFile(inPath, data, 0644); err != nil {
		return "", err
	}
	defer os.Remove(inPath)

	cmd := exec.Command("ffmpeg", "-y", "-i", inPath, "-ac", "1", "-ar", "24000", "-f", "s16le", pcmPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		_ = os.Remove(pcmPath)
		return "", fmt.Errorf("ffmpeg convert failed: %v (%s)", err, string(out))
	}
	return pcmPath, nil
}

func (b *TelegramBot) getTelegramFilePath(fileID string) (string, error) {
	resp, err := b.client.Get(fmt.Sprintf("%s/getFile?file_id=%s", b.baseURL, fileID))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("getFile failed: status=%d body=%s", resp.StatusCode, string(body))
	}
	var result struct {
		OK     bool `json:"ok"`
		Result struct {
			FilePath string `json:"file_path"`
		} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	if !result.OK || result.Result.FilePath == "" {
		return "", fmt.Errorf("empty telegram file path")
	}
	return result.Result.FilePath, nil
}

// Telegram types
type TelegramUpdate struct {
	UpdateID int             `json:"update_id"`
	Message  TelegramMessage `json:"message"`
}

type TelegramMessage struct {
	MessageID int           `json:"message_id"`
	From      TelegramUser  `json:"from"`
	Chat      TelegramChat  `json:"chat"`
	Date      int           `json:"date"`
	Text      string        `json:"text"`
	Caption   string        `json:"caption,omitempty"`
	Voice     TelegramVoice `json:"voice,omitempty"`
	ThreadID  int           `json:"message_thread_id,omitempty"`
}

type TelegramVoice struct {
	FileID   string `json:"file_id"`
	Duration int    `json:"duration,omitempty"`
	MimeType string `json:"mime_type,omitempty"`
}

type TelegramUser struct {
	ID           int    `json:"id"`
	IsBot        bool   `json:"is_bot"`
	FirstName    string `json:"first_name"`
	LastName     string `json:"last_name"`
	Username     string `json:"username"`
	LanguageCode string `json:"language_code"`
}

type TelegramChat struct {
	ID        int    `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Username  string `json:"username"`
	Type      string `json:"type"`
}

// Long Polling implementation

// deleteWebhook removes any existing webhook to enable getUpdates
func (b *TelegramBot) deleteWebhook() {
	resp, err := b.client.Post(b.baseURL+"/deleteWebhook", "application/json", nil)
	if err != nil {
		log.Printf("[Telegram] channel=telegram action=deleteWebhook error=%v", err)
		return
	}
	defer resp.Body.Close()

	var result struct {
		OK          bool   `json:"ok"`
		Description string `json:"description"`
	}
	json.NewDecoder(resp.Body).Decode(&result)

	if result.OK {
		log.Printf("[Telegram] channel=telegram action=deleteWebhook status=success")
	} else {
		log.Printf("[Telegram] channel=telegram action=deleteWebhook status=failed reason=%s", result.Description)
	}
}

// startLongPolling starts the Long Polling loop with worker pool
func (b *TelegramBot) startLongPolling() {
	log.Printf("[RELOAD] Starting Long Polling loop with %d workers...", b.workerCnt)

	// Start worker pool
	b.wg.Add(b.workerCnt)
	for i := 0; i < b.workerCnt; i++ {
		go b.messageWorker(i)
	}

	for {
		select {
		case <-b.stopCh:
			log.Printf("🛑 Long Polling stopping, waiting for workers...")
			close(b.msgCh)
			b.wg.Wait()
			log.Printf("[OK] All workers stopped")
			return
		default:
			b.pollUpdates()
			time.Sleep(b.pollInterval)
		}
	}
}

// messageWorker processes messages from the bounded queue
func (b *TelegramBot) messageWorker(id int) {
	defer b.wg.Done()
	for update := range b.msgCh {
		if update.Message.Text != "" {
			b.processMessage(update.Message)
		}
	}
}

// pollUpdates fetches new updates from Telegram
func (b *TelegramBot) pollUpdates() {
	b.muOffset.Lock()
	offset := b.offset
	b.muOffset.Unlock()

	url := fmt.Sprintf("%s/getUpdates?timeout=30&offset=%d", b.baseURL, offset)
	resp, err := b.client.Get(url)
	if err != nil {
		log.Printf("[Telegram] channel=telegram action=pollUpdates error=%v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		log.Printf("[Telegram] channel=telegram action=pollUpdates http_status=%d", resp.StatusCode)
		return
	}

	var result struct {
		OK     bool              `json:"ok"`
		Result []TelegramUpdate `json:"result"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Printf("[Telegram] channel=telegram action=pollUpdates error=decode_failed details=%v", err)
		return
	}

	if !result.OK || len(result.Result) == 0 {
		return
	}

	// Process each update via bounded worker pool
	for _, update := range result.Result {
		b.muOffset.Lock()
		if update.UpdateID >= b.offset {
			b.offset = update.UpdateID + 1
		}
		b.muOffset.Unlock()

		if update.Message.Text != "" {
			select {
			case b.msgCh <- update:
				// Sent to worker pool
			default:
				// Queue full - log and drop (backpressure)
				log.Printf("[Telegram] channel=telegram action=pollUpdates status=dropped update_id=%d reason=queue_full", update.UpdateID)
			}
		}
	}
}

// TelegramCreateFromEnv creates Telegram bot from environment
func TelegramCreateFromEnv(agentRPC types.AgentRPCInterface) (*TelegramBot, error) {
	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("TELEGRAM_BOT_TOKEN not set")
	}
	return NewTelegramBot(token, agentRPC), nil
}
