// Image Tool - Analyze images using vision models
//
// Supports analyzing images from local paths or URLs using configured image models.
// Works with OpenAI, Anthropic, Google, and other vision-capable providers.

package tools

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type ImageTool struct {
	imageModel   string
	imageAPIKey  string
	imageBaseURL string
	httpClient  *http.Client
}

// ImageToolOption configures the image tool
type ImageToolOption func(*ImageTool)

// WithImageModel sets the image model to use
func WithImageModel(model string) ImageToolOption {
	return func(t *ImageTool) {
		t.imageModel = model
	}
}

// WithImageAPIKey sets the API key for image model calls
func WithImageAPIKey(key string) ImageToolOption {
	return func(t *ImageTool) {
		t.imageAPIKey = key
	}
}

// WithImageBaseURL sets the base URL for image model API
func WithImageBaseURL(url string) ImageToolOption {
	return func(t *ImageTool) {
		t.imageBaseURL = url
	}
}

// NewImageTool creates a new image analysis tool
func NewImageTool(opts ...ImageToolOption) *ImageTool {
	t := &ImageTool{
		imageModel:   "gpt-4o",
		imageBaseURL: "https://api.openai.com/v1",
		httpClient:   &http.Client{},
	}
	for _, opt := range opts {
		opt(t)
	}
	return t
}

func (t *ImageTool) Name() string {
	return "image"
}

func (t *ImageTool) Description() string {
	return "Analyze images using vision models. Supports local paths and URLs. Returns detailed description."
}

func (t *ImageTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"image": map[string]interface{}{
				"type":        "string",
				"description": "Image file path or URL to analyze",
			},
			"prompt": map[string]interface{}{
				"type":        "string",
				"description": "Question about the image (default: 'Describe this image in detail.')",
			},
			"model": map[string]interface{}{
				"type":        "string",
				"description": "Override the default image model",
			},
			"maxBytesMb": map[string]interface{}{
				"type":        "number",
				"description": "Maximum image size in MB (default: 20)",
			},
		},
		"required": []string{"image"},
	}
}

func (t *ImageTool) Execute(args map[string]interface{}) (interface{}, error) {
	imagePath := GetString(args, "image")
	if imagePath == "" {
		return nil, &ImageError{Message: "image path or URL is required"}
	}

	prompt := GetString(args, "prompt")
	if prompt == "" {
		prompt = "Describe this image in detail."
	}

	model := GetString(args, "model")
	if model == "" {
		model = t.imageModel
	}

	maxBytesMb := GetFloat64(args, "maxBytesMb")
	if maxBytesMb == 0 {
		maxBytesMb = 20
	}

	// Load and validate image
	imageData, contentType, err := t.loadImage(imagePath, int(maxBytesMb*1024*1024))
	if err != nil {
		return nil, &ImageError{Message: err.Error()}
	}

	// Determine provider and call appropriate API
	result, err := t.analyzeImage(model, prompt, imageData, contentType)
	if err != nil {
		return nil, &ImageError{Message: err.Error()}
	}

	return ImageResult{
		Model:    model,
		Prompt:   prompt,
		Image:    imagePath,
		Response: result,
	}, nil
}

// loadImage loads an image from path or URL
func (t *ImageTool) loadImage(imagePath string, maxBytes int) ([]byte, string, error) {
	// Check if it's a URL
	if strings.HasPrefix(imagePath, "http://") || strings.HasPrefix(imagePath, "https://") {
		return t.loadFromURL(imagePath, maxBytes)
	}

	// Load from local path
	return t.loadFromFile(imagePath, maxBytes)
}

// loadFromURL fetches image from URL
func (t *ImageTool) loadFromURL(url string, maxBytes int) ([]byte, string, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, "", err
	}

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, int64(maxBytes+1024)))
	if err != nil {
		return nil, "", err
	}

	if len(data) > maxBytes {
		return nil, "", fmt.Errorf("image too large (max %d bytes)", maxBytes)
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "image/jpeg"
	}

	return data, contentType, nil
}

// loadFromFile reads image from local file
func (t *ImageTool) loadFromFile(path string, maxBytes int) ([]byte, string, error) {
	// Jail check
	absPath, err := IsPathAllowed(path)
	if err != nil {
		return nil, "", err
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return nil, "", err
	}
	if info.IsDir() {
		return nil, "", fmt.Errorf("path is a directory")
	}
	if int(info.Size()) > maxBytes {
		return nil, "", fmt.Errorf("image too large (max %d bytes)", maxBytes)
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		return nil, "", err
	}

	ext := strings.ToLower(filepath.Ext(path))
	contentType := map[string]string{
		".jpg":  "image/jpeg",
		".jpeg": "image/jpeg",
		".png":  "image/png",
		".gif":  "image/gif",
		".webp": "image/webp",
		".bmp":  "image/bmp",
	}[ext]
	if contentType == "" {
		contentType = "image/jpeg"
	}

	return data, contentType, nil
}

