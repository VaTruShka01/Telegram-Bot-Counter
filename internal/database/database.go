package database

import (
	"context"
	"fmt"
	"log"
	"math"
	"time"

	"telegram-expense-bot/internal/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// DB wraps MongoDB operations
type DB struct {
	client           *mongo.Client
	collection       *mongo.Collection
	archiveCollection *mongo.Collection
}

// New creates a new database connection
func New(ctx context.Context, uri, dbName, collName string) (*DB, error) {
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MongoDB: %w", err)
	}

	if err = client.Ping(ctx, nil); err != nil {
		return nil, fmt.Errorf("failed to ping MongoDB: %w", err)
	}

	database := client.Database(dbName)
	collection := database.Collection(collName)
	archiveCollection := database.Collection("monthly_archives")
	
	log.Println("Successfully connected to MongoDB")
	return &DB{
		client:           client,
		collection:       collection,
		archiveCollection: archiveCollection,
	}, nil
}

// Close closes the database connection
func (db *DB) Close(ctx context.Context) error {
	return db.client.Disconnect(ctx)
}

// InsertTransaction inserts a new transaction
func (db *DB) InsertTransaction(ctx context.Context, tx *models.Transaction) error {
	tx.CreatedAt = time.Now().Unix()
	_, err := db.collection.InsertOne(ctx, tx)
	if err != nil {
		return fmt.Errorf("failed to insert transaction: %w", err)
	}
	return nil
}

// FindTransaction finds a transaction by ID
func (db *DB) FindTransaction(ctx context.Context, id string) (*models.Transaction, error) {
	var tx models.Transaction
	err := db.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&tx)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to find transaction: %w", err)
	}
	return &tx, nil
}

// UpdateTransaction updates a transaction
func (db *DB) UpdateTransaction(ctx context.Context, id string, update bson.M) error {
	filter := bson.M{"_id": id}
	_, err := db.collection.UpdateOne(ctx, filter, bson.M{"$set": update})
	if err != nil {
		return fmt.Errorf("failed to update transaction: %w", err)
	}
	return nil
}

// DeleteTransaction deletes a transaction by ID
func (db *DB) DeleteTransaction(ctx context.Context, id string) error {
	_, err := db.collection.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return fmt.Errorf("failed to delete transaction: %w", err)
	}
	return nil
}

// GetAllTransactions returns all transactions
func (db *DB) GetAllTransactions(ctx context.Context) ([]models.Transaction, error) {
	cursor, err := db.collection.Find(ctx, bson.M{})
	if err != nil {
		return nil, fmt.Errorf("failed to fetch transactions: %w", err)
	}
	defer cursor.Close(ctx)

	var transactions []models.Transaction
	for cursor.Next(ctx) {
		var tx models.Transaction
		if err := cursor.Decode(&tx); err == nil {
			transactions = append(transactions, tx)
		}
	}
	return transactions, nil
}

// GetRecentTransactions returns recent transactions with limit (0 = no limit)
func (db *DB) GetRecentTransactions(ctx context.Context, limit int) ([]models.Transaction, error) {
	opts := options.Find().SetSort(bson.D{{"createdAt", -1}})
	if limit > 0 {
		opts = opts.SetLimit(int64(limit))
	}
	cursor, err := db.collection.Find(ctx, bson.M{}, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch recent transactions: %w", err)
	}
	defer cursor.Close(ctx)

	var transactions []models.Transaction
	for cursor.Next(ctx) {
		var tx models.Transaction
		if err := cursor.Decode(&tx); err == nil {
			transactions = append(transactions, tx)
		}
	}
	return transactions, nil
}

// DeleteAllTransactions deletes all transactions
func (db *DB) DeleteAllTransactions(ctx context.Context) error {
	_, err := db.collection.DeleteMany(ctx, bson.M{})
	if err != nil {
		return fmt.Errorf("failed to delete all transactions: %w", err)
	}
	return nil
}

