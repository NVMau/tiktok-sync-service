-- Migration: Add tiktok_shop_id to products and orders tables
-- Purpose: Store real TikTok shop ID for synchronization (instead of only FK to shops.id)

-- =====================================================
-- PRODUCTS TABLE
-- =====================================================

-- Add tiktok_shop_id column to products
ALTER TABLE tiktok_sync.products
ADD COLUMN IF NOT EXISTS tiktok_shop_id VARCHAR(100);

-- Update existing products with shop_id from shops table
UPDATE tiktok_sync.products p
SET tiktok_shop_id = s.shop_id
FROM tiktok_sync.shops s
WHERE p.shop_id = s.id
  AND p.tiktok_shop_id IS NULL;

-- Make it NOT NULL after backfill
ALTER TABLE tiktok_sync.products
ALTER COLUMN tiktok_shop_id SET NOT NULL;

-- Create index for querying by tiktok_shop_id
CREATE INDEX IF NOT EXISTS idx_products_tiktok_shop_id ON tiktok_sync.products(tiktok_shop_id);

-- =====================================================
-- ORDERS TABLE
-- =====================================================

-- Add tiktok_shop_id column to orders
ALTER TABLE tiktok_sync.orders
ADD COLUMN IF NOT EXISTS tiktok_shop_id VARCHAR(100);

-- Update existing orders with shop_id from shops table
UPDATE tiktok_sync.orders o
SET tiktok_shop_id = s.shop_id
FROM tiktok_sync.shops s
WHERE o.shop_id = s.id
  AND o.tiktok_shop_id IS NULL;

-- Make it NOT NULL after backfill
ALTER TABLE tiktok_sync.orders
ALTER COLUMN tiktok_shop_id SET NOT NULL;

-- Create index for querying by tiktok_shop_id
CREATE INDEX IF NOT EXISTS idx_orders_tiktok_shop_id ON tiktok_sync.orders(tiktok_shop_id);
