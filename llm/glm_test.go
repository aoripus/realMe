package llm

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRequestPayload(t *testing.T) {
	// This test verifies that the JSON structure matches what the API expects
	messages := []Message{
		{
			Role: "user",
			Content: []ContentItem{
				{Type: "text", Text: "Hello"},
				{Type: "image_url", ImageURL: &ImageURL{URL: "http://example.com/image.jpg"}},
			},
		},
	}
	
	reqBody := ChatRequest{
		Model:    "glm-4.6v-flash",
		Messages: messages,
		Thinking: &Thinking{Type: "enabled"},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatalf("Failed to marshal request: %v", err)
	}

	jsonStr := string(jsonData)
	
	if !strings.Contains(jsonStr, `"thinking":{"type":"enabled"}`) {
		t.Errorf("JSON missing thinking field: %s", jsonStr)
	}
	if !strings.Contains(jsonStr, `"model":"glm-4.6v-flash"`) {
		t.Errorf("JSON missing model field: %s", jsonStr)
	}
}