// CalculateTotals calculates user balances and category totals
func (db *DB) CalculateTotals(ctx context.Context) (float64, map[string]float64, map[string]float64, error) {
	transactions, err := db.GetAllTransactions(ctx)
	if err != nil {
		return 0, nil, nil, err
	}

	userTotals := make(map[string]float64)
	categoryTotals := make(map[string]float64)

	for _, tx := range transactions {
		// Each user's contribution is half the transaction amount
		absHalf := math.Abs(tx.Amount / 2)
		userTotals[tx.Author] += absHalf
		
		if tx.Category != "" {
			categoryTotals[tx.Category] += math.Abs(tx.Amount)
		}
	}

	// Calculate net balance (difference between users)
	var users []string
	for user := range userTotals {
		users = append(users, user)
	}
	
	var balance float64 = 0
	if len(users) >= 2 {
		// First user owes positive, second user owes negative
		balance = userTotals[users[0]] - userTotals[users[1]]
	}

	return balance, categoryTotals, userTotals, nil
}

// ArchiveMonthlyData archives current month's data and returns the archive
func (db *DB) ArchiveMonthlyData(ctx context.Context) (*models.MonthlyArchive, error) {
	// Get all current transactions
	transactions, err := db.GetAllTransactions(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get transactions for archive: %w", err)
	}

	if len(transactions) == 0 {
		return nil, fmt.Errorf("no transactions to archive")
	}

	// Calculate totals
	balance, categoryTotals, userTotals, err := db.CalculateTotals(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate totals for archive: %w", err)
	}

	// Calculate additional stats
	totalSpent := 0.0
	highestAmount := 0.0
	lowestAmount := math.MaxFloat64
	uniqueDays := make(map[string]bool)

	for _, tx := range transactions {
		amt := math.Abs(tx.Amount)
		totalSpent += amt
		
		if amt > highestAmount {
			highestAmount = amt
		}
		if amt < lowestAmount {
			lowestAmount = amt
		}
		
		day := time.Unix(tx.CreatedAt, 0).Format("2006-01-02")
		uniqueDays[day] = true
	}

	avgTransaction := totalSpent / float64(len(transactions))

	// Create archive record
	now := time.Now()
	monthID := now.Format("2006-01")
	
	archive := &models.MonthlyArchive{
		ID:                monthID,
		Year:              now.Year(),
		Month:             int(now.Month()),
		MonthName:         now.Format("January"),
		TotalSpent:        totalSpent,
		TotalTransactions: len(transactions),
		Balance:           balance,
		UserTotals:        userTotals,
		CategoryTotals:    categoryTotals,
		Transactions:      transactions,
		AvgTransaction:    avgTransaction,
		HighestTransaction: highestAmount,
		LowestTransaction:  lowestAmount,
		DaysWithSpending:  len(uniqueDays),
		ArchivedAt:        now.Unix(),
	}

	// Insert archive (upsert to handle re-runs)
	opts := options.ReplaceOptions{}
	opts.SetUpsert(true)
	_, err = db.archiveCollection.ReplaceOne(ctx, bson.M{"_id": monthID}, archive, &opts)
	if err != nil {
		return nil, fmt.Errorf("failed to save monthly archive: %w", err)
	}

	return archive, nil
}

// GetMonthlyArchive retrieves archived data for a specific month
func (db *DB) GetMonthlyArchive(ctx context.Context, monthID string) (*models.MonthlyArchive, error) {
	var archive models.MonthlyArchive
	err := db.archiveCollection.FindOne(ctx, bson.M{"_id": monthID}).Decode(&archive)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, fmt.Errorf("no archive found for month %s", monthID)
		}
		return nil, fmt.Errorf("failed to retrieve archive: %w", err)
	}
	return &archive, nil
}

// GetRecentArchives retrieves the most recent archived months
func (db *DB) GetRecentArchives(ctx context.Context, limit int) ([]models.MonthlyArchive, error) {
	opts := options.Find().SetSort(bson.D{{"archivedAt", -1}})
	if limit > 0 {
		opts = opts.SetLimit(int64(limit))
	}

	cursor, err := db.archiveCollection.Find(ctx, bson.M{}, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch recent archives: %w", err)
	}
	defer cursor.Close(ctx)

	var archives []models.MonthlyArchive
	for cursor.Next(ctx) {
		var archive models.MonthlyArchive
		if err := cursor.Decode(&archive); err == nil {
			archives = append(archives, archive)
		}
	}

	return archives, nil
}

// GetAllArchives retrieves all archived months
func (db *DB) GetAllArchives(ctx context.Context) ([]models.MonthlyArchive, error) {
	return db.GetRecentArchives(ctx, 0)
}