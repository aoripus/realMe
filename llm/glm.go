package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"realMe/config"
	"time"
)

type GLMClient struct {
	APIKey string
	Model  string
}

type Message struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"` // string or []ContentItem
}

type ContentItem struct {
	Type     string    `json:"type"`
	Text     string    `json:"text,omitempty"`
	ImageURL *ImageURL `json:"image_url,omitempty"`
}

type ImageURL struct {
	URL string `json:"url"`
}

type ChatRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Thinking *Thinking `json:"thinking,omitempty"`
	Stream   bool      `json:"stream,omitempty"`
}

type Thinking struct {
	Type string `json:"type"`
}

type ChatResponse struct {
	Choices []Choice `json:"choices"`
}

type Choice struct {
	Message Message `json:"message"`
}

func NewGLMClient(cfg *config.Config) *GLMClient {
	return &GLMClient{
		APIKey: cfg.GLMApiKey,
		Model:  "glm-4.6v-flash",
	}
}

func (c *GLMClient) Chat(messages []Message) (string, error) {
	reqBody := ChatRequest{
		Model:    c.Model,
		Messages: messages,
		Thinking: &Thinking{Type: "enabled"},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", "https://open.bigmodel.cn/api/paas/v4/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API error: %s", string(body))
	}

	var chatResp ChatResponse
	if err := json.Unmarshal(body, &chatResp); err != nil {
		return "", err
	}

	if len(chatResp.Choices) > 0 {
		content := chatResp.Choices[0].Message.Content
		// Content can be string or array. For assistant responses, it's usually string.
		if strContent, ok := content.(string); ok {
			return strContent, nil
		}
		// Handle complex content if needed
		return fmt.Sprintf("%v", content), nil
	}

	return "", fmt.Errorf("no response choices")
}
