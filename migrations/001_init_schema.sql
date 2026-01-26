-- ============================================================================
-- TikTok Shop Sync - Database Schema
-- Generated from GORM AutoMigrate (exact match)
-- ============================================================================

-- Create schema
CREATE SCHEMA IF NOT EXISTS tiktok_sync;

-- ============================================================================
-- SHOPS - Thông tin TikTok Shop
-- ============================================================================
CREATE TABLE IF NOT EXISTS tiktok_sync.shops (
    id              BIGSERIAL PRIMARY KEY,
    shop_id         VARCHAR(100) NOT NULL,
    shop_cipher     VARCHAR(500),
    shop_name       VARCHAR(255),
    region          VARCHAR(50),
    status          VARCHAR(50) DEFAULT 'active',
    created_at      TIMESTAMP WITH TIME ZONE,
    updated_at      TIMESTAMP WITH TIME ZONE
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_tiktok_sync_shops_shop_id ON tiktok_sync.shops(shop_id);

-- ============================================================================
-- OAUTH_TOKENS - Access/Refresh tokens cho mỗi shop
-- ============================================================================
CREATE TABLE IF NOT EXISTS tiktok_sync.oauth_tokens (
    id              BIGSERIAL PRIMARY KEY,
    shop_id         BIGINT NOT NULL,
    access_token    VARCHAR(500) NOT NULL,
    refresh_token   VARCHAR(500) NOT NULL,
    expires_at      TIMESTAMP WITH TIME ZONE,
    scopes          TEXT,
    updated_at      TIMESTAMP WITH TIME ZONE
);

CREATE INDEX IF NOT EXISTS idx_tiktok_sync_oauth_tokens_shop_id ON tiktok_sync.oauth_tokens(shop_id);

-- ============================================================================
-- PRODUCTS - Sản phẩm trên TikTok Shop
-- ============================================================================
CREATE TABLE IF NOT EXISTS tiktok_sync.products (
    id                  BIGSERIAL PRIMARY KEY,
    shop_id             BIGINT NOT NULL,
    tik_tok_product_id  VARCHAR(100) NOT NULL,
    title               VARCHAR(500) NOT NULL,
    description         TEXT,
    category_id         VARCHAR(100),
    status              VARCHAR(50) DEFAULT 'ACTIVE',
    sku_count           BIGINT DEFAULT 0,
    main_images         JSONB,
    raw_data            JSONB,
    synced_at           TIMESTAMP WITH TIME ZONE,
    created_at          TIMESTAMP WITH TIME ZONE,
    updated_at          TIMESTAMP WITH TIME ZONE
);

CREATE INDEX IF NOT EXISTS idx_tiktok_sync_products_shop_id ON tiktok_sync.products(shop_id);
CREATE INDEX IF NOT EXISTS idx_tiktok_sync_products_status ON tiktok_sync.products(status);
CREATE UNIQUE INDEX IF NOT EXISTS idx_tiktok_sync_products_tik_tok_product_id ON tiktok_sync.products(tik_tok_product_id);

-- ============================================================================
-- SKUS - Mỗi SKU = 1 số điện thoại cụ thể
-- ============================================================================
CREATE TABLE IF NOT EXISTS tiktok_sync.skus (
    id                  BIGSERIAL PRIMARY KEY,
    product_id          BIGINT NOT NULL,
    tik_tok_sku_id      VARCHAR(100),
    seller_sku          VARCHAR(100) NOT NULL,
    price               NUMERIC(15,2) NOT NULL,
    original_price      NUMERIC(15,2),
    quantity            BIGINT NOT NULL DEFAULT 1,
    
    -- Sync status
    sync_status         VARCHAR(50) NOT NULL DEFAULT 'PENDING',
    sale_status         VARCHAR(50) NOT NULL DEFAULT 'AVAILABLE',
    error_message       TEXT,
    push_attempts       BIGINT DEFAULT 0,
    
    -- Variant attributes
    sales_attributes    JSONB,
    inventory_info      JSONB,
    
    -- Timestamps
    synced_at           TIMESTAMP WITH TIME ZONE,
    created_at          TIMESTAMP WITH TIME ZONE,
    updated_at          TIMESTAMP WITH TIME ZONE
);

CREATE INDEX IF NOT EXISTS idx_tiktok_sync_skus_product_id ON tiktok_sync.skus(product_id);
CREATE INDEX IF NOT EXISTS idx_tiktok_sync_skus_tik_tok_sk_uid ON tiktok_sync.skus(tik_tok_sku_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_tiktok_sync_skus_seller_sku ON tiktok_sync.skus(seller_sku);
CREATE INDEX IF NOT EXISTS idx_tiktok_sync_skus_sync_status ON tiktok_sync.skus(sync_status);
CREATE INDEX IF NOT EXISTS idx_tiktok_sync_skus_sale_status ON tiktok_sync.skus(sale_status);

-- ============================================================================
-- ORDERS - Mirror orders từ TikTok
-- ============================================================================
CREATE TABLE IF NOT EXISTS tiktok_sync.orders (
    id                      BIGSERIAL PRIMARY KEY,
    shop_id                 BIGINT NOT NULL,
    tik_tok_order_id        VARCHAR(100) NOT NULL,
    tik_tok_order_status    VARCHAR(50),
    payment_status          VARCHAR(50),
    buyer_info              JSONB,
    shipping_address        JSONB,
    total_amount            NUMERIC(15,2),
    currency                VARCHAR(10),
    placed_at               TIMESTAMP WITH TIME ZONE,
    paid_at                 TIMESTAMP WITH TIME ZONE,
    tracking_number         VARCHAR(100),
    shipping_provider       VARCHAR(100),
    raw_payload             JSONB,
    local_order_id          VARCHAR(50),
    sync_state              VARCHAR(50) DEFAULT 'new',
    created_at              TIMESTAMP WITH TIME ZONE,
    updated_at              TIMESTAMP WITH TIME ZONE
);

CREATE INDEX IF NOT EXISTS idx_tiktok_sync_orders_shop_id ON tiktok_sync.orders(shop_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_tiktok_sync_orders_tik_tok_order_id ON tiktok_sync.orders(tik_tok_order_id);
CREATE INDEX IF NOT EXISTS idx_tiktok_sync_orders_tik_tok_order_status ON tiktok_sync.orders(tik_tok_order_status);
CREATE INDEX IF NOT EXISTS idx_tiktok_sync_orders_sync_state ON tiktok_sync.orders(sync_state);
CREATE INDEX IF NOT EXISTS idx_tiktok_sync_orders_local_order_id ON tiktok_sync.orders(local_order_id);

-- ============================================================================
-- ORDER_ITEMS - Chi tiết items trong order
-- ============================================================================
CREATE TABLE IF NOT EXISTS tiktok_sync.order_items (
    id                      BIGSERIAL PRIMARY KEY,
    order_id                BIGINT NOT NULL,
    sk_uid                  BIGINT,
    tik_tok_order_item_id   VARCHAR(100),
    tik_tok_product_id      VARCHAR(100),
    tik_tok_sk_uid          VARCHAR(100),
    seller_sku              VARCHAR(100),
    qty                     BIGINT NOT NULL DEFAULT 1,
    price                   NUMERIC(15,2),
    created_at              TIMESTAMP WITH TIME ZONE
);

CREATE INDEX IF NOT EXISTS idx_tiktok_sync_order_items_order_id ON tiktok_sync.order_items(order_id);
CREATE INDEX IF NOT EXISTS idx_tiktok_sync_order_items_sk_uid ON tiktok_sync.order_items(sk_uid);
CREATE INDEX IF NOT EXISTS idx_tiktok_sync_order_items_tik_tok_order_item_id ON tiktok_sync.order_items(tik_tok_order_item_id);
CREATE INDEX IF NOT EXISTS idx_tiktok_sync_order_items_seller_sku ON tiktok_sync.order_items(seller_sku);

-- ============================================================================
-- WEBHOOK_EVENTS - Inbox webhook events (idempotent)
-- ============================================================================
CREATE TABLE IF NOT EXISTS tiktok_sync.webhook_events (
    id                  BIGSERIAL PRIMARY KEY,
    shop_id             BIGINT NOT NULL,
    event_id            VARCHAR(255) NOT NULL,
    event_type          VARCHAR(100),
    received_at         TIMESTAMP WITH TIME ZONE,
    payload             JSONB,
    signature_valid     BOOLEAN,
    processed_at        TIMESTAMP WITH TIME ZONE,
    process_status      VARCHAR(50) DEFAULT 'pending',
    error               TEXT
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_tiktok_sync_webhook_events_event_id ON tiktok_sync.webhook_events(event_id);
CREATE INDEX IF NOT EXISTS idx_tiktok_sync_webhook_events_shop_id ON tiktok_sync.webhook_events(shop_id);
CREATE INDEX IF NOT EXISTS idx_tiktok_sync_webhook_events_event_type ON tiktok_sync.webhook_events(event_type);
CREATE INDEX IF NOT EXISTS idx_tiktok_sync_webhook_events_process_status ON tiktok_sync.webhook_events(process_status);

-- ============================================================================
-- SYNC_JOBS - Outbox jobs để push lên TikTok
-- ============================================================================
CREATE TABLE IF NOT EXISTS tiktok_sync.sync_jobs (
    id              BIGSERIAL PRIMARY KEY,
    shop_id         BIGINT NOT NULL,
    job_type        VARCHAR(50) NOT NULL,
    dedupe_key      VARCHAR(255),
    payload         JSONB,
    status          VARCHAR(50) DEFAULT 'queued',
    attempts        BIGINT DEFAULT 0,
    run_after       TIMESTAMP WITH TIME ZONE,
    last_error      TEXT,
    created_at      TIMESTAMP WITH TIME ZONE,
    updated_at      TIMESTAMP WITH TIME ZONE
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_tiktok_sync_sync_jobs_dedupe_key ON tiktok_sync.sync_jobs(dedupe_key);
CREATE INDEX IF NOT EXISTS idx_tiktok_sync_sync_jobs_shop_id ON tiktok_sync.sync_jobs(shop_id);
CREATE INDEX IF NOT EXISTS idx_tiktok_sync_sync_jobs_job_type ON tiktok_sync.sync_jobs(job_type);
CREATE INDEX IF NOT EXISTS idx_tiktok_sync_sync_jobs_status ON tiktok_sync.sync_jobs(status);
CREATE INDEX IF NOT EXISTS idx_tiktok_sync_sync_jobs_run_after ON tiktok_sync.sync_jobs(run_after);

-- ============================================================================
-- COMMENTS
-- ============================================================================
COMMENT ON SCHEMA tiktok_sync IS 'TikTok Shop Sync - Schema for syncing orders and products';
COMMENT ON TABLE tiktok_sync.shops IS 'Connected TikTok Shops';
COMMENT ON TABLE tiktok_sync.oauth_tokens IS 'OAuth access/refresh tokens per shop';
COMMENT ON TABLE tiktok_sync.products IS 'Products synced from TikTok';
COMMENT ON TABLE tiktok_sync.skus IS 'SKUs - Each SKU is a phone number with quantity 0 or 1';
COMMENT ON TABLE tiktok_sync.orders IS 'Orders mirrored from TikTok';
COMMENT ON TABLE tiktok_sync.order_items IS 'Line items in each order';
COMMENT ON TABLE tiktok_sync.webhook_events IS 'Incoming webhook events with idempotency';
COMMENT ON TABLE tiktok_sync.sync_jobs IS 'Outbox pattern for async jobs';
