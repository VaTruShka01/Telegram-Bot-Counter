package handlers

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"math"
	"strings"
	"time"

	"telegram-expense-bot/internal/config"
	"telegram-expense-bot/internal/database"
	"telegram-expense-bot/internal/models"
	"telegram-expense-bot/internal/utils"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// CommandHandler handles bot commands
type CommandHandler struct {
	db     *database.DB
	config *config.Config
}

// NewCommandHandler creates a new command handler
func NewCommandHandler(db *database.DB, config *config.Config) *CommandHandler {
	return &CommandHandler{
		db:     db,
		config: config,
	}
}

// SendTotals sends current transaction totals
func (h *CommandHandler) SendTotals(bot *tgbotapi.BotAPI, chatID int64) {
	ctx := context.Background()
	balance, categoryTotals, userTotals, err := h.db.CalculateTotals(ctx)
	if err != nil {
		log.Println("Failed to calculate totals:", err)
		msg := tgbotapi.NewMessage(chatID, "Error calculating totals.")
		bot.Send(msg)
		return
	}

	// Get additional analytics
	transactions, _ := h.db.GetAllTransactions(ctx)
	
	var totalsText string
	totalsText += "📊 **EXPENSE SUMMARY**\n"
	totalsText += "═══════════════════\n\n"

	// Balance section
	var users []string
	for user := range userTotals {
		users = append(users, user)
	}

	if len(users) >= 2 {
		totalsText += "💰 **Balance:**\n"
		if balance > 0 {
			totalsText += fmt.Sprintf("   %s owes **%.2f$** to %s\n\n", users[1], math.Abs(balance), users[0])
		} else if balance < 0 {
			totalsText += fmt.Sprintf("   %s owes **%.2f$** to %s\n\n", users[0], math.Abs(balance), users[1])
		} else {
			totalsText += "   ✅ All settled! (0$)\n\n"
		}

		// User contributions
		totalsText += "👥 **User Contributions:**\n"
		for user, amount := range userTotals {
			totalsText += fmt.Sprintf("   %s: %.2f$\n", user, amount)
		}
		totalsText += "\n"
	} else {
		totalsText += "❌ No transactions found\n\n"
	}

	// Category breakdown with percentages and analysis
	if len(categoryTotals) > 0 {
		totalSpent := 0.0
		for _, amt := range categoryTotals {
			totalSpent += amt
		}

		totalsText += "📈 **Category Breakdown:**\n"
		
		// Sort categories by amount (highest first)
		type CategoryData struct {
			Name   string
			Amount float64
			Percent float64
		}
		
		var categories []CategoryData
		for cat, amt := range categoryTotals {
			percent := (amt / totalSpent) * 100
			categories = append(categories, CategoryData{cat, amt, percent})
		}
		
		// Simple sort by amount (descending)
		for i := 0; i < len(categories)-1; i++ {
			for j := i + 1; j < len(categories); j++ {
				if categories[i].Amount < categories[j].Amount {
					categories[i], categories[j] = categories[j], categories[i]
				}
			}
		}

		for _, cat := range categories {
			// Visual bar representation
			bars := int(cat.Percent / 10) // 1 bar per 10%
			if bars == 0 && cat.Percent > 0 {
				bars = 1
			}
			barGraph := ""
			for i := 0; i < bars; i++ {
				barGraph += "█"
			}
			if bars < 10 {
				for i := bars; i < 10; i++ {
					barGraph += "░"
				}
			}
			
			totalsText += fmt.Sprintf("   %s **%.2f$** (%.1f%%)\n   %s\n", 
				cat.Name, cat.Amount, cat.Percent, barGraph)
		}
		
		totalsText += fmt.Sprintf("\n💵 **TOTAL SPENT: %.2f$**\n\n", totalSpent)

		// Analytics
		if len(transactions) > 0 {
			avgTransaction := totalSpent / float64(len(transactions))
			totalsText += "📊 **Analytics:**\n"
			totalsText += fmt.Sprintf("   • Total transactions: %d\n", len(transactions))
			totalsText += fmt.Sprintf("   • Average per transaction: %.2f$\n", avgTransaction)
			
			// Find highest and lowest transaction
			highestAmount := 0.0
			lowestAmount := math.MaxFloat64
			for _, tx := range transactions {
				amt := math.Abs(tx.Amount)
				if amt > highestAmount {
					highestAmount = amt
				}
				if amt < lowestAmount {
					lowestAmount = amt
				}
			}
			
			totalsText += fmt.Sprintf("   • Highest transaction: %.2f$\n", highestAmount)
			totalsText += fmt.Sprintf("   • Lowest transaction: %.2f$\n", lowestAmount)
			
			// Most used category
			if len(categories) > 0 {
				totalsText += fmt.Sprintf("   • Top category: %s (%.1f%%)\n", categories[0].Name, categories[0].Percent)
			}
		}
	}

	totalsText += "\n🔄 Use /history to see all transactions"

	msg := tgbotapi.NewMessage(chatID, totalsText)
	msg.ParseMode = "Markdown"
	bot.Send(msg)
}

