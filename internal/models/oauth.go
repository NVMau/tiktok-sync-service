package models

import (
	"time"
)

type OAuthToken struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	ShopID       uint      `gorm:"index;not null" json:"shop_id"`
	AccessToken  string    `gorm:"size:500;not null" json:"-"`
	RefreshToken string    `gorm:"size:500;not null" json:"-"`
	ExpiresAt    time.Time `json:"expires_at"`
	Scopes       string    `gorm:"type:text" json:"scopes"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (OAuthToken) TableName() string {
	return "tiktok_sync.oauth_tokens"
}
