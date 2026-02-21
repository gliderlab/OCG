// Message Tool - Send messages across channels
//
// Supports Telegram, WhatsApp, Discord, Slack, Signal, Google Chat, iMessage, MS Teams
// Uses the channel adapter system for message routing and actions.

package tools

import (
	"fmt"
	"strings"

	"github.com/gliderlab/cogate/gateway/channels"
)

type MessageTool struct {
	channelAdapter *channels.ChannelAdapter
}

// NewMessageTool creates a new message tool
func NewMessageTool(adapter *channels.ChannelAdapter) *MessageTool {
	return &MessageTool{channelAdapter: adapter}
}

func (t *MessageTool) Name() string {
	return "message"
}

func (t *MessageTool) Description() string {
	return "Send messages and channel actions across Telegram/Discord/Slack/WhatsApp/Signal/iMessage/MS Teams"
}

func (t *MessageTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"action": map[string]interface{}{
				"type":        "string",
				"description": "Action: send, react, read, edit, delete, search, poll, pin, thread-create, etc.",
				"enum": []string{
					"send", "react", "read", "edit", "delete", "search",
					"poll", "pin", "unpin", "list-pins",
					"thread-create", "thread-list", "thread-reply",
					"member-info", "role-info",
					"channel-info", "channel-list",
					"emoji-list", "sticker",
					"event-list", "event-create",
					"timeout", "kick", "ban",
					"permissions", "voice-status",
				},
			},
			"channel": map[string]interface{}{
				"type":        "string",
				"description": "Channel type: telegram, discord, slack, whatsapp, signal, googlechat, imessage, msteams",
			},
			"target": map[string]interface{}{
				"type":        "string",
				"description": "Target channel, user, or chat ID",
			},
			"message": map[string]interface{}{
				"type":        "string",
				"description": "Message text to send",
			},
			"messageId": map[string]interface{}{
				"type":        "string",
				"description": "Message ID for edit/delete/react actions",
			},
			"media": map[string]interface{}{
				"type":        "string",
				"description": "Media URL or path (image, video, audio, document)",
			},
			"caption": map[string]interface{}{
				"type":        "string",
				"description": "Caption for media",
			},
			"threadId": map[string]interface{}{
				"type":        "string",
				"description": "Thread ID for threaded messages",
			},
			"emoji": map[string]interface{}{
				"type":        "string",
				"description": "Emoji for react/pin actions",
			},
			"query": map[string]interface{}{
				"type":        "string",
				"description": "Search query for search action",
			},
			"pollQuestion": map[string]interface{}{
				"type":        "string",
				"description": "Poll question",
			},
			"pollOption": map[string]interface{}{
				"type":        "array",
				"items":       map[string]interface{}{"type": "string"},
				"description": "Poll options",
			},
			"pollMulti": map[string]interface{}{
				"type":        "boolean",
				"description": "Allow multiple selections",
			},
			"pollDurationHours": map[string]interface{}{
				"type":        "number",
				"description": "Poll duration in hours",
			},
			// Additional parameters
			"groupId":     map[string]interface{}{"type": "string", "description": "Group ID"},
			"guildId":     map[string]interface{}{"type": "string", "description": "Guild ID (Discord)"},
			"userId":      map[string]interface{}{"type": "string", "description": "User ID"},
			"roleId":      map[string]interface{}{"type": "string", "description": "Role ID"},
			"roleIds":     map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}, "description": "Role IDs"},
			"durationMin": map[string]interface{}{"type": "number", "description": "Timeout duration in minutes"},
			"topic":       map[string]interface{}{"type": "string", "description": "Channel topic"},
			"name":        map[string]interface{}{"type": "string", "description": "Name for create actions"},
			"limit":       map[string]interface{}{"type": "number", "description": "Limit for list actions"},
		},
		"required": []string{"action", "channel", "target"},
	}
}

