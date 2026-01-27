-- Migration: Add TikTok IDs to remaining tables
-- Purpose: Store real TikTok IDs for synchronization consistency

-- =====================================================
-- ORDER_ITEMS TABLE - add tiktok_order_id
-- =====================================================

ALTER TABLE tiktok_sync.order_items
ADD COLUMN IF NOT EXISTS tiktok_order_id VARCHAR(100);

-- Update existing order_items with tik_tok_order_id from orders table
UPDATE tiktok_sync.order_items oi
SET tiktok_order_id = o.tik_tok_order_id
FROM tiktok_sync.orders o
WHERE oi.order_id = o.id
  AND oi.tiktok_order_id IS NULL;

-- Make it NOT NULL after backfill
ALTER TABLE tiktok_sync.order_items
ALTER COLUMN tiktok_order_id SET NOT NULL;

CREATE INDEX IF NOT EXISTS idx_order_items_tiktok_order_id ON tiktok_sync.order_items(tiktok_order_id);

-- =====================================================
-- WEBHOOK_EVENTS TABLE - add tiktok_shop_id
-- =====================================================

ALTER TABLE tiktok_sync.webhook_events
ADD COLUMN IF NOT EXISTS tiktok_shop_id VARCHAR(100);

-- Update existing webhook_events with shop_id from shops table
UPDATE tiktok_sync.webhook_events we
SET tiktok_shop_id = s.shop_id
FROM tiktok_sync.shops s
WHERE we.shop_id = s.id
  AND we.tiktok_shop_id IS NULL;

-- Make it NOT NULL after backfill
ALTER TABLE tiktok_sync.webhook_events
ALTER COLUMN tiktok_shop_id SET NOT NULL;

CREATE INDEX IF NOT EXISTS idx_webhook_events_tiktok_shop_id ON tiktok_sync.webhook_events(tiktok_shop_id);

-- =====================================================
-- SYNC_JOBS TABLE - add tiktok_shop_id
-- =====================================================

ALTER TABLE tiktok_sync.sync_jobs
ADD COLUMN IF NOT EXISTS tiktok_shop_id VARCHAR(100);

-- Update existing sync_jobs with shop_id from shops table
UPDATE tiktok_sync.sync_jobs sj
SET tiktok_shop_id = s.shop_id
FROM tiktok_sync.shops s
WHERE sj.shop_id = s.id
  AND sj.tiktok_shop_id IS NULL;

-- Make it NOT NULL after backfill
ALTER TABLE tiktok_sync.sync_jobs
ALTER COLUMN tiktok_shop_id SET NOT NULL;

CREATE INDEX IF NOT EXISTS idx_sync_jobs_tiktok_shop_id ON tiktok_sync.sync_jobs(tiktok_shop_id);

-- =====================================================
-- OAUTH_TOKENS TABLE - add tiktok_shop_id
-- =====================================================

ALTER TABLE tiktok_sync.oauth_tokens
ADD COLUMN IF NOT EXISTS tiktok_shop_id VARCHAR(100);

-- Update existing oauth_tokens with shop_id from shops table
UPDATE tiktok_sync.oauth_tokens ot
SET tiktok_shop_id = s.shop_id
FROM tiktok_sync.shops s
WHERE ot.shop_id = s.id
  AND ot.tiktok_shop_id IS NULL;

-- Make it NOT NULL after backfill
ALTER TABLE tiktok_sync.oauth_tokens
ALTER COLUMN tiktok_shop_id SET NOT NULL;

CREATE INDEX IF NOT EXISTS idx_oauth_tokens_tiktok_shop_id ON tiktok_sync.oauth_tokens(tiktok_shop_id);