// ResetDatabase resets all transactions
func (h *CommandHandler) ResetDatabase(bot *tgbotapi.BotAPI, chatID int64) {
	ctx := context.Background()
	err := h.db.DeleteAllTransactions(ctx)
	if err != nil {
		log.Println("Failed to reset database:", err)
		msg := tgbotapi.NewMessage(chatID, "Failed to reset DB.")
		bot.Send(msg)
	} else {
		msg := tgbotapi.NewMessage(chatID, "All transactions deleted. Fresh start!")
		bot.Send(msg)
	}
}

// SendHelp sends help information
func (h *CommandHandler) SendHelp(bot *tgbotapi.BotAPI, chatID int64) {
	helpText := `**📊 Expense Tracker Bot**

**🏠 Basic Commands:**
• /totals - Show current month summary
• /history - Show all transactions
• /help - Show this help

**📈 Analytics & Comparison:**
• /compare - Compare recent months
• /trends - Analyze spending trends
• /export - Export CSV data
• /export compare - Export comparison CSV
• /export 2025-01 - Export specific month

**🔧 Management:**
• /reset - Reset all transactions ⚠️

**💰 Adding Transactions:**
• Send a number (e.g., 25.50) to add expense
• Edit your message to update the amount
• Use 🗑️ Delete button to remove transactions

**🗂️ Categories:**
Groceries, Household, Entertainment, LCBO, Dining Out, Other

**💡 How it works:**
1. Send any number as a message
2. Choose a category from the buttons
3. The amount is split 50/50 between users
4. Monthly data is automatically archived
5. CSV exports are sent to chat history

**📊 Monthly Process:**
• Expenses tracked throughout the month
• Automatic monthly reset (1st of month, 9 AM)
• Data archived to database + CSV export
• Historical comparison and trend analysis`

	msg := tgbotapi.NewMessage(chatID, helpText)
	msg.ParseMode = "Markdown"
	bot.Send(msg)
}

// SendTransactionHistory sends recent transaction history
func (h *CommandHandler) SendTransactionHistory(bot *tgbotapi.BotAPI, chatID int64, limit int) {
	ctx := context.Background()
	transactions, err := h.db.GetRecentTransactions(ctx, limit)
	if err != nil {
		log.Println("Failed to fetch transaction history:", err)
		msg := tgbotapi.NewMessage(chatID, "Error fetching transaction history.")
		bot.Send(msg)
		return
	}

	if len(transactions) == 0 {
		msg := tgbotapi.NewMessage(chatID, "No transactions found.")
		bot.Send(msg)
		return
	}

	historyText := "**📜 Recent Transactions:**\n"
	for i, tx := range transactions {
		timeStr := time.Unix(tx.CreatedAt, 0).Format("Jan 2, 15:04")
		category := tx.Category
		if category == "" {
			category = "Uncategorized"
		}
		historyText += fmt.Sprintf("%d. **%.2f$** by %s (%s) - %s\n", 
			i+1, math.Abs(tx.Amount), tx.Author, category, timeStr)
	}

	msg := tgbotapi.NewMessage(chatID, historyText)
	msg.ParseMode = "Markdown"
	bot.Send(msg)
}

