package models

// MonthlyArchive represents archived monthly data
type MonthlyArchive struct {
	ID            string        `bson:"_id" json:"id"`                    // Format: "2025-01"
	Year          int           `bson:"year" json:"year"`
	Month         int           `bson:"month" json:"month"`
	MonthName     string        `bson:"monthName" json:"monthName"`
	TotalSpent    float64       `bson:"totalSpent" json:"totalSpent"`
	TotalTransactions int       `bson:"totalTransactions" json:"totalTransactions"`
	Balance       float64       `bson:"balance" json:"balance"`
	UserTotals    map[string]float64 `bson:"userTotals" json:"userTotals"`
	CategoryTotals map[string]float64 `bson:"categoryTotals" json:"categoryTotals"`
	Transactions  []Transaction `bson:"transactions" json:"transactions"`
	AvgTransaction float64      `bson:"avgTransaction" json:"avgTransaction"`
	HighestTransaction float64  `bson:"highestTransaction" json:"highestTransaction"`
	LowestTransaction float64   `bson:"lowestTransaction" json:"lowestTransaction"`
	DaysWithSpending int        `bson:"daysWithSpending" json:"daysWithSpending"`
	ArchivedAt    int64         `bson:"archivedAt" json:"archivedAt"`
}