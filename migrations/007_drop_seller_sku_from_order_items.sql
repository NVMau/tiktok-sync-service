-- Migration 007: Drop seller_sku from order_items
-- seller_sku is redundant - we can lookup via tik_tok_sku_id -> skus table

-- Drop the index first
DROP INDEX IF EXISTS tiktok_sync.idx_order_items_seller_sku;

-- Drop the column
ALTER TABLE tiktok_sync.order_items DROP COLUMN IF EXISTS seller_sku;