// MonthlyReset performs monthly reset and sends stats
func (h *CommandHandler) MonthlyReset(bot *tgbotapi.BotAPI) {
	ctx := context.Background()
	chatID := h.config.ChatID

	// Archive current month's data (with fallback)
	var archive *models.MonthlyArchive
	archiveErr := h.safeArchiveData(ctx, &archive)
	if archiveErr != nil {
		log.Printf("Archive failed but continuing with reset: %v", archiveErr)
	}

	// Get current data for the report (fallback to recalculation if archive failed)
	var balance, totalSpent float64
	var categoryTotals, userTotals map[string]float64
	var transactions []models.Transaction
	var totalTransactions int

	if archive != nil {
		// Use archived data
		balance = archive.Balance
		totalSpent = archive.TotalSpent
		categoryTotals = archive.CategoryTotals
		userTotals = archive.UserTotals
		transactions = archive.Transactions
		totalTransactions = archive.TotalTransactions
	} else {
		// Fallback: calculate fresh data
		var err error
		balance, categoryTotals, userTotals, err = h.db.CalculateTotals(ctx)
		if err != nil {
			log.Println("Failed to calculate totals for monthly reset:", err)
			return
		}
		transactions, _ = h.db.GetAllTransactions(ctx)
		totalTransactions = len(transactions)
		for _, amt := range categoryTotals {
			totalSpent += amt
		}
	}
	
	var monthlyText string
	monthlyText += "📅 **MONTHLY EXPENSE REPORT**\n"
	monthlyText += "════════════════════════════\n\n"

	if totalTransactions == 0 {
		monthlyText += "❌ No transactions this month\n"
	} else {
		monthlyText += fmt.Sprintf("📊 **Month Summary:**\n")
		monthlyText += fmt.Sprintf("   • Total transactions: %d\n", totalTransactions)
		monthlyText += fmt.Sprintf("   • Total spent: **%.2f$**\n", totalSpent)
		if totalTransactions > 0 {
			monthlyText += fmt.Sprintf("   • Average per transaction: %.2f$\n", totalSpent/float64(totalTransactions))
		}
		monthlyText += fmt.Sprintf("   • Average per day: %.2f$\n\n", totalSpent/30.0) // Approximate

		// Add archive status
		if archive != nil {
			monthlyText += "💾 **Archive Status:** ✅ Data archived successfully\n\n"
		} else {
			monthlyText += "💾 **Archive Status:** ⚠️ Archive failed (data in report only)\n\n"
		}

		// Final balance
		var users []string
		for user := range userTotals {
			users = append(users, user)
		}

		if len(users) >= 2 {
			monthlyText += "💰 **Final Balance:**\n"
			if balance > 0 {
				monthlyText += fmt.Sprintf("   %s owes **%.2f$** to %s\n\n", users[1], math.Abs(balance), users[0])
			} else if balance < 0 {
				monthlyText += fmt.Sprintf("   %s owes **%.2f$** to %s\n\n", users[0], math.Abs(balance), users[1])
			} else {
				monthlyText += "   ✅ Perfect balance! (0$)\n\n"
			}

			// User spending breakdown
			monthlyText += "👥 **User Spending:**\n"
			for user, amount := range userTotals {
				percentage := (amount / totalSpent) * 100
				monthlyText += fmt.Sprintf("   %s: %.2f$ (%.1f%%)\n", user, amount, percentage)
			}
			monthlyText += "\n"
		}

		// Top categories
		if len(categoryTotals) > 0 {
			monthlyText += "🏆 **Top Categories:**\n"
			
			// Sort categories by amount
			type CategoryData struct {
				Name   string
				Amount float64
				Percent float64
			}
			
			var categories []CategoryData
			for cat, amt := range categoryTotals {
				percent := (amt / totalSpent) * 100
				categories = append(categories, CategoryData{cat, amt, percent})
			}
			
			// Sort by amount (descending)
			for i := 0; i < len(categories)-1; i++ {
				for j := i + 1; j < len(categories); j++ {
					if categories[i].Amount < categories[j].Amount {
						categories[i], categories[j] = categories[j], categories[i]
					}
				}
			}

			// Show top 3 categories or all if less than 3
			topCount := len(categories)
			if topCount > 3 {
				topCount = 3
			}
			
			for i := 0; i < topCount; i++ {
				cat := categories[i]
				medal := "🥇"
				if i == 1 {
					medal = "🥈"
				} else if i == 2 {
					medal = "🥉"
				}
				monthlyText += fmt.Sprintf("   %s %s: %.2f$ (%.1f%%)\n", medal, cat.Name, cat.Amount, cat.Percent)
			}
			monthlyText += "\n"
		}

		// Fun insights
		monthlyText += "🎯 **Month Insights:**\n"
		if len(transactions) > 0 {
			// Find highest and lowest transaction
			highestAmount := 0.0
			lowestAmount := math.MaxFloat64
			for _, tx := range transactions {
				amt := math.Abs(tx.Amount)
				if amt > highestAmount {
					highestAmount = amt
				}
				if amt < lowestAmount {
					lowestAmount = amt
				}
			}
			
			monthlyText += fmt.Sprintf("   • Biggest splurge: %.2f$\n", highestAmount)
			monthlyText += fmt.Sprintf("   • Smallest expense: %.2f$\n", lowestAmount)
			
			// Calculate days with spending
			uniqueDays := make(map[string]bool)
			for _, tx := range transactions {
				day := time.Unix(tx.CreatedAt, 0).Format("2006-01-02")
				uniqueDays[day] = true
			}
			monthlyText += fmt.Sprintf("   • Days with spending: %d\n", len(uniqueDays))
		}
	}

	monthlyText += "\n🔄 **Starting fresh for next month!**\n"
	if archive != nil {
		monthlyText += "All transactions have been archived.\n"
		monthlyText += "📊 CSV export will be sent shortly..."
	} else {
		monthlyText += "Transactions cleared (archive failed)."
	}

	// Send the text report first
	msg := tgbotapi.NewMessage(chatID, monthlyText)
	msg.ParseMode = "Markdown"
	bot.Send(msg)

	// Export CSV if archive was successful (with fallback)
	if archive != nil {
		h.safeExportCSV(bot, chatID, archive)
	}

	// Clear DB (with error handling)
	err := h.db.DeleteAllTransactions(ctx)
	if err != nil {
		log.Println("Failed to delete monthly data:", err)
		// Send error message to user
		errorMsg := tgbotapi.NewMessage(chatID, "⚠️ Warning: Failed to clear transactions. Manual cleanup may be needed.")
		bot.Send(errorMsg)
	} else {
		log.Println("Monthly reset complete.")
	}
}

