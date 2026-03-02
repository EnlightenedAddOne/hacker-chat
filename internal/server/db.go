package server

import (
	"database/sql"
	"log"

	"hacker-chat/internal/protocol"

	_ "modernc.org/sqlite"
)

var DB *sql.DB

// InitDB 初始化本地数据库并建表。
func InitDB() {
	var err error
	DB, err = sql.Open("sqlite", "./matrix.db")
	if err != nil {
		log.Fatalf("[FATAL] Failed to mount Matrix memory core: %v", err)
	}

	createTableSQL := `
	CREATE TABLE IF NOT EXISTS messages (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		msg_type TEXT NOT NULL,
		sender TEXT NOT NULL,
		target TEXT,
		content TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`

	if _, err := DB.Exec(createTableSQL); err != nil {
		log.Fatalf("[FATAL] Failed to initialize database schema: %v", err)
	}

	log.Println("[SYSTEM] Matrix memory core (SQLite) mounted successfully.")
}

// SaveMessage 将广播消息落盘归档。
func SaveMessage(msg protocol.Message) {
	if DB == nil {
		return
	}

	if msg.Type == protocol.TypeCommand {
		return
	}

	insertSQL := `INSERT INTO messages (msg_type, sender, target, content, created_at) VALUES (?, ?, ?, ?, ?)`
	if _, err := DB.Exec(insertSQL, msg.Type, msg.Sender, msg.Target, msg.Content, msg.Timestamp); err != nil {
		log.Printf("[WARN] Failed to archive message: %v", err)
	}
}

// GetHistory 提取最近 50 条公开记录（排除私聊）。
func GetHistory() []protocol.Message {
	if DB == nil {
		return nil
	}

	querySQL := `SELECT msg_type, sender, content, created_at FROM messages
		WHERE msg_type != 'PRIVATE' ORDER BY id DESC LIMIT 50`

	rows, err := DB.Query(querySQL)
	if err != nil {
		log.Printf("[WARN] Failed to retrieve history: %v", err)
		return nil
	}
	defer rows.Close()

	history := make([]protocol.Message, 0, 50)
	for rows.Next() {
		var msg protocol.Message
		if err := rows.Scan(&msg.Type, &msg.Sender, &msg.Content, &msg.Timestamp); err == nil {
			history = append(history, msg)
		}
	}

	for i, j := 0, len(history)-1; i < j; i, j = i+1, j-1 {
		history[i], history[j] = history[j], history[i]
	}

	return history
}