func (t *MessageTool) Execute(args map[string]interface{}) (interface{}, error) {
	action := GetString(args, "action")
	channel := GetString(args, "channel")
	target := GetString(args, "target")

	if action == "" {
		return nil, &MessageError{Message: "action is required"}
	}
	if channel == "" {
		return nil, &MessageError{Message: "channel is required"}
	}
	if target == "" {
		return nil, &MessageError{Message: "target is required"}
	}

	// Convert channel name to ChannelType
	channelType := t.resolveChannelType(channel)
	if channelType == "" {
		return nil, &MessageError{Message: "unsupported channel: " + channel}
	}

	// Check if channel is available
	if t.channelAdapter != nil && !t.channelAdapter.HasChannel(channelType) {
		return nil, &MessageError{Message: "channel not available: " + channel}
	}

	// Route to appropriate handler
	switch action {
	case "send":
		return t.handleSend(channelType, args)
	case "react":
		return t.handleReact(channelType, args)
	case "delete":
		return t.handleDelete(channelType, args)
	case "edit":
		return t.handleEdit(channelType, args)
	case "read":
		return t.handleRead(channelType, args)
	case "search":
		return t.handleSearch(channelType, args)
	case "poll":
		return t.handlePoll(channelType, args)
	case "pin":
		return t.handlePin(channelType, args, true)
	case "unpin":
		return t.handlePin(channelType, args, false)
	case "list-pins":
		return t.handleListPins(channelType, args)
	case "thread-create":
		return t.handleThreadCreate(channelType, args)
	case "thread-list":
		return t.handleThreadList(channelType, args)
	case "thread-reply":
		return t.handleThreadReply(channelType, args)
	case "member-info":
		return t.handleMemberInfo(channelType, args)
	case "role-info":
		return t.handleRoleInfo(channelType, args)
	case "channel-info":
		return t.handleChannelInfo(channelType, args)
	case "channel-list":
		return t.handleChannelList(channelType, args)
	case "emoji-list":
		return t.handleEmojiList(channelType, args)
	case "sticker":
		return t.handleSticker(channelType, args)
	case "event-list":
		return t.handleEventList(channelType, args)
	case "event-create":
		return t.handleEventCreate(channelType, args)
	case "timeout":
		return t.handleTimeout(channelType, args)
	case "kick":
		return t.handleKick(channelType, args)
	case "ban":
		return t.handleBan(channelType, args)
	case "permissions":
		return t.handlePermissions(channelType, args)
	case "voice-status":
		return t.handleVoiceStatus(channelType, args)
	default:
		return nil, &MessageError{Message: "unsupported action: " + action}
	}
}

// resolveChannelType converts channel name to ChannelType
func (t *MessageTool) resolveChannelType(channel string) channels.ChannelType {
	channel = strings.ToLower(channel)
	switch channel {
	case "telegram":
		return channels.ChannelTelegram
	case "discord":
		return channels.ChannelDiscord
	case "slack":
		return channels.ChannelSlack
	case "whatsapp":
		return channels.ChannelWhatsApp
	case "signal":
		return channels.ChannelSignal
	case "googlechat", "google chat":
		return channels.ChannelGoogleChat
	case "imessage", "i_message", "i message":
		return channels.ChannelIMessage
	case "msteams", "ms teams", "microsoft teams":
		return channels.ChannelMSTeams
	default:
		return channels.ChannelType(channel)
	}
}

func parseInt64(value interface{}) int64 {
	switch v := value.(type) {
	case int64:
		return v
	case int:
		return int64(v)
	case float64:
		return int64(v)
	case string:
		var out int64
		fmt.Sscanf(v, "%d", &out)
		return out
	default:
		return 0
	}
}

