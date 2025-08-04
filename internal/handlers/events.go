package handlers

import (
	"context"
	"fmt"
	"log"
	"math"
	"strconv"
	"strings"
	"time"

	"telegram-expense-bot/internal/config"
	"telegram-expense-bot/internal/database"
	"telegram-expense-bot/internal/models"
	"telegram-expense-bot/internal/utils"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.mongodb.org/mongo-driver/bson"
)

// EventHandler handles Telegram events
type EventHandler struct {
	db       *database.DB
	config   *config.Config
	commands *CommandHandler
}

// NewEventHandler creates a new event handler
func NewEventHandler(db *database.DB, config *config.Config) *EventHandler {
	return &EventHandler{
		db:       db,
		config:   config,
		commands: NewCommandHandler(db, config),
	}
}

// HandleMessage handles incoming messages
func (h *EventHandler) HandleMessage(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	// Ignore messages from bots
	if message.From.IsBot {
		return
	}

	// Only process messages from the configured chat
	if message.Chat.ID != h.config.ChatID {
		return
	}

	// Handle commands
	if message.IsCommand() {
		h.handleCommand(bot, message)
		return
	}

	// Handle edited messages
	if message.EditDate != 0 {
		h.handleEditedMessage(bot, message)
		return
	}

	// Check if user is authorized (any user in the configured chat is authorized)
	if !h.config.IsAuthorizedUser(message.From.UserName, message.Chat.ID) {
		return
	}

	// Try to parse as transaction amount
	h.handleNewTransaction(bot, message)
}

// handleCommand processes bot commands
func (h *EventHandler) handleCommand(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	switch message.Command() {
	case "totals":
		h.commands.SendTotals(bot, message.Chat.ID)
	case "reset":
		h.commands.ResetDatabase(bot, message.Chat.ID)
	case "help", "start":
		h.commands.SendHelp(bot, message.Chat.ID)
	case "history":
		h.commands.SendTransactionHistory(bot, message.Chat.ID, 0) // 0 means all transactions
	case "compare":
		h.commands.SendMonthlyComparison(bot, message.Chat.ID)
	case "trends":
		h.commands.SendSpendingTrends(bot, message.Chat.ID)
	case "export":
		h.commands.ExportMonthlyData(bot, message.Chat.ID, message.Text)
	}
}

// handleNewTransaction processes a new transaction
func (h *EventHandler) handleNewTransaction(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	amount, err := utils.ValidateAmount(message.Text)
	if err != nil {
		// Not a valid amount, ignore
		return
	}

	ctx := context.Background()

	// Create transaction ID from message ID
	transactionID := strconv.Itoa(message.MessageID)

	// Create a new transaction
	tx := &models.Transaction{
		ID:     transactionID,
		Amount: amount,
		Author: message.From.UserName,
	}

	err = h.db.InsertTransaction(ctx, tx)
	if err != nil {
		log.Println("Failed to insert transaction:", err)
		msg := tgbotapi.NewMessage(message.Chat.ID, "Failed to save transaction in DB.")
		bot.Send(msg)
		return
	}

	h.sendCategorySelection(bot, message.Chat.ID, transactionID)
}

// sendCategorySelection sends category selection inline keyboard
func (h *EventHandler) sendCategorySelection(bot *tgbotapi.BotAPI, chatID int64, transactionID string) {
	keyboard := utils.BuildInlineKeyboard(h.config.Categories, transactionID)

	msg := tgbotapi.NewMessage(chatID, "Select a category:")
	msg.ReplyMarkup = keyboard

	sentMsg, err := bot.Send(msg)
	if err != nil {
		log.Println("Failed to send category selection:", err)
		return
	}

	// Store the button message ID in the database
	ctx := context.Background()
	buttonMsgID := strconv.Itoa(sentMsg.MessageID)
	err = h.db.UpdateTransaction(ctx, transactionID, bson.M{"buttonMessageId": buttonMsgID})
	if err != nil {
		log.Println("Failed to update buttonMessageId in DB:", err)
	}
}

// HandleCallbackQuery handles inline button callbacks
func (h *EventHandler) HandleCallbackQuery(bot *tgbotapi.BotAPI, callback *tgbotapi.CallbackQuery) {
	// Only process callbacks from the configured chat
	if callback.Message.Chat.ID != h.config.ChatID {
		return
	}

	if strings.HasPrefix(callback.Data, "category_") {
		h.handleCategorySelection(bot, callback)
	} else if strings.HasPrefix(callback.Data, "delete_") {
		h.handleTransactionDeletion(bot, callback)
	}

	// Answer the callback to remove loading state
	callbackConfig := tgbotapi.NewCallback(callback.ID, "")
	bot.Request(callbackConfig)
}

