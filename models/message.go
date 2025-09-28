package models

import (
	"time"

	"gorm.io/gorm"
)

type Message struct {
	gorm.Model
	ConversationID uint      `gorm:"index;not null"`
	Sender         string    `gorm:"size:20;not null"` // "user" or "bot"
	Text           string    `gorm:"type:text;not null"`
	Timestamp      time.Time `gorm:"autoCreateTime"`
}