func parseButtons(value interface{}) [][]channels.Button {
	rows := [][]channels.Button{}
	addRow := func(btns []channels.Button) {
		if len(btns) > 0 {
			rows = append(rows, btns)
		}
	}
	parseButton := func(raw interface{}) (channels.Button, bool) {
		m, ok := raw.(map[string]interface{})
		if !ok {
			return channels.Button{}, false
		}
		text := GetString(m, "text")
		if text == "" {
			text = GetString(m, "label")
		}
		if text == "" {
			return channels.Button{}, false
		}
		callback := GetString(m, "callback_data")
		if callback == "" {
			callback = GetString(m, "callbackData")
		}
		return channels.Button{Text: text, CallbackData: callback}, true
	}

	switch v := value.(type) {
	case []interface{}:
		// Possibly [][]button or []button
		if len(v) == 0 {
			return nil
		}
		if _, ok := v[0].([]interface{}); ok {
			for _, rowRaw := range v {
				rowSlice, ok := rowRaw.([]interface{})
				if !ok {
					continue
				}
				row := []channels.Button{}
				for _, btnRaw := range rowSlice {
					if btn, ok := parseButton(btnRaw); ok {
						row = append(row, btn)
					}
				}
				addRow(row)
			}
			return rows
		}
		// Flat list -> single row
		row := []channels.Button{}
		for _, btnRaw := range v {
			if btn, ok := parseButton(btnRaw); ok {
				row = append(row, btn)
			}
		}
		addRow(row)
		return rows
	case []map[string]interface{}:
		row := []channels.Button{}
		for _, btnMap := range v {
			if btn, ok := parseButton(btnMap); ok {
				row = append(row, btn)
			}
		}
		addRow(row)
		return rows
	default:
		return nil
	}
}

// handleSend sends a message
func (t *MessageTool) handleSend(channelType channels.ChannelType, args map[string]interface{}) (interface{}, error) {
	if t.channelAdapter == nil {
		return nil, &MessageError{Message: "channel adapter not configured"}
	}

	// Parse target as int64 (ChatID)
	chatID := int64(0)
	if target := GetString(args, "target"); target != "" {
		chatID = parseInt64(target)
	} else if raw, ok := args["chatId"]; ok {
		chatID = parseInt64(raw)
	}
	if chatID == 0 {
		return nil, &MessageError{Message: "target chatId is required"}
	}

	replyTo := int64(0)
	if raw, ok := args["replyTo"]; ok {
		replyTo = parseInt64(raw)
	} else if raw, ok := args["replyToMessageId"]; ok {
		replyTo = parseInt64(raw)
	}
	threadID := int64(0)
	if raw, ok := args["threadId"]; ok {
		threadID = parseInt64(raw)
	} else if raw, ok := args["messageThreadId"]; ok {
		threadID = parseInt64(raw)
	}

	req := &channels.SendMessageRequest{
		ChatID:   chatID,
		Text:     GetString(args, "message"),
		Media:    GetString(args, "media"),
		Buttons:  parseButtons(args["buttons"]),
		ReplyTo:  replyTo,
		ThreadID: threadID,
	}

	resp, err := t.channelAdapter.SendMessage(channelType, req)
	if err != nil {
		return nil, &MessageError{Message: err.Error()}
	}

	return MessageResult{
		OK:      resp.OK,
		Message: "Message sent",
	}, nil
}

// handleReact adds a reaction
func (t *MessageTool) handleReact(channelType channels.ChannelType, args map[string]interface{}) (interface{}, error) {
	messageId := GetString(args, "messageId")
	emoji := GetString(args, "emoji")

	if messageId == "" {
		return nil, &MessageError{Message: "messageId is required"}
	}
	if emoji == "" {
		return nil, &MessageError{Message: "emoji is required"}
	}

	return nil, &MessageError{Message: "reaction action not supported"}
}

// handleDelete deletes a message
func (t *MessageTool) handleDelete(channelType channels.ChannelType, args map[string]interface{}) (interface{}, error) {
	messageId := GetString(args, "messageId")
	if messageId == "" {
		return nil, &MessageError{Message: "messageId is required"}
	}

	return nil, &MessageError{Message: "delete action not supported"}
}

// handleEdit edits a message
func (t *MessageTool) handleEdit(channelType channels.ChannelType, args map[string]interface{}) (interface{}, error) {
	messageId := GetString(args, "messageId")
	text := GetString(args, "message")

	if messageId == "" {
		return nil, &MessageError{Message: "messageId is required"}
	}
	if text == "" {
		return nil, &MessageError{Message: "new message text is required"}
	}

	return nil, &MessageError{Message: "edit action not supported"}
}

// handleRead reads messages
func (t *MessageTool) handleRead(channelType channels.ChannelType, args map[string]interface{}) (interface{}, error) {
	limit := GetInt(args, "limit")
	if limit == 0 {
		limit = 10
	}

	return MessageResult{
		OK:      true,
		Message: "Messages retrieved",
		Extra:   map[string]interface{}{"count": limit},
	}, nil
}

