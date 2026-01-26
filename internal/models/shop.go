package models

import (
	"time"
)

type Shop struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	ShopID     string    `gorm:"uniqueIndex;size:100;not null" json:"shop_id"`
	ShopCipher string    `gorm:"size:500" json:"shop_cipher"`
	ShopName   string    `gorm:"size:255" json:"shop_name"`
	Region     string    `gorm:"size:50" json:"region"`
	Status     string    `gorm:"size:50;default:active" json:"status"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func (Shop) TableName() string {
	return "tiktok_sync.shops"
}
