-- Migration: Fix SKU indexes
-- Problem: 
--   1. seller_sku was UNIQUE but can duplicate across different products/shops
--   2. tik_tok_sku_id should be UNIQUE (it's the TikTok-generated ID)

-- Step 1: Drop the incorrect UNIQUE index on seller_sku
DROP INDEX IF EXISTS tiktok_sync.idx_tiktok_sync_skus_seller_sku;

-- Step 2: Create a regular index on seller_sku (for lookups, not unique)
CREATE INDEX IF NOT EXISTS idx_tiktok_sync_skus_seller_sku ON tiktok_sync.skus(seller_sku);

-- Step 3: Drop the non-unique index on tik_tok_sku_id
DROP INDEX IF EXISTS tiktok_sync.idx_tiktok_sync_skus_tik_tok_sk_uid;

-- Step 4: Create UNIQUE index on tik_tok_sku_id (TikTok ID is globally unique)
CREATE UNIQUE INDEX IF NOT EXISTS idx_tiktok_sync_skus_tik_tok_sku_id_unique 
ON tiktok_sync.skus(tik_tok_sku_id) 
WHERE tik_tok_sku_id IS NOT NULL;

-- Step 5: Add index on order_items.tik_tok_sk_uid for efficient joins
CREATE INDEX IF NOT EXISTS idx_tiktok_sync_order_items_tik_tok_sk_uid 
ON tiktok_sync.order_items(tik_tok_sk_uid);
