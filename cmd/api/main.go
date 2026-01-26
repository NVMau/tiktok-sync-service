package main

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	fiberlogger "github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"go.uber.org/zap"

	"github.com/user/sync-tiktok-mps/internal/config"
	"github.com/user/sync-tiktok-mps/internal/database"
	"github.com/user/sync-tiktok-mps/internal/handlers"
	"github.com/user/sync-tiktok-mps/internal/logger"
)

func main() {
	// Load configuration
	cfg := config.Load()

	// Initialize logger
	logger.Init(&logger.Config{
		Level:       cfg.LogLevel,
		Environment: cfg.Environment,
		OutputPath:  cfg.LogOutput,
	})
	defer logger.Sync()

	log := logger.Log.Named("main")

	// Connect to database
	if err := database.Connect(cfg.DatabaseURL); err != nil {
		log.Fatal("failed to connect to database", zap.Error(err))
	}

	// Run migrations
	if err := database.Migrate(); err != nil {
		log.Fatal("failed to run migrations", zap.Error(err))
	}

	// Create Fiber app
	app := fiber.New(fiber.Config{
		AppName: "TikTok Sync Server",
	})

	// Middleware
	app.Use(recover.New())
	app.Use(fiberlogger.New(fiberlogger.Config{
		Format: "${time} | ${status} | ${latency} | ${ip} | ${method} | ${path}\n",
	}))
	app.Use(cors.New())

	// Health check
	app.Get("/healthz", handlers.HealthCheck)

	// API routes
	api := app.Group("/api/v1")
	registerRoutes(api, cfg)

	// Start server
	log.Info("starting server",
		zap.String("port", cfg.Port),
		zap.String("environment", cfg.Environment),
		zap.String("log_level", cfg.LogLevel))

	if err := app.Listen(":" + cfg.Port); err != nil {
		log.Fatal("failed to start server", zap.Error(err))
	}
}

// registerRoutes sets up all API routes
func registerRoutes(api fiber.Router, cfg *config.Config) {
	// Webhooks
	webhookHandler := handlers.NewWebhookHandler(cfg)
	api.Post("/webhooks/tiktok", webhookHandler.HandleTikTokWebhook)
	api.Get("/webhooks/events", webhookHandler.GetWebhookEvents)
	api.Post("/webhooks/events/:id/retry", webhookHandler.RetryWebhookEvent)

	// Admin
	adminHandler := handlers.NewAdminHandler(cfg)
	admin := api.Group("/admin")
	admin.Get("/auth/url", adminHandler.GetAuthorizationURL)
	admin.Get("/auth/callback", adminHandler.HandleAuthCallback)
	admin.Get("/shops", adminHandler.ListShops)
	admin.Get("/shops/:id", adminHandler.GetShopDetail)
	admin.Post("/shops/:id/refresh-token", adminHandler.RefreshShopToken)
	admin.Post("/shops/:id/test", adminHandler.TestShopConnection)
	admin.Get("/shops/:shop_id/products", adminHandler.GetProducts)
	admin.Get("/shops/:shop_id/orders", adminHandler.GetOrders)
	admin.Post("/shops/:shop_id/orders/sync", adminHandler.SyncOrders)

	// Orders
	ordersHandler := handlers.NewOrdersHandler(cfg)
	api.Get("/orders", ordersHandler.ListOrders)
	api.Get("/orders/pending", ordersHandler.GetPendingOrders)
	api.Get("/orders/manual-review", ordersHandler.GetManualReviewOrders)
	api.Get("/orders/stats", ordersHandler.GetOrderStats)
	api.Get("/orders/:id", ordersHandler.GetOrder)
	api.Post("/orders/:id/sync", ordersHandler.SyncOrder)

	// Products & SKUs
	inventoryHandler := handlers.NewInventoryHandler(cfg)

	// Products
	api.Get("/shops/:shop_id/products", inventoryHandler.ListProducts)
	api.Post("/shops/:shop_id/products/sync", inventoryHandler.SyncProducts)
	api.Get("/products/:id", inventoryHandler.GetProduct)

	// SKUs
	api.Get("/products/:product_id/skus", inventoryHandler.ListSKUs)
	api.Post("/products/:product_id/skus", inventoryHandler.CreateSKU)
	api.Post("/products/:product_id/skus/batch", inventoryHandler.CreateSKUsBatch)
	api.Get("/skus/:id", inventoryHandler.GetSKU)
	api.Put("/skus/:id/price", inventoryHandler.UpdateInventory)

	// Inventory sync
	api.Post("/shops/:shop_id/skus/:sku_id/sync", inventoryHandler.SyncSKUInventory)
	api.Post("/shops/:shop_id/products/:product_id/sync-inventory", inventoryHandler.SyncProductInventory)
	api.Get("/shops/:shop_id/inventory/stats", inventoryHandler.GetInventoryStats)
	api.Get("/shops/:shop_id/skus/pending", inventoryHandler.GetPendingSKUs)

	// Push SKU to TikTok
	api.Post("/shops/:shop_id/products/:product_id/push-skus", inventoryHandler.PushPendingSKUs)
	api.Post("/shops/:shop_id/skus/:sku_id/push", inventoryHandler.PushSingleSKU)

	// Legacy endpoints
	api.Get("/inventory", inventoryHandler.ListInventory)
	api.Get("/inventory/:id", inventoryHandler.GetInventory)

	// Fulfillment
	fulfillmentHandler := handlers.NewFulfillmentHandler(cfg)
	api.Get("/shops/:shop_id/orders/ready-to-ship", fulfillmentHandler.GetReadyToShipOrders)
	api.Get("/shops/:shop_id/orders/in-transit", fulfillmentHandler.GetInTransitOrders)
	api.Post("/shops/:shop_id/orders/:order_id/ship", fulfillmentHandler.ShipOrder)
	api.Post("/shops/:shop_id/orders/:order_id/ship-async", fulfillmentHandler.ShipOrderAsync)
	api.Get("/shops/:shop_id/shipping-providers", fulfillmentHandler.GetShippingProviders)
	api.Get("/shops/:shop_id/warehouses", fulfillmentHandler.GetWarehouses)
}