// safeArchiveData safely archives monthly data with error handling
func (h *CommandHandler) safeArchiveData(ctx context.Context, archive **models.MonthlyArchive) error {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Archive panic recovered: %v", r)
		}
	}()

	archiveData, err := h.db.ArchiveMonthlyData(ctx)
	if err != nil {
		return fmt.Errorf("failed to archive: %w", err)
	}

	*archive = archiveData
	return nil
}

// safeExportCSV safely exports CSV with error handling and fallbacks
func (h *CommandHandler) safeExportCSV(bot *tgbotapi.BotAPI, chatID int64, archive *models.MonthlyArchive) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("CSV export panic recovered: %v", r)
			// Send fallback message
			msg := tgbotapi.NewMessage(chatID, "⚠️ CSV export failed. Data is still archived in database.")
			bot.Send(msg)
		}
	}()

	// Generate CSV
	var buffer bytes.Buffer
	err := utils.GenerateMonthlyCSV(archive, &buffer)
	if err != nil {
		log.Printf("Failed to generate CSV: %v", err)
		msg := tgbotapi.NewMessage(chatID, "⚠️ CSV generation failed. Data is still archived in database.")
		bot.Send(msg)
		return
	}

	// Create filename
	filename := fmt.Sprintf("expenses_%s_%d.csv", archive.MonthName, archive.Year)
	
	// Send CSV file
	document := tgbotapi.FileBytes{
		Name:  filename,
		Bytes: buffer.Bytes(),
	}

	documentMsg := tgbotapi.NewDocument(chatID, document)
	documentMsg.Caption = fmt.Sprintf("📊 Monthly expense data for %s %d\n💾 %d transactions, %.2f$ total", 
		archive.MonthName, archive.Year, archive.TotalTransactions, archive.TotalSpent)

	_, err = bot.Send(documentMsg)
	if err != nil {
		log.Printf("Failed to send CSV file: %v", err)
		msg := tgbotapi.NewMessage(chatID, "⚠️ Failed to send CSV file. Data is archived in database - use /export command later.")
		bot.Send(msg)
	}
}

