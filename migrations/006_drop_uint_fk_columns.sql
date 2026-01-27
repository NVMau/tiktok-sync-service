-- Migration 006: Drop old uint FK columns
-- After code refactor to use TikTok ID strings as foreign keys

-- ============================================================================
-- STEP 1: Add unique indexes on TikTok ID columns (for referential integrity)
-- ============================================================================

-- Unique index for order_items (composite key)
CREATE UNIQUE INDEX IF NOT EXISTS uidx_order_items_tiktok_order_item 
    ON tiktok_sync.order_items(tik_tok_order_id, tik_tok_order_item_id);

-- Unique index for oauth_tokens (one token per shop)
-- Note: This may fail if there are duplicates - check first
CREATE UNIQUE INDEX IF NOT EXISTS uidx_oauth_tokens_tiktok_shop 
    ON tiktok_sync.oauth_tokens(tik_tok_shop_id);

-- ============================================================================
-- STEP 2: Drop old uint FK columns
-- ============================================================================

-- Products: drop shop_id (use tik_tok_shop_id instead)
ALTER TABLE tiktok_sync.products DROP COLUMN IF EXISTS shop_id;

-- SKUs: drop product_id (use tik_tok_product_id instead)
ALTER TABLE tiktok_sync.skus DROP COLUMN IF EXISTS product_id;

-- Orders: drop shop_id (use tik_tok_shop_id instead)
ALTER TABLE tiktok_sync.orders DROP COLUMN IF EXISTS shop_id;

-- Order Items: drop order_id (use tik_tok_order_id instead)
ALTER TABLE tiktok_sync.order_items DROP COLUMN IF EXISTS order_id;

-- Order Items: drop sku_id (use tik_tok_sku_id instead for SKU lookup)
ALTER TABLE tiktok_sync.order_items DROP COLUMN IF EXISTS sku_id;

-- Webhook Events: drop shop_id (use tik_tok_shop_id instead)
ALTER TABLE tiktok_sync.webhook_events DROP COLUMN IF EXISTS shop_id;

-- Sync Jobs: drop shop_id (use tik_tok_shop_id instead)
ALTER TABLE tiktok_sync.sync_jobs DROP COLUMN IF EXISTS shop_id;

-- OAuth Tokens: drop shop_id (use tik_tok_shop_id instead)
ALTER TABLE tiktok_sync.oauth_tokens DROP COLUMN IF EXISTS shop_id;

-- ============================================================================
-- STEP 3: (Optional) Add foreign key constraints on string columns
-- Uncomment if you want referential integrity enforced at DB level
-- ============================================================================

-- ALTER TABLE tiktok_sync.products
--     ADD CONSTRAINT fk_products_shop 
--     FOREIGN KEY (tik_tok_shop_id) REFERENCES tiktok_sync.shops(shop_id) ON DELETE CASCADE;

-- ALTER TABLE tiktok_sync.skus
--     ADD CONSTRAINT fk_skus_product 
--     FOREIGN KEY (tik_tok_product_id) REFERENCES tiktok_sync.products(tik_tok_product_id) ON DELETE CASCADE;

-- ALTER TABLE tiktok_sync.orders
--     ADD CONSTRAINT fk_orders_shop 
--     FOREIGN KEY (tik_tok_shop_id) REFERENCES tiktok_sync.shops(shop_id) ON DELETE CASCADE;

-- ALTER TABLE tiktok_sync.order_items
--     ADD CONSTRAINT fk_order_items_order 
--     FOREIGN KEY (tik_tok_order_id) REFERENCES tiktok_sync.orders(tik_tok_order_id) ON DELETE CASCADE;

-- ALTER TABLE tiktok_sync.webhook_events
--     ADD CONSTRAINT fk_webhook_events_shop 
--     FOREIGN KEY (tik_tok_shop_id) REFERENCES tiktok_sync.shops(shop_id) ON DELETE CASCADE;

-- ALTER TABLE tiktok_sync.sync_jobs
--     ADD CONSTRAINT fk_sync_jobs_shop 
--     FOREIGN KEY (tik_tok_shop_id) REFERENCES tiktok_sync.shops(shop_id) ON DELETE CASCADE;

-- ALTER TABLE tiktok_sync.oauth_tokens
--     ADD CONSTRAINT fk_oauth_tokens_shop 
--     FOREIGN KEY (tik_tok_shop_id) REFERENCES tiktok_sync.shops(shop_id) ON DELETE CASCADE;
