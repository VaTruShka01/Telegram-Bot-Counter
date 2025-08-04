package utils

import (
	"encoding/csv"
	"fmt"
	"io"
	"math"
	"strconv"
	"time"

	"telegram-expense-bot/internal/models"
)

// GenerateMonthlyCSV creates a CSV file content for monthly data
func GenerateMonthlyCSV(archive *models.MonthlyArchive, writer io.Writer) error {
	csvWriter := csv.NewWriter(writer)
	defer csvWriter.Flush()

	// Header section
	header := [][]string{
		{"Monthly Expense Report"},
		{"Month", archive.MonthName + " " + strconv.Itoa(archive.Year)},
		{"Generated", time.Now().Format("2006-01-02 15:04:05")},
		{}, // Empty row
		{"SUMMARY"},
		{"Total Spent", fmt.Sprintf("%.2f", archive.TotalSpent)},
		{"Total Transactions", strconv.Itoa(archive.TotalTransactions)},
		{"Average Transaction", fmt.Sprintf("%.2f", archive.AvgTransaction)},
		{"Highest Transaction", fmt.Sprintf("%.2f", archive.HighestTransaction)},
		{"Lowest Transaction", fmt.Sprintf("%.2f", archive.LowestTransaction)},
		{"Days with Spending", strconv.Itoa(archive.DaysWithSpending)},
		{"Balance", fmt.Sprintf("%.2f", archive.Balance)},
		{}, // Empty row
	}

	// Write header
	for _, row := range header {
		if err := csvWriter.Write(row); err != nil {
			return fmt.Errorf("failed to write header: %w", err)
		}
	}

	// User totals section
	if len(archive.UserTotals) > 0 {
		if err := csvWriter.Write([]string{"USER SPENDING"}); err != nil {
			return err
		}
		if err := csvWriter.Write([]string{"User", "Amount", "Percentage"}); err != nil {
			return err
		}
		
		for user, amount := range archive.UserTotals {
			percentage := (amount / archive.TotalSpent) * 100
			row := []string{
				user,
				fmt.Sprintf("%.2f", amount),
				fmt.Sprintf("%.1f%%", percentage),
			}
			if err := csvWriter.Write(row); err != nil {
				return err
			}
		}
		if err := csvWriter.Write([]string{}); err != nil {
			return err
		}
	}

	// Category totals section
	if len(archive.CategoryTotals) > 0 {
		if err := csvWriter.Write([]string{"CATEGORY BREAKDOWN"}); err != nil {
			return err
		}
		if err := csvWriter.Write([]string{"Category", "Amount", "Percentage"}); err != nil {
			return err
		}
		
		for category, amount := range archive.CategoryTotals {
			percentage := (amount / archive.TotalSpent) * 100
			row := []string{
				category,
				fmt.Sprintf("%.2f", amount),
				fmt.Sprintf("%.1f%%", percentage),
			}
			if err := csvWriter.Write(row); err != nil {
				return err
			}
		}
		if err := csvWriter.Write([]string{}); err != nil {
			return err
		}
	}

	// Transactions section
	if len(archive.Transactions) > 0 {
		if err := csvWriter.Write([]string{"DETAILED TRANSACTIONS"}); err != nil {
			return err
		}
		if err := csvWriter.Write([]string{"Date", "Time", "Amount", "Author", "Category"}); err != nil {
			return err
		}
		
		for _, tx := range archive.Transactions {
			date := time.Unix(tx.CreatedAt, 0)
			category := tx.Category
			if category == "" {
				category = "Uncategorized"
			}
			
			row := []string{
				date.Format("2006-01-02"),
				date.Format("15:04:05"),
				fmt.Sprintf("%.2f", math.Abs(tx.Amount)),
				tx.Author,
				category,
			}
			if err := csvWriter.Write(row); err != nil {
				return err
			}
		}
	}

	return nil
}

// GenerateComparisonCSV creates a comparison CSV for multiple months
func GenerateComparisonCSV(archives []models.MonthlyArchive, writer io.Writer) error {
	if len(archives) == 0 {
		return fmt.Errorf("no archives provided for comparison")
	}

	csvWriter := csv.NewWriter(writer)
	defer csvWriter.Flush()

	// Header
	header := [][]string{
		{"Monthly Comparison Report"},
		{"Generated", time.Now().Format("2006-01-02 15:04:05")},
		{}, // Empty row
	}

	for _, row := range header {
		if err := csvWriter.Write(row); err != nil {
			return err
		}
	}

	// Summary comparison
	summaryHeader := []string{"Metric"}
	for _, archive := range archives {
		summaryHeader = append(summaryHeader, archive.MonthName+" "+strconv.Itoa(archive.Year))
	}
	if err := csvWriter.Write(summaryHeader); err != nil {
		return err
	}

	// Write metrics rows
	metrics := []string{
		"Total Spent",
		"Total Transactions", 
		"Average Transaction",
		"Highest Transaction",
		"Lowest Transaction",
		"Days with Spending",
		"Balance",
	}

	for _, metric := range metrics {
		row := []string{metric}
		for _, archive := range archives {
			var value string
			switch metric {
			case "Total Spent":
				value = fmt.Sprintf("%.2f", archive.TotalSpent)
			case "Total Transactions":
				value = strconv.Itoa(archive.TotalTransactions)
			case "Average Transaction":
				value = fmt.Sprintf("%.2f", archive.AvgTransaction)
			case "Highest Transaction":
				value = fmt.Sprintf("%.2f", archive.HighestTransaction)
			case "Lowest Transaction":
				value = fmt.Sprintf("%.2f", archive.LowestTransaction)
			case "Days with Spending":
				value = strconv.Itoa(archive.DaysWithSpending)
			case "Balance":
				value = fmt.Sprintf("%.2f", archive.Balance)
			}
			row = append(row, value)
		}
		if err := csvWriter.Write(row); err != nil {
			return err
		}
	}

	// Add growth rates if we have multiple months
	if len(archives) > 1 {
		if err := csvWriter.Write([]string{}); err != nil {
			return err
		}
		if err := csvWriter.Write([]string{"GROWTH RATES (Month-over-Month)"}); err != nil {
			return err
		}
		
		growthHeader := []string{"Metric"}
		for i := 1; i < len(archives); i++ {
			growthHeader = append(growthHeader, fmt.Sprintf("%s vs %s", 
				archives[i].MonthName, archives[i-1].MonthName))
		}
		if err := csvWriter.Write(growthHeader); err != nil {
			return err
		}

		for _, metric := range []string{"Total Spent", "Total Transactions", "Average Transaction"} {
			row := []string{metric}
			for i := 1; i < len(archives); i++ {
				var current, previous float64
				switch metric {
				case "Total Spent":
					current = archives[i].TotalSpent
					previous = archives[i-1].TotalSpent
				case "Total Transactions":
					current = float64(archives[i].TotalTransactions)
					previous = float64(archives[i-1].TotalTransactions)
				case "Average Transaction":
					current = archives[i].AvgTransaction
					previous = archives[i-1].AvgTransaction
				}
				
				var growth string
				if previous != 0 {
					growthRate := ((current - previous) / previous) * 100
					growth = fmt.Sprintf("%.1f%%", growthRate)
				} else {
					growth = "N/A"
				}
				row = append(row, growth)
			}
			if err := csvWriter.Write(row); err != nil {
				return err
			}
		}
	}

	return nil
}