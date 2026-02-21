// +build ignore

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gliderlab/cogate/pkg/llm"
	"github.com/gliderlab/cogate/pkg/llm/providers/google"
)

func main() {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" || apiKey == "your_api_key_here" {
		apiKey = os.Getenv("GOOGLE_API_KEY")
	}
	if apiKey == "" || apiKey == "your_api_key_here" {
		apiKey = "YOUR_API_KEY_HERE"
	}
	os.Setenv("GEMINI_API_KEY", apiKey)

	model := "models/gemini-2.5-flash-native-audio-preview-12-2025"
	fmt.Println("=== Function Calling Test ===")
	fmt.Printf("Model: %s\n", model)

	// Define a simple weather tool
	tools := []llm.Tool{
		{
			Type: "function",
			Function: &llm.ToolFunction{
				Name:        "get_weather",
				Description: "Get the current weather in a location",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"location": map[string]interface{}{
							"type":        "string",
							"description": "The city to get weather for",
						},
						"unit": map[string]interface{}{
							"type": "string",
							"enum":     []string{"celsius", "fahrenheit"},
							"default":   "celsius",
							"description": "Temperature unit",
						},
					},
					"required": []string{"location"},
				},
			},
		},
		{
			Type: "function",
			Function: &llm.ToolFunction{
				Name:        "get_time",
				Description: "Get the current time for a timezone",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"timezone": map[string]interface{}{
							"type":        "string",
							"description": "IANA timezone (e.g., Asia/Shanghai, America/New_York)",
						},
					},
					"required": []string{"timezone"},
				},
			},
		},
	}

	googleProvider := google.New(llm.Config{Type: llm.ProviderGoogle, APIKey: apiKey, Model: model})
	affective := true
	cfg := llm.RealtimeConfig{
		Model:                 model,
		APIKey:                apiKey,
		Voice:                 "Kore",
		EnableAffectiveDialog: &affective,
		Tools:                 tools,
	}
	rt, err := googleProvider.Realtime(context.Background(), cfg)
	if err != nil {
		fmt.Printf("Failed to create Realtime provider: %v\n", err)
		os.Exit(1)
	}

	var replyAudio []byte

	// Receive tool call
	rt.OnToolCall(func(toolCall llm.ToolCall) {
		fmt.Printf("\n[Tool call!] ID: %s, Name: %s\n", toolCall.ID, toolCall.Function.Name)
		args, _ := json.MarshalIndent(toolCall.Function.Parameters, "", "  ")
		fmt.Printf("Args: %s\n", string(args))

		// Simulate tool execution result
		var result string
		params := toolCall.Function.Parameters
		switch toolCall.Function.Name {
		case "get_weather":
			location := ""
			if paramsMap, ok := params.(map[string]interface{}); ok {
				if loc, ok := paramsMap["location"].(string); ok {
					location = loc
				}
			}
			result = fmt.Sprintf(`{"location": "%s", "temperature": 22, "condition": "sunny"}`, location)
		case "get_time":
			tz := "UTC"
			if paramsMap, ok := params.(map[string]interface{}); ok {
				if t, ok := paramsMap["timezone"].(string); ok {
					tz = t
				}
			}
			result = fmt.Sprintf(`{"timezone": "%s", "time": "2024-01-01T12:00:00Z"}`, tz)
		default:
			result = `{"error": "unknown function"}`
		}

		fmt.Printf("\nSend tool response: %s\n", result)
		err := rt.SendToolResponse(context.Background(), llm.ToolResponse{
			ID:     toolCall.ID,
			Name:   toolCall.Function.Name,
			Result: result,
		})
		if err != nil {
			fmt.Printf("Send tool response failed: %v\n", err)
		}

		// Send empty message to continue
		fmt.Println("Send continue signal...")
		err = rt.SendText(context.Background(), "Continue based on tool result")
		if err != nil {
			fmt.Printf("Send continue signal... %v\n", err)
		}
	})

	// Receive final text (debug)
	rt.OnText(func(text string) {
		fmt.Printf("[Final text reply] %s\n", text)
	})

	// Receive final response
	rt.OnAudio(func(audio []byte) {
		replyAudio = append([]byte(nil), audio...)
		fmt.Printf("[Output audio] received %d bytes WAV\n", len(audio))
	})

	rt.OnError(func(err error) { fmt.Printf("[Error] %v\n", err) })
	rt.OnDisconnect(func() { fmt.Println("[Disconnect]") })

	fmt.Println("\nConnecting...")
	err = rt.Connect(context.Background(), cfg)
	if err != nil {
		fmt.Printf("Connection failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Connection success!")

	// Send request requiring tool call - just ask weather
	prompt := "What's the weather like in Tokyo?"

	fmt.Printf("\nSending: %s\n", prompt)
	err = rt.SendText(context.Background(), prompt)
	if err != nil {
		fmt.Printf("Send failed: %v\n", err)
	}

	// Waiting for response
	fmt.Println("\nWaiting for response (30s)...")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	select {
	case <-ctx.Done():
		fmt.Println("\nTimeout")
	case <-sigChan:
		fmt.Println("\nInterrupt signal received")
	}

	fmt.Println("\nDisconnect...")
	_ = rt.Disconnect()

	if len(replyAudio) > 0 {
		out := "/tmp/function_call_reply.wav"
		_ = os.WriteFile(out, replyAudio, 0644)
		fmt.Printf("Wrote audio: %s (%d bytes)\n", out, len(replyAudio))
	}
	fmt.Println("Done!")
}
