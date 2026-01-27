-- Migration: Add tiktok_product_id to skus table
-- Purpose: Store real TikTok product ID for synchronization (instead of only FK to products.id)

-- Add tiktok_product_id column to skus
ALTER TABLE tiktok_sync.skus
ADD COLUMN IF NOT EXISTS tiktok_product_id VARCHAR(100);

-- Update existing skus with tik_tok_product_id from products table
UPDATE tiktok_sync.skus s
SET tiktok_product_id = p.tik_tok_product_id
FROM tiktok_sync.products p
WHERE s.product_id = p.id
  AND s.tiktok_product_id IS NULL;

-- Make it NOT NULL after backfill
ALTER TABLE tiktok_sync.skus
ALTER COLUMN tiktok_product_id SET NOT NULL;

-- Create index for querying by tiktok_product_id
CREATE INDEX IF NOT EXISTS idx_skus_tiktok_product_id ON tiktok_sync.skus(tiktok_product_id);
