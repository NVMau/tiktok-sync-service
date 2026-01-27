package models

import (
	"time"
)

type OAuthToken struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	TikTokShopID string    `gorm:"column:tik_tok_shop_id;size:100;uniqueIndex;not null" json:"tiktok_shop_id"` // FK → shops.shop_id (unique per shop)
	AccessToken  string    `gorm:"size:500;not null" json:"-"`
	RefreshToken string    `gorm:"size:500;not null" json:"-"`
	ExpiresAt    time.Time `json:"expires_at"`
	Scopes       string    `gorm:"type:text" json:"scopes"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (OAuthToken) TableName() string {
	return "tiktok_sync.oauth_tokens"
}
