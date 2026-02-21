// +build ignore

package main

import (
	"context"
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
	fmt.Println("=== Google Gemini Realtime Test (SDK) ===")
	fmt.Printf("Model: %s\n", model)
	fmt.Printf("API Key: %s\n", apiKey[:15]+"...")

	googleProvider := google.New(llm.Config{Type: llm.ProviderGoogle, APIKey: apiKey, Model: model})
	affective := true
	proactive := false
	cfg := llm.RealtimeConfig{
		Model:                        model,
		APIKey:                       apiKey,
		Voice:                        "Kore",
		EnableAffectiveDialog:        &affective,
		ProactiveAudio:               &proactive,
		InputAudioTranscription:      true,
		OutputAudioTranscription:     true,
		SessionResumption:            true,
		SessionResumptionTransparent: true,
		VADStartSensitivity:          "low",
		VADEndSensitivity:            "low",
		VADSilenceDurationMs:         120,
		MediaResolution:              "medium",
	}
	rt, err := googleProvider.Realtime(context.Background(), cfg)
	if err != nil {
		fmt.Printf("Failed to create Realtime provider: %v\n", err)
		os.Exit(1)
	}

	var wav []byte
	rt.OnText(func(text string) { fmt.Printf("[Text] %s\n", text) })
	rt.OnAudio(func(audio []byte) {
		wav = append([]byte(nil), audio...)
		fmt.Printf("[Audio] Received full WAV %d bytes\n", len(audio))
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

	fmt.Println("\nSending test message...")
	err = rt.SendText(context.Background(), "Hello, please introduce yourself for 10 seconds")
	if err != nil {
		fmt.Printf("Send failed: %v\n", err)
	}

	fmt.Println("\nWaiting for response (12s)... Ctrl+C exit")
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
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

	if len(wav) > 0 {
		out := "/tmp/gemini-live-out.wav"
		_ = os.WriteFile(out, wav, 0644)
		fmt.Printf("Wrote audio: %s (%d bytes WAV)\n", out, len(wav))
	} else {
		fmt.Println("No audio data received")
	}
	fmt.Println("Done!")
}