// SendMonthlyComparison compares recent months
func (h *CommandHandler) SendMonthlyComparison(bot *tgbotapi.BotAPI, chatID int64) {
	ctx := context.Background()
	
	archives, err := h.db.GetRecentArchives(ctx, 3)
	if err != nil || len(archives) == 0 {
		msg := tgbotapi.NewMessage(chatID, "❌ No archived months found for comparison.\nUse the bot for a month and wait for monthly reset to generate archives.")
		bot.Send(msg)
		return
	}

	if len(archives) == 1 {
		msg := tgbotapi.NewMessage(chatID, "📊 Only one month archived. Need at least 2 months for comparison.\nCheck back after next monthly reset!")
		bot.Send(msg)
		return
	}

	// Generate comparison text
	var comparisonText string
	comparisonText += "📊 **MONTHLY COMPARISON**\n"
	comparisonText += "═══════════════════════\n\n"

	// Summary table
	comparisonText += "📈 **Spending Overview:**\n"
	for i, archive := range archives {
		emoji := "📅"
		if i == 0 {
			emoji = "🆕" // Most recent
		}
		comparisonText += fmt.Sprintf("%s %s %d: **%.2f$** (%d transactions)\n", 
			emoji, archive.MonthName, archive.Year, archive.TotalSpent, archive.TotalTransactions)
	}
	comparisonText += "\n"

	// Month-over-month changes
	if len(archives) >= 2 {
		current := archives[0]
		previous := archives[1]
		
		spendingChange := current.TotalSpent - previous.TotalSpent
		spendingPercent := (spendingChange / previous.TotalSpent) * 100
		
		transactionChange := current.TotalTransactions - previous.TotalTransactions
		
		comparisonText += "📈 **Month-over-Month:**\n"
		
		spendingEmoji := "📈"
		if spendingChange < 0 {
			spendingEmoji = "📉"
		}
		comparisonText += fmt.Sprintf("%s Spending: %.2f$ (%+.1f%%)\n", spendingEmoji, spendingChange, spendingPercent)
		
		transactionEmoji := "📈"
		if transactionChange < 0 {
			transactionEmoji = "📉"
		}
		comparisonText += fmt.Sprintf("%s Transactions: %+d\n\n", transactionEmoji, transactionChange)
	}

	// Category comparison (top categories)
	if len(archives) >= 2 {
		comparisonText += "🏆 **Top Categories Comparison:**\n"
		current := archives[0]
		previous := archives[1]
		
		// Get top 3 categories from current month
		type CategoryData struct {
			Name   string
			Amount float64
		}
		
		var currentCategories []CategoryData
		for cat, amt := range current.CategoryTotals {
			currentCategories = append(currentCategories, CategoryData{cat, amt})
		}
		
		// Sort by amount
		for i := 0; i < len(currentCategories)-1; i++ {
			for j := i + 1; j < len(currentCategories); j++ {
				if currentCategories[i].Amount < currentCategories[j].Amount {
					currentCategories[i], currentCategories[j] = currentCategories[j], currentCategories[i]
				}
			}
		}
		
		topCount := len(currentCategories)
		if topCount > 3 {
			topCount = 3
		}
		
		for i := 0; i < topCount; i++ {
			cat := currentCategories[i]
			currentAmount := cat.Amount
			previousAmount := previous.CategoryTotals[cat.Name]
			
			var changeText string
			if previousAmount > 0 {
				change := currentAmount - previousAmount
				changePercent := (change / previousAmount) * 100
				if change > 0 {
					changeText = fmt.Sprintf(" (+%.1f%%)", changePercent)
				} else if change < 0 {
					changeText = fmt.Sprintf(" (%.1f%%)", changePercent)
				} else {
					changeText = " (no change)"
				}
			} else {
				changeText = " (new category)"
			}
			
			comparisonText += fmt.Sprintf("   %s: %.2f$%s\n", cat.Name, currentAmount, changeText)
		}
		comparisonText += "\n"
	}

	// Insights
	comparisonText += "💡 **Insights:**\n"
	if len(archives) >= 2 {
		current := archives[0]
		previous := archives[1]
		
		avgCurrent := current.AvgTransaction
		avgPrevious := previous.AvgTransaction
		
		if avgCurrent > avgPrevious {
			comparisonText += "   • Average transaction amount increased\n"
		} else if avgCurrent < avgPrevious {
			comparisonText += "   • Average transaction amount decreased\n"
		}
		
		if current.DaysWithSpending > previous.DaysWithSpending {
			comparisonText += "   • More active spending days\n"
		} else if current.DaysWithSpending < previous.DaysWithSpending {
			comparisonText += "   • Fewer active spending days\n"
		}
	}

	comparisonText += "\n📄 Use /export to get detailed CSV comparison"

	msg := tgbotapi.NewMessage(chatID, comparisonText)
	msg.ParseMode = "Markdown"
	bot.Send(msg)
}

