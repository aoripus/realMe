package config

import (
	"os"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	// Set environment variables
	os.Setenv("TELEGRAM_APP_ID", "123456")
	os.Setenv("TELEGRAM_APP_HASH", "test_hash")
	os.Setenv("TARGET_GROUP_ID", "987654321")
	os.Setenv("GLM_API_KEY", "test_key")

	defer func() {
		os.Unsetenv("TELEGRAM_APP_ID")
		os.Unsetenv("TELEGRAM_APP_HASH")
		os.Unsetenv("TARGET_GROUP_ID")
		os.Unsetenv("GLM_API_KEY")
	}()

	cfg := LoadConfig()

	if cfg.TelegramAppID != 123456 {
		t.Errorf("Expected TelegramAppID 123456, got %d", cfg.TelegramAppID)
	}
	if cfg.TelegramAppHash != "test_hash" {
		t.Errorf("Expected TelegramAppHash 'test_hash', got %s", cfg.TelegramAppHash)
	}
	if cfg.TargetGroupID != 987654321 {
		t.Errorf("Expected TargetGroupID 987654321, got %d", cfg.TargetGroupID)
	}
	if cfg.GLMApiKey != "test_key" {
		t.Errorf("Expected GLMApiKey 'test_key', got %s", cfg.GLMApiKey)
	}
}
