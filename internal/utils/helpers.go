package utils

import (
	"fmt"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// ValidateAmount validates and parses amount from string
func ValidateAmount(text string) (float64, error) {
	text = strings.TrimSpace(text)
	
	// Try to parse as float
	amount, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid amount format")
	}
	
	if amount <= 0 {
		return 0, fmt.Errorf("amount must be positive")
	}
	
	return amount, nil
}

// BuildInlineKeyboard builds inline keyboard for category selection
func BuildInlineKeyboard(categories []string, messageID string) tgbotapi.InlineKeyboardMarkup {
	var rows [][]tgbotapi.InlineKeyboardButton
	
	// Create 2 buttons per row for categories
	for i := 0; i < len(categories); i += 2 {
		var row []tgbotapi.InlineKeyboardButton
		
		// First button
		btn1 := tgbotapi.NewInlineKeyboardButtonData(
			categories[i],
			fmt.Sprintf("category_%s_%s", categories[i], messageID),
		)
		row = append(row, btn1)
		
		// Second button if exists
		if i+1 < len(categories) {
			btn2 := tgbotapi.NewInlineKeyboardButtonData(
				categories[i+1],
				fmt.Sprintf("category_%s_%s", categories[i+1], messageID),
			)
			row = append(row, btn2)
		}
		
		rows = append(rows, row)
	}
	
	// Add delete button as a separate row
	deleteBtn := tgbotapi.NewInlineKeyboardButtonData(
		"🗑️ Delete Transaction",
		fmt.Sprintf("delete_%s", messageID),
	)
	deleteRow := []tgbotapi.InlineKeyboardButton{deleteBtn}
	rows = append(rows, deleteRow)
	
	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}