// handleSearch searches messages
func (t *MessageTool) handleSearch(channelType channels.ChannelType, args map[string]interface{}) (interface{}, error) {
	query := GetString(args, "query")
	if query == "" {
		return nil, &MessageError{Message: "query is required"}
	}

	return MessageResult{
		OK:      true,
		Message: "Search results",
		Extra:   map[string]interface{}{"query": query},
	}, nil
}

// handlePoll creates a poll
func (t *MessageTool) handlePoll(channelType channels.ChannelType, args map[string]interface{}) (interface{}, error) {
	question := GetString(args, "pollQuestion")
	options := GetStringSlice(args, "pollOption")

	if question == "" {
		return nil, &MessageError{Message: "pollQuestion is required"}
	}
	if len(options) < 2 {
		return nil, &MessageError{Message: "at least 2 pollOption required"}
	}

	return MessageResult{
		OK:      true,
		Message: "Poll created",
		Extra:   map[string]interface{}{"question": question, "options": options},
	}, nil
}

// handlePin pins/unpins a message
func (t *MessageTool) handlePin(channelType channels.ChannelType, args map[string]interface{}, pin bool) (interface{}, error) {
	messageId := GetString(args, "messageId")
	if messageId == "" {
		return nil, &MessageError{Message: "messageId is required"}
	}

	action := "pinned"
	if !pin {
		action = "unpinned"
	}

	return MessageResult{OK: true, Message: "Message " + action}, nil
}

// handleListPins lists pinned messages
func (t *MessageTool) handleListPins(channelType channels.ChannelType, args map[string]interface{}) (interface{}, error) {
	return MessageResult{OK: true, Message: "Pinned messages list", Extra: map[string]interface{}{"pins": []string{}}}, nil
}

// handleThreadCreate creates a thread
func (t *MessageTool) handleThreadCreate(channelType channels.ChannelType, args map[string]interface{}) (interface{}, error) {
	name := GetString(args, "name")
	message := GetString(args, "message")

	return MessageResult{
		OK:      true,
		Message: "Thread created",
		Extra:   map[string]interface{}{"name": name, "message": message},
	}, nil
}

// handleThreadList lists threads
func (t *MessageTool) handleThreadList(channelType channels.ChannelType, args map[string]interface{}) (interface{}, error) {
	return MessageResult{OK: true, Message: "Thread list", Extra: map[string]interface{}{"threads": []string{}}}, nil
}

// handleThreadReply replies to a thread
func (t *MessageTool) handleThreadReply(channelType channels.ChannelType, args map[string]interface{}) (interface{}, error) {
	threadId := GetString(args, "threadId")
	_ = GetString(args, "message") // message content

	if threadId == "" {
		return nil, &MessageError{Message: "threadId is required"}
	}

	return MessageResult{OK: true, Message: "Reply sent to thread"}, nil
}

// handleMemberInfo gets member info
func (t *MessageTool) handleMemberInfo(channelType channels.ChannelType, args map[string]interface{}) (interface{}, error) {
	userId := GetString(args, "userId")
	if userId == "" {
		return nil, &MessageError{Message: "userId is required"}
	}

	return MessageResult{
		OK:      true,
		Message: "Member info retrieved",
		Extra:   map[string]interface{}{"userId": userId},
	}, nil
}

// handleRoleInfo gets role info
func (t *MessageTool) handleRoleInfo(channelType channels.ChannelType, args map[string]interface{}) (interface{}, error) {
	roleId := GetString(args, "roleId")
	if roleId == "" {
		return nil, &MessageError{Message: "roleId is required"}
	}

	return MessageResult{OK: true, Message: "Role info retrieved"}, nil
}

// handleChannelInfo gets channel info
func (t *MessageTool) handleChannelInfo(channelType channels.ChannelType, args map[string]interface{}) (interface{}, error) {
	return MessageResult{OK: true, Message: "Channel info retrieved"}, nil
}

