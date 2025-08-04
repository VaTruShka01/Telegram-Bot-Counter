# Telegram Expense Bot

A Telegram bot for tracking shared expenses between two users, converted from the original Discord bot.

## Features

- Track expenses by sending numbers as messages
- Categorize expenses using inline buttons
- Calculate who owes what with automatic 50/50 splits
- View transaction history and totals
- Monthly automatic reset with statistics
- Edit and delete transaction support
- MongoDB storage for persistence

## Setup

1. **Create a Telegram Bot:**
   - Message [@BotFather](https://t.me/botfather) on Telegram
   - Use `/newbot` command and follow instructions
   - Save the bot token

2. **Get Chat ID:**
   - Add your bot to a group or get your private chat ID
   - Use `/start` command to begin interaction

3. **Configure Environment:**
   ```bash
   cp .env.example .env
   ```
   Edit `.env` with your credentials:
   - `TELEGRAM_BOT_TOKEN`: Your bot token from BotFather
   - `TELEGRAM_CHAT_ID`: The chat ID where the bot should work
   - `MONGODB_URI`: Your MongoDB connection string
   - `MONGODB_DB`: Database name to use

4. **Install Dependencies:**
   ```bash
   go mod tidy
   ```

5. **Run the Bot:**
   ```bash
   go run main.go
   ```

## Usage

### Commands
- `/totals` - Show current balance and category totals
- `/reset` - Reset all transactions (⚠️ careful!)
- `/help` - Show help information
- `/history` - Show last 10 transactions

### Adding Transactions
1. Send any number as a message (e.g., `25.50`)
2. Select a category from the inline buttons
3. The bot confirms the transaction

### Editing/Deleting
- Edit your original message to change the amount
- Delete your original message to remove the transaction

## Categories

The bot includes these expense categories:
- Groceries 🛒
- Household 🏠
- Entertainment 🎉
- LCBO 🥂
- Dining Out 🍽️
- Other 🗂️

## How It Works

1. **Transaction Creation**: Send a number, bot creates a transaction record
2. **Category Selection**: Choose category via inline buttons
3. **Balance Calculation**: Each expense is split 50/50 between authorized users
4. **Monthly Reset**: Automatic reset on the 1st of each month at 9 AM

## Configuration

Edit `internal/config/config.go` to change:
- Authorized users (currently `vatrushka2` and `lerotko`)
- Expense categories
- Other bot settings

## Database Schema

Transactions are stored in MongoDB with this structure:
```json
{
  "_id": "message_id",
  "amount": 25.50,
  "author": "username",
  "category": "Groceries 🛒",
  "buttonMessageId": "123",
  "confirmationMessageId": "124",
  "createdAt": 1234567890
}
```

## Differences from Discord Version

- Uses Telegram Bot API instead of Discord
- Inline keyboards instead of Discord components
- Message IDs are integers instead of strings
- Single button message instead of multiple button messages
- Simplified message handling due to Telegram's different architecture