// handleCategorySelection processes category selection
func (h *EventHandler) handleCategorySelection(bot *tgbotapi.BotAPI, callback *tgbotapi.CallbackQuery) {
	parts := strings.Split(callback.Data, "_")
	if len(parts) < 3 {
		return
	}

	newCategory := parts[1]
	transactionID := parts[2]

	ctx := context.Background()
	tx, err := h.db.FindTransaction(ctx, transactionID)
	if err != nil || tx == nil {
		log.Println("Transaction not found:", err)
		return
	}

	// Delete old confirmation message if exists
	if tx.ConfirmationMessageID != "" {
		confirmMsgID, _ := strconv.Atoi(tx.ConfirmationMessageID)
		deleteMsg := tgbotapi.NewDeleteMessage(callback.Message.Chat.ID, confirmMsgID)
		bot.Request(deleteMsg)
	}

	// Update transaction category
	err = h.db.UpdateTransaction(ctx, transactionID, bson.M{"category": newCategory})
	if err != nil {
		log.Println("Failed to update category in DB:", err)
		return
	}

	// Update the category selection message to show confirmation and allow re-selection
	content := fmt.Sprintf("✅ Added %.2f$ to %s category.\n\nTap a different category to change:", math.Abs(tx.Amount), newCategory)
	
	// Rebuild the keyboard with the updated transaction
	keyboard := utils.BuildInlineKeyboard(h.config.Categories, transactionID)
	
	editMsg := tgbotapi.NewEditMessageText(callback.Message.Chat.ID, callback.Message.MessageID, content)
	editMsg.ReplyMarkup = &keyboard
	
	_, err = bot.Send(editMsg)
	if err != nil {
		log.Println("Failed to update category selection message:", err)
	}
}

// handleTransactionDeletion handles transaction deletion via callback
func (h *EventHandler) handleTransactionDeletion(bot *tgbotapi.BotAPI, callback *tgbotapi.CallbackQuery) {
	parts := strings.Split(callback.Data, "_")
	if len(parts) < 2 {
		return
	}

	transactionID := parts[1]
	ctx := context.Background()

	// Find the transaction first to get details
	tx, err := h.db.FindTransaction(ctx, transactionID)
	if err != nil || tx == nil {
		// Clean up the callback message since transaction doesn't exist
		deleteMsg := tgbotapi.NewDeleteMessage(callback.Message.Chat.ID, callback.Message.MessageID)
		bot.Request(deleteMsg)
		return
	}

	// Delete from database
	err = h.db.DeleteTransaction(ctx, transactionID)
	if err != nil {
		log.Println("Failed to delete transaction from DB:", err)
		return
	}

	// Delete the category selection message
	deleteMsg := tgbotapi.NewDeleteMessage(callback.Message.Chat.ID, callback.Message.MessageID)
	bot.Request(deleteMsg)

	// Delete confirmation message if it exists
	if tx.ConfirmationMessageID != "" {
		confirmMsgID, _ := strconv.Atoi(tx.ConfirmationMessageID)
		deleteConfirmMsg := tgbotapi.NewDeleteMessage(callback.Message.Chat.ID, confirmMsgID)
		bot.Request(deleteConfirmMsg)
	}

	// Send deletion confirmation
	content := fmt.Sprintf("🗑️ Deleted transaction: %.2f$", math.Abs(tx.Amount))
	confirmMsg := tgbotapi.NewMessage(callback.Message.Chat.ID, content)
	sentConfirm, err := bot.Send(confirmMsg)
	if err == nil {
		// Auto-delete the confirmation message after 5 seconds
		go func() {
			time.Sleep(5 * time.Second)
			deleteConfirm := tgbotapi.NewDeleteMessage(callback.Message.Chat.ID, sentConfirm.MessageID)
			bot.Request(deleteConfirm)
		}()
	}

}

// handleEditedMessage handles message edits
func (h *EventHandler) handleEditedMessage(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	// Check if user is authorized
	if !h.config.IsAuthorizedUser(message.From.UserName, message.Chat.ID) {
		return
	}

	// Parse new amount
	newAmount, err := utils.ValidateAmount(message.Text)
	if err != nil {
		// Not a valid amount, ignore
		return
	}

	ctx := context.Background()
	transactionID := strconv.Itoa(message.MessageID)

	// Find existing transaction
	tx, err := h.db.FindTransaction(ctx, transactionID)
	if err != nil || tx == nil {
		// Transaction not found
		return
	}

	// Update transaction amount
	err = h.db.UpdateTransaction(ctx, transactionID, bson.M{"amount": newAmount})
	if err != nil {
		log.Println("Failed to update transaction amount:", err)
		return
	}

	// Update category selection message if it exists and has a category
	if tx.ButtonMessageID != "" {
		buttonMsgID, _ := strconv.Atoi(tx.ButtonMessageID)
		if tx.Category != "" {
			// Update with confirmation and keep buttons
			content := fmt.Sprintf("✅ Updated to %.2f$ in %s category.\n\nTap a different category to change:", math.Abs(newAmount), tx.Category)
			keyboard := utils.BuildInlineKeyboard(h.config.Categories, transactionID)
			editMsg := tgbotapi.NewEditMessageText(message.Chat.ID, buttonMsgID, content)
			editMsg.ReplyMarkup = &keyboard
			bot.Send(editMsg)
		} else {
			// No category selected yet, just update the selection message
			content := "Select a category:"
			keyboard := utils.BuildInlineKeyboard(h.config.Categories, transactionID)
			editMsg := tgbotapi.NewEditMessageText(message.Chat.ID, buttonMsgID, content)
			editMsg.ReplyMarkup = &keyboard
			bot.Send(editMsg)
		}
	}
}