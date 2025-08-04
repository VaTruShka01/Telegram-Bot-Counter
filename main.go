package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"telegram-expense-bot/internal/config"
	"telegram-expense-bot/internal/database"
	"telegram-expense-bot/internal/handlers"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/robfig/cron/v3"
)

func main() {
	// Load configuration
	cfg := config.Load()

	// Initialize database
	ctx := context.Background()
	db, err := database.New(ctx, cfg.MongoURI, cfg.MongoDB, "transactions")
	if err != nil {
		log.Fatal("Failed to initialize MongoDB:", err)
	}
	defer db.Close(ctx)

	// Create Telegram bot
	bot, err := tgbotapi.NewBotAPI(cfg.TelegramToken)
	if err != nil {
		log.Fatal("Failed to create Telegram bot:", err)
	}

	bot.Debug = false
	log.Printf("Bot started: %s", bot.Self.UserName)

	// Set up handlers
	eventHandler := handlers.NewEventHandler(db, cfg)
	commandHandler := handlers.NewCommandHandler(db, cfg)

	// Set up cron job for monthly reset
	c := cron.New()
	_, err = c.AddFunc("0 9 1 * *", func() {
		log.Println("Executing monthly reset...")
		commandHandler.MonthlyReset(bot)
	})
	if err != nil {
		log.Fatal("Failed to add cron job:", err)
	}
	c.Start()

	fmt.Println("Bot is running...")

	// Start listening for updates
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := bot.GetUpdatesChan(u)

	// Handle updates
	go func() {
		for update := range updates {
			if update.Message != nil {
				eventHandler.HandleMessage(bot, update.Message)
			} else if update.EditedMessage != nil {
				eventHandler.HandleMessage(bot, update.EditedMessage)
			} else if update.CallbackQuery != nil {
				eventHandler.HandleCallbackQuery(bot, update.CallbackQuery)
			}
		}
	}()

	// Wait for interrupt signal
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-stop

	fmt.Println("Shutting down bot...")
}