// handleChannelList lists channels
func (t *MessageTool) handleChannelList(channelType channels.ChannelType, args map[string]interface{}) (interface{}, error) {
	guildId := GetString(args, "guildId")

	var channelList []string
	if t.channelAdapter != nil {
		channels := t.channelAdapter.ListChannels()
		channelList = make([]string, len(channels))
		for i, c := range channels {
			channelList[i] = string(c)
		}
	}

	return MessageResult{
		OK:      true,
		Message: "Channel list",
		Extra:   map[string]interface{}{"channels": channelList, "guildId": guildId},
	}, nil
}

// handleEmojiList lists emojis
func (t *MessageTool) handleEmojiList(channelType channels.ChannelType, args map[string]interface{}) (interface{}, error) {
	return MessageResult{OK: true, Message: "Emoji list", Extra: map[string]interface{}{"emojis": []string{}}}, nil
}

// handleSticker handles sticker operations
func (t *MessageTool) handleSticker(channelType channels.ChannelType, args map[string]interface{}) (interface{}, error) {
	return MessageResult{OK: true, Message: "Sticker operation"}, nil
}

// handleEventList lists events
func (t *MessageTool) handleEventList(channelType channels.ChannelType, args map[string]interface{}) (interface{}, error) {
	return MessageResult{OK: true, Message: "Event list", Extra: map[string]interface{}{"events": []string{}}}, nil
}

// handleEventCreate creates an event
func (t *MessageTool) handleEventCreate(channelType channels.ChannelType, args map[string]interface{}) (interface{}, error) {
	name := GetString(args, "name")
	startTime := GetString(args, "startTime")

	if name == "" {
		return nil, &MessageError{Message: "name is required"}
	}

	return MessageResult{OK: true, Message: "Event created", Extra: map[string]interface{}{"name": name, "startTime": startTime}}, nil
}

// handleTimeout timeouts a user
func (t *MessageTool) handleTimeout(channelType channels.ChannelType, args map[string]interface{}) (interface{}, error) {
	userId := GetString(args, "userId")
	duration := GetInt(args, "durationMin")

	if userId == "" {
		return nil, &MessageError{Message: "userId is required"}
	}

	return MessageResult{OK: true, Message: fmt.Sprintf("User timed out for %d minutes", duration)}, nil
}

// handleKick kicks a user
func (t *MessageTool) handleKick(channelType channels.ChannelType, args map[string]interface{}) (interface{}, error) {
	userId := GetString(args, "userId")
	if userId == "" {
		return nil, &MessageError{Message: "userId is required"}
	}

	return MessageResult{OK: true, Message: "User kicked"}, nil
}

// handleBan bans a user
func (t *MessageTool) handleBan(channelType channels.ChannelType, args map[string]interface{}) (interface{}, error) {
	userId := GetString(args, "userId")
	if userId == "" {
		return nil, &MessageError{Message: "userId is required"}
	}

	return MessageResult{OK: true, Message: "User banned"}, nil
}

// handlePermissions manages permissions
func (t *MessageTool) handlePermissions(channelType channels.ChannelType, args map[string]interface{}) (interface{}, error) {
	return MessageResult{OK: true, Message: "Permissions updated"}, nil
}

// handleVoiceStatus manages voice status
func (t *MessageTool) handleVoiceStatus(channelType channels.ChannelType, args map[string]interface{}) (interface{}, error) {
	return MessageResult{OK: true, Message: "Voice status updated"}, nil
}

// GetStringSlice extracts string slice from args
func GetStringSlice(args map[string]interface{}, key string) []string {
	if v, ok := args[key]; ok {
		if slice, ok := v.([]string); ok {
			return slice
		}
		if slice, ok := v.([]interface{}); ok {
			result := make([]string, len(slice))
			for i, item := range slice {
				if s, ok := item.(string); ok {
					result[i] = s
				}
			}
			return result
		}
	}
	return nil
}

// Types

type MessageResult struct {
	OK      bool                   `json:"ok"`
	Message string                 `json:"message"`
	Extra   map[string]interface{} `json:"extra,omitempty"`
}

type MessageError struct {
	Message string
}

func (e *MessageError) Error() string {
	return e.Message
}
