package models

import (
	"time"

	"gorm.io/datatypes"
)

type EventProcessStatus string

const (
	EventStatusPending   EventProcessStatus = "pending"
	EventStatusProcessed EventProcessStatus = "processed"
	EventStatusFailed    EventProcessStatus = "failed"
)

type WebhookEvent struct {
	ID             uint               `gorm:"primaryKey" json:"id"`
	ShopID         uint               `gorm:"index;not null" json:"shop_id"`
	EventID        string             `gorm:"size:255;uniqueIndex;not null" json:"event_id"`
	EventType      string             `gorm:"size:100;index" json:"event_type"`
	ReceivedAt     time.Time          `json:"received_at"`
	Payload        datatypes.JSON     `gorm:"type:jsonb" json:"payload"`
	SignatureValid bool               `json:"signature_valid"`
	ProcessedAt    *time.Time         `json:"processed_at"`
	ProcessStatus  EventProcessStatus `gorm:"size:50;default:pending;index" json:"process_status"`
	Error          string             `gorm:"type:text" json:"error"`
}

func (WebhookEvent) TableName() string {
	return "tiktok_sync.webhook_events"
}

type JobStatus string

const (
	JobStatusQueued    JobStatus = "queued"
	JobStatusRunning   JobStatus = "running"
	JobStatusSucceeded JobStatus = "succeeded"
	JobStatusFailed    JobStatus = "failed"
	JobStatusDead      JobStatus = "dead"
)

type JobType string

const (
	JobTypeProcessWebhook   JobType = "PROCESS_WEBHOOK_EVENT"
	JobTypeProductUpsert    JobType = "PRODUCT_UPSERT"
	JobTypeInventoryPush    JobType = "INVENTORY_PUSH"
	JobTypeShipPackage      JobType = "SHIP_PACKAGE"
	JobTypeOrderReconcile   JobType = "ORDER_RECONCILE"
)

type SyncJob struct {
	ID         uint           `gorm:"primaryKey" json:"id"`
	ShopID     uint           `gorm:"index;not null" json:"shop_id"`
	JobType    JobType        `gorm:"size:50;index;not null" json:"job_type"`
	DedupeKey  *string        `gorm:"size:255;uniqueIndex" json:"dedupe_key"`
	Payload    datatypes.JSON `gorm:"type:jsonb" json:"payload"`
	Status     JobStatus      `gorm:"size:50;default:queued;index" json:"status"`
	Attempts   int            `gorm:"default:0" json:"attempts"`
	RunAfter   time.Time      `gorm:"index" json:"run_after"`
	LastError  string         `gorm:"type:text" json:"last_error"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
}

func (SyncJob) TableName() string {
	return "tiktok_sync.sync_jobs"
}
