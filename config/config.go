package config

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	TelegramAppID    int
	TelegramAppHash  string
	TargetGroupID    int64
	GLMApiKey        string
}

func LoadConfig() *Config {
	err := godotenv.Load()
	if err != nil {
		log.Println("Error loading .env file, using system env vars")
	}

	appIDStr := os.Getenv("TELEGRAM_APP_ID")
	appID, err := strconv.Atoi(appIDStr)
	if err != nil {
		log.Fatal("TELEGRAM_APP_ID is not a valid integer")
	}

	groupIDStr := os.Getenv("TARGET_GROUP_ID")
	groupID, err := strconv.ParseInt(groupIDStr, 10, 64)
	if err != nil {
		log.Println("TARGET_GROUP_ID is not set or invalid, using 0 (all groups/chats)")
		groupID = 0
	}

	return &Config{
		TelegramAppID:   appID,
		TelegramAppHash: os.Getenv("TELEGRAM_APP_HASH"),
		TargetGroupID:   groupID,
		GLMApiKey:       os.Getenv("GLM_API_KEY"),
	}
}
