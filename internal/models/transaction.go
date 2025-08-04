package models

// Transaction represents one record in MongoDB.
type Transaction struct {
	ID                  string   `bson:"_id" json:"id"`
	Amount              float64  `bson:"amount" json:"amount"`
	Author              string   `bson:"author" json:"author"`
	Category            string   `bson:"category,omitempty" json:"category,omitempty"`
	ButtonMessageID     string   `bson:"buttonMessageId,omitempty" json:"buttonMessageId,omitempty"`
	ConfirmationMessageID string `bson:"confirmationMessageId,omitempty" json:"confirmationMessageId,omitempty"`
	CreatedAt           int64    `bson:"createdAt" json:"createdAt"`
}