// SendSpendingTrends analyzes spending trends
func (h *CommandHandler) SendSpendingTrends(bot *tgbotapi.BotAPI, chatID int64) {
	ctx := context.Background()
	
	archives, err := h.db.GetRecentArchives(ctx, 6) // Last 6 months
	if err != nil || len(archives) == 0 {
		msg := tgbotapi.NewMessage(chatID, "❌ No archived data found for trend analysis.")
		bot.Send(msg)
		return
	}

	var trendsText string
	trendsText += "📈 **SPENDING TRENDS ANALYSIS**\n"
	trendsText += "═══════════════════════════════\n\n"

	// Spending trend over time
	trendsText += "💰 **Monthly Spending Trend:**\n"
	totalSpent := 0.0
	for i := len(archives) - 1; i >= 0; i-- { // Show chronologically
		archive := archives[i]
		trendEmoji := "📊"
		if i > 0 && archive.TotalSpent > archives[i-1].TotalSpent {
			trendEmoji = "📈"
		} else if i > 0 && archive.TotalSpent < archives[i-1].TotalSpent {
			trendEmoji = "📉"
		}
		
		trendsText += fmt.Sprintf("%s %s %d: %.2f$\n", trendEmoji, archive.MonthName, archive.Year, archive.TotalSpent)
		totalSpent += archive.TotalSpent
	}
	
	avgMonthlySpending := totalSpent / float64(len(archives))
	trendsText += fmt.Sprintf("\n📊 **Average Monthly Spending:** %.2f$\n\n", avgMonthlySpending)

	// Category trends
	categoryTotals := make(map[string]float64)
	categoryMonths := make(map[string]int)
	
	for _, archive := range archives {
		for cat, amount := range archive.CategoryTotals {
			categoryTotals[cat] += amount
			categoryMonths[cat]++
		}
	}
	
	if len(categoryTotals) > 0 {
		trendsText += "🏷️ **Category Trends (Avg/Month):**\n"
		
		// Sort categories by total spending
		type CategoryAvg struct {
			Name string
			Avg  float64
		}
		
		var categoryAvgs []CategoryAvg
		for cat, total := range categoryTotals {
			avg := total / float64(categoryMonths[cat])
			categoryAvgs = append(categoryAvgs, CategoryAvg{cat, avg})
		}
		
		// Sort by average
		for i := 0; i < len(categoryAvgs)-1; i++ {
			for j := i + 1; j < len(categoryAvgs); j++ {
				if categoryAvgs[i].Avg < categoryAvgs[j].Avg {
					categoryAvgs[i], categoryAvgs[j] = categoryAvgs[j], categoryAvgs[i]
				}
			}
		}
		
		for _, catAvg := range categoryAvgs {
			percentage := (catAvg.Avg / avgMonthlySpending) * 100
			trendsText += fmt.Sprintf("   %s: %.2f$/month (%.1f%%)\n", catAvg.Name, catAvg.Avg, percentage)
		}
		trendsText += "\n"
	}

	// Transaction patterns
	totalTransactions := 0
	for _, archive := range archives {
		totalTransactions += archive.TotalTransactions
	}
	avgTransactionsPerMonth := float64(totalTransactions) / float64(len(archives))
	
	trendsText += "📱 **Transaction Patterns:**\n"
	trendsText += fmt.Sprintf("   • Avg transactions/month: %.1f\n", avgTransactionsPerMonth)
	trendsText += fmt.Sprintf("   • Total months analyzed: %d\n", len(archives))
	trendsText += fmt.Sprintf("   • Total transactions: %d\n\n", totalTransactions)

	// Seasonal insights (if we have enough data)
	if len(archives) >= 3 {
		trendsText += "🔍 **Insights:**\n"
		
		// Find highest and lowest spending months
		highest := archives[0]
		lowest := archives[0]
		
		for _, archive := range archives {
			if archive.TotalSpent > highest.TotalSpent {
				highest = archive
			}
			if archive.TotalSpent < lowest.TotalSpent {
				lowest = archive
			}
		}
		
		trendsText += fmt.Sprintf("   • Highest spending: %s %d (%.2f$)\n", highest.MonthName, highest.Year, highest.TotalSpent)
		trendsText += fmt.Sprintf("   • Lowest spending: %s %d (%.2f$)\n", lowest.MonthName, lowest.Year, lowest.TotalSpent)
		
		// Volatility
		variance := 0.0
		for _, archive := range archives {
			diff := archive.TotalSpent - avgMonthlySpending
			variance += diff * diff
		}
		stdDev := math.Sqrt(variance / float64(len(archives)))
		volatility := (stdDev / avgMonthlySpending) * 100
		
		trendsText += fmt.Sprintf("   • Spending volatility: %.1f%%\n", volatility)
		
		if volatility < 20 {
			trendsText += "   • 🟢 Consistent spending pattern\n"
		} else if volatility < 40 {
			trendsText += "   • 🟡 Moderate spending variation\n"
		} else {
			trendsText += "   • 🔴 High spending volatility\n"
		}
	}

	msg := tgbotapi.NewMessage(chatID, trendsText)
	msg.ParseMode = "Markdown"
	bot.Send(msg)
}

