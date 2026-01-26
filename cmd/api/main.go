package main

import (
	"log"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"

	"github.com/user/sync-tiktok-mps/internal/config"
	"github.com/user/sync-tiktok-mps/internal/database"
	"github.com/user/sync-tiktok-mps/internal/handlers"
)

func main() {
	cfg := config.Load()

	if err := database.Connect(cfg.DatabaseURL); err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	if err := database.Migrate(); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	app := fiber.New(fiber.Config{
		AppName: "TikTok Sync Server",
	})

	app.Use(recover.New())
	app.Use(logger.New())
	app.Use(cors.New())

	app.Get("/healthz", handlers.HealthCheck)

	api := app.Group("/api/v1")

	webhookHandler := handlers.NewWebhookHandler(cfg)
	api.Post("/webhooks/tiktok", webhookHandler.HandleTikTokWebhook)
	api.Get("/webhooks/events", webhookHandler.GetWebhookEvents)
	api.Post("/webhooks/events/:id/retry", webhookHandler.RetryWebhookEvent)

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

	ordersHandler := handlers.NewOrdersHandler(cfg)
	api.Get("/orders", ordersHandler.ListOrders)
	api.Get("/orders/pending", ordersHandler.GetPendingOrders)
	api.Get("/orders/manual-review", ordersHandler.GetManualReviewOrders)
	api.Get("/orders/stats", ordersHandler.GetOrderStats)
	api.Get("/orders/:id", ordersHandler.GetOrder)
	api.Post("/orders/:id/sync", ordersHandler.SyncOrder)

	// TODO: Enable inventory sync later
	// inventoryHandler := handlers.NewInventoryHandler(cfg)
	// api.Get("/inventory", inventoryHandler.ListInventory)
	// api.Get("/inventory/stats", inventoryHandler.GetInventoryStats)
	// api.Get("/inventory/:id", inventoryHandler.GetInventory)
	// api.Put("/shops/:shop_id/inventory/:sim_id", inventoryHandler.UpdateInventory)
	// api.Post("/shops/:shop_id/inventory/:sim_id/sync", inventoryHandler.SyncInventory)
	// api.Post("/shops/:shop_id/inventory/sync-all", inventoryHandler.SyncAllDirty)
	// api.Get("/mappings", inventoryHandler.ListMappings)
	// api.Post("/shops/:shop_id/mappings", inventoryHandler.CreateMapping)

	fulfillmentHandler := handlers.NewFulfillmentHandler(cfg)
	api.Get("/shops/:shop_id/orders/ready-to-ship", fulfillmentHandler.GetReadyToShipOrders)
	api.Get("/shops/:shop_id/orders/in-transit", fulfillmentHandler.GetInTransitOrders)
	api.Post("/shops/:shop_id/orders/:order_id/ship", fulfillmentHandler.ShipOrder)
	api.Post("/shops/:shop_id/orders/:order_id/ship-async", fulfillmentHandler.ShipOrderAsync)
	api.Get("/shops/:shop_id/shipping-providers", fulfillmentHandler.GetShippingProviders)
	api.Get("/shops/:shop_id/warehouses", fulfillmentHandler.GetWarehouses)

	log.Printf("Starting server on port %s", cfg.Port)
	log.Printf("TikTok App Key: %s", cfg.TikTokAppKey)
	if err := app.Listen(":" + cfg.Port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
