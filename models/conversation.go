package models

import "gorm.io/gorm"

type Conversation struct {
	gorm.Model
	UserID   uint      `gorm:"not null;index"`
	Title    string    `gorm:"size:200"`
	Messages []Message `gorm:"constraint:OnDelete:CASCADE"`
}
