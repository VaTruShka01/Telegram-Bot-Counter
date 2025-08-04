package config

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

// Config holds all configuration for the application
type Config struct {
	TelegramToken string
	MongoURI      string
	MongoDB       string
	ChatID        int64
	Categories    []string
}

// Load loads configuration from environment variables
func Load() *Config {
	err := godotenv.Load()
	if err != nil {
		log.Println("No .env file found or error loading it.")
	}

	chatIDStr := os.Getenv("TELEGRAM_CHAT_ID")
	chatID, err := strconv.ParseInt(chatIDStr, 10, 64)
	if err != nil {
		log.Fatal("Invalid TELEGRAM_CHAT_ID:", err)
	}

	config := &Config{
		TelegramToken: os.Getenv("TELEGRAM_BOT_TOKEN"),
		MongoURI:      os.Getenv("MONGODB_URI"),
		MongoDB:       os.Getenv("MONGODB_DB"),
		ChatID:        chatID,
		Categories: []string{
			"Groceries 🛒",
			"Household 🏠",
			"Entertainment 🎉",
			"LCBO 🥂",
			"Dining Out 🍽️",
			"Other 🗂️",
		},
	}

	// Validate required fields
	if config.TelegramToken == "" {
		log.Fatal("TELEGRAM_BOT_TOKEN not set")
	}
	if config.MongoURI == "" {
		log.Fatal("MONGODB_URI not set")
	}
	if config.MongoDB == "" {
		log.Fatal("MONGODB_DB not set")
	}
	if config.ChatID == 0 {
		log.Fatal("TELEGRAM_CHAT_ID not set")
	}

	return config
}

// IsAuthorizedUser checks if the user is in the configured chat (always true since we already filter by chat ID)
func (c *Config) IsAuthorizedUser(username string, chatID int64) bool {
	// If the message is from the configured chat, the user is authorized
	return chatID == c.ChatID
}