// analyzeImage calls the appropriate vision API
func (t *ImageTool) analyzeImage(model, prompt string, imageData []byte, contentType string) (string, error) {
	// Determine provider based on model name
	modelLower := strings.ToLower(model)

	if strings.Contains(modelLower, "gpt") || strings.Contains(modelLower, "openai") {
		return t.callOpenAI(model, prompt, imageData, contentType)
	} else if strings.Contains(modelLower, "claude") || strings.Contains(modelLower, "anthropic") {
		return t.callAnthropic(model, prompt, imageData, contentType)
	} else if strings.Contains(modelLower, "gemini") || strings.Contains(modelLower, "google") {
		return t.callGoogleVision(model, prompt, imageData, contentType)
	}

	// Default to OpenAI-compatible API
	return t.callOpenAI(model, prompt, imageData, contentType)
}

// callOpenAI calls OpenAI vision API
func (t *ImageTool) callOpenAI(model, prompt string, imageData []byte, contentType string) (string, error) {
	// Encode image as base64
	encoded := base64.StdEncoding.EncodeToString(imageData)
	dataURL := fmt.Sprintf("data:%s;base64,%s", contentType, encoded)

	// Build request
	type MessageContent struct {
		Type     string `json:"type"`
		Text     string `json:"text"`
		ImageURL *struct {
			URL string `json:"url"`
		} `json:"image_url,omitempty"`
	}

	msgContent := MessageContent{
		Type: "text",
		Text: prompt,
	}
	msgContent.ImageURL = &struct {
		URL string `json:"url"`
	}{URL: dataURL}

	reqBody := map[string]interface{}{
		"model": model,
		"messages": []map[string]interface{}{
			{
				"role":    "user",
				"content": []interface{}{msgContent},
			},
		},
		"max_tokens": 4096,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	// Use configured base URL or default
	baseURL := t.imageBaseURL
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}

	req, err := http.NewRequest("POST", baseURL+"/chat/completions", bytes.NewReader(bodyBytes))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	if t.imageAPIKey != "" {
		req.Header.Set("Authorization", "Bearer "+t.imageAPIKey)
	}

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API error %d: %s", resp.StatusCode, string(respBody))
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	// Extract response
	choices, ok := result["choices"].([]map[string]interface{})
	if !ok || len(choices) == 0 {
		return "", fmt.Errorf("no response from API")
	}

	message, ok := choices[0]["message"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("invalid response format")
	}

	content, ok := message["content"].(string)
	if !ok {
		return "", fmt.Errorf("no content in response")
	}

	return content, nil
}

// callAnthropic calls Anthropic vision API (Claude)
func (t *ImageTool) callAnthropic(model, prompt string, imageData []byte, contentType string) (string, error) {
	encoded := base64.StdEncoding.EncodeToString(imageData)

	reqBody := map[string]interface{}{
		"model": model,
		"max_tokens": 4096,
		"messages": []map[string]interface{}{
			{
				"role": "user",
				"content": []map[string]interface{}{
					{
						"type":      "image",
						"source": map[string]string{
							"type":      "base64",
							"media_type": contentType,
							"data":      encoded,
						},
					},
					{
						"type": "text",
						"text": prompt,
					},
				},
			},
		},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", "https://api.anthropic.com/v1/messages", bytes.NewReader(bodyBytes))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", t.imageAPIKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API error %d: %s", resp.StatusCode, string(respBody))
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	content, ok := result["content"].([]interface{})
	if !ok || len(content) == 0 {
		return "", fmt.Errorf("no response content")
	}

	// Handle different response formats
	contentMap, ok := content[0].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("invalid content format")
	}

	text, ok := contentMap["text"].(string)
	if !ok {
		return "", fmt.Errorf("no text in response")
	}

	return text, nil
}

// callGoogleVision calls Google Gemini Vision API
func (t *ImageTool) callGoogleVision(model, prompt string, imageData []byte, contentType string) (string, error) {
	encoded := base64.StdEncoding.EncodeToString(imageData)

	reqBody := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"parts": []map[string]interface{}{
					{
						"inline_data": map[string]string{
							"mime_type": contentType,
							"data":      encoded,
						},
					},
					{
						"text": prompt,
					},
				},
			},
		},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	// Extract model name (e.g., gemini-pro-vision -> gemini-1.5-pro)
	apiModel := "gemini-1.5-pro"
	if strings.Contains(model, "1.5") {
		apiModel = "gemini-1.5-pro"
	}

	req, err := http.NewRequest("POST", 
		"https://generativelanguage.googleapis.com/v1beta/models/"+apiModel+":generateContent?key="+t.imageAPIKey,
		bytes.NewReader(bodyBytes))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API error %d: %s", resp.StatusCode, string(respBody))
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	candidates, ok := result["candidates"].([]map[string]interface{})
	if !ok || len(candidates) == 0 {
		return "", fmt.Errorf("no response from API")
	}

	content, ok := candidates[0]["content"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("invalid response format")
	}

	parts, ok := content["parts"].([]map[string]interface{})
	if !ok || len(parts) == 0 {
		return "", fmt.Errorf("no content parts")
	}

	text, ok := parts[0]["text"].(string)
	if !ok {
		return "", fmt.Errorf("no text in response")
	}

	return text, nil
}

// Types

type ImageResult struct {
	Model    string `json:"model"`
	Prompt   string `json:"prompt"`
	Image    string `json:"image"`
	Response string `json:"response"`
}

type ImageError struct {
	Message string
}

func (e *ImageError) Error() string {
	return e.Message
}
