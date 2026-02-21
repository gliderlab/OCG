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
	fmt.Println("=== Google Gemini Realtime Bidirectional Voice Test ===")
	fmt.Printf("Model: %s\n", model)

	googleProvider := google.New(llm.Config{Type: llm.ProviderGoogle, APIKey: apiKey, Model: model})
	affective := true
	cfg := llm.RealtimeConfig{
		Model:                 model,
		APIKey:                apiKey,
		Voice:                 "Kore",
		EnableAffectiveDialog: &affective,
		InputAudioTranscription: true,
	}
	rt, err := googleProvider.Realtime(context.Background(), cfg)
	if err != nil {
		fmt.Printf("Failed to create Realtime provider: %v\n", err)
		os.Exit(1)
	}

	// Receive model output audio
	rt.OnAudio(func(audio []byte) {
		fmt.Printf("[Output audio] received %d bytes WAV\n", len(audio))
		// Here you can write audio to file or play
		_ = os.WriteFile("/tmp/reply.wav", audio, 0644)
	})

	// Receive model output text
	rt.OnText(func(text string) {
		fmt.Printf("[Output text] %s\n", text)
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

	// Simulate mic input (actually need to read mic)
	// Use short silence PCM to simulate
	sampleRate := 24000
	durationMs := 500 // 500ms
	silentPCM := make([]byte, sampleRate*durationMs/1000*2) // 16-bit mono

	fmt.Println("\nSending simulated mic audio (2s silence)...")
	err = rt.SendAudio(context.Background(), silentPCM)
	if err != nil {
		fmt.Printf("Failed to send audio: %v\n", err)
	} else {
		fmt.Println("Audio sent")
	}

	// End audio stream
	fmt.Println("End audio stream...")
	err = rt.EndAudio(context.Background())
	if err != nil {
		fmt.Printf("End audio stream failed: %v\n", err)
	}

	// Send text prompt
	fmt.Println("\nSend text prompt...")
	err = rt.SendText(context.Background(), "Hello, I just sent 2 seconds of audio, please reply")
	if err != nil {
		fmt.Printf("Failed to send text: %v\n", err)
	}

	// Waiting for response
	fmt.Println("\nWaiting for response (10s)...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
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
	fmt.Println("Done!")
}