// ExportMonthlyData exports specific month data or comparison
func (h *CommandHandler) ExportMonthlyData(bot *tgbotapi.BotAPI, chatID int64, commandText string) {
	ctx := context.Background()
	
	// Parse command arguments
	args := strings.Fields(commandText)
	
	if len(args) == 1 {
		// No arguments - export most recent month
		archives, err := h.db.GetRecentArchives(ctx, 1)
		if err != nil || len(archives) == 0 {
			msg := tgbotapi.NewMessage(chatID, "❌ No archived data found.\nUsage: /export [month-year] or /export compare")
			bot.Send(msg)
			return
		}
		
		h.safeExportCSV(bot, chatID, &archives[0])
		return
	}
	
	if len(args) == 2 && args[1] == "compare" {
		// Export comparison CSV
		archives, err := h.db.GetRecentArchives(ctx, 6)
		if err != nil || len(archives) < 2 {
			msg := tgbotapi.NewMessage(chatID, "❌ Need at least 2 archived months for comparison.")
			bot.Send(msg)
			return
		}
		
		// Generate comparison CSV
		var buffer bytes.Buffer
		err = utils.GenerateComparisonCSV(archives, &buffer)
		if err != nil {
			log.Printf("Failed to generate comparison CSV: %v", err)
			msg := tgbotapi.NewMessage(chatID, "⚠️ Failed to generate comparison CSV.")
			bot.Send(msg)
			return
		}
		
		// Send file
		filename := fmt.Sprintf("comparison_%s.csv", time.Now().Format("2006-01"))
		document := tgbotapi.FileBytes{
			Name:  filename,
			Bytes: buffer.Bytes(),
		}
		
		documentMsg := tgbotapi.NewDocument(chatID, document)
		documentMsg.Caption = fmt.Sprintf("📊 Monthly comparison report\n📈 %d months analyzed", len(archives))
		
		bot.Send(documentMsg)
		return
	}
	
	// Try to parse specific month (format: YYYY-MM or Month-YYYY)
	monthID := ""
	if len(args) >= 2 {
		// Try different formats
		arg := args[1]
		if len(arg) == 7 && arg[4] == '-' {
			// Format: 2025-01
			monthID = arg
		} else {
			// Try to parse other formats (Month-Year, etc.)
			msg := tgbotapi.NewMessage(chatID, "❌ Invalid format. Use: /export 2025-01 or /export compare")
			bot.Send(msg)
			return
		}
	}
	
	// Get specific month archive
	archive, err := h.db.GetMonthlyArchive(ctx, monthID)
	if err != nil {
		msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ No archive found for %s", monthID))
		bot.Send(msg)
		return
	}
	
	h.safeExportCSV(bot, chatID, archive)
}