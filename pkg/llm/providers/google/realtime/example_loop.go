// +build ignore

package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
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
	fmt.Println("=== Voice Loop Test (TTS -> Gemini Live -> Whisper) ===")
	fmt.Printf("Model: %s\n", model)

	googleProvider := google.New(llm.Config{Type: llm.ProviderGoogle, APIKey: apiKey, Model: model})
	affective := true
	cfg := llm.RealtimeConfig{
		Model:                   model,
		APIKey:                  apiKey,
		Voice:                   "Kore",
		EnableAffectiveDialog:   &affective,
		InputAudioTranscription: true, // Enable input transcription
	}
	rt, err := googleProvider.Realtime(context.Background(), cfg)
	if err != nil {
		fmt.Printf("Failed to create Realtime provider: %v\n", err)
		os.Exit(1)
	}

	var replyAudio []byte

	// Receive transcription (input)
	rt.OnTranscription(func(result llm.TranscriptionResult) {
		fmt.Printf("[Input transcription] %s\n", result.Text)
	})

	// Receive text output
	rt.OnText(func(text string) {
		fmt.Printf("[Output text] %s\n", text)
	})

	// Receive audio output
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

	// ===== Round 1: English Test =====
	prompt := "Hello! Please introduce yourself in English, keep it short."

	fmt.Println("\n=== Round 1: English Test ===")
	fmt.Printf("Sending: %s\n", prompt)
	err = rt.SendText(context.Background(), prompt)
	if err != nil {
		fmt.Printf("Send failed: %v\n", err)
	}

	// Waiting for response
	ctx1, cancel1 := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel1()
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-ctx1.Done():
		fmt.Println("\nRound 1 timeout")
	case <-sigChan:
		fmt.Println("\nInterrupt signal received")
	}

	// Transcribe Output audio with Whisper
	if len(replyAudio) > 0 {
		fmt.Println("\nTranscribe Output audio with Whisper...")
		outFile := "/tmp/gemini_reply.wav"
		_ = os.WriteFile(outFile, replyAudio, 0644)

		// Run whisper
		cmd := exec.Command("whisper", outFile, "--model", "base", "--language", "English", "--output_format", "txt")
		output, err := cmd.CombinedOutput()
		if err != nil {
			fmt.Printf("Whisper transcription failed: %v\n", err)
		} else {
			fmt.Printf("[Whisper transcription result]\n%s\n", string(output))
		}
	}

	// ===== Round 2: Continue conversation based on reply =====
	fmt.Println("\n=== Round 2: Follow-up ===")
	replyPrompt := "That's great! What can you help me with?"

	replyAudio = nil // Reset

	fmt.Printf("Sending: %s\n", replyPrompt)
	err = rt.SendText(context.Background(), replyPrompt)
	if err != nil {
		fmt.Printf("Send failed: %v\n", err)
	}

	ctx2, cancel2 := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel2()
	select {
	case <-ctx2.Done():
		fmt.Println("\nRound 2 timeout")
	case <-sigChan:
		fmt.Println("\nInterrupt signal received")
	}

	if len(replyAudio) > 0 {
		fmt.Println("\nTranscribe round 2 audio with Whisper...")
		outFile := "/tmp/gemini_reply2.wav"
		_ = os.WriteFile(outFile, replyAudio, 0644)

		cmd := exec.Command("whisper", outFile, "--model", "base", "--language", "English", "--output_format", "txt")
		output, err := cmd.CombinedOutput()
		if err != nil {
			fmt.Printf("Whisper transcription failed: %v\n", err)
		} else {
			fmt.Printf("[Whisper transcription result]\n%s\n", string(output))
		}
	}

	fmt.Println("\nDisconnect...")
	_ = rt.Disconnect()
	fmt.Println("Done!")
}
