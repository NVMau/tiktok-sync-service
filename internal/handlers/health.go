package handlers

import (
	"github.com/gofiber/fiber/v2"
)

type HealthResponse struct {
	Status  string `json:"status"`
	Service string `json:"service"`
}

func HealthCheck(c *fiber.Ctx) error {
	return c.JSON(HealthResponse{
		Status:  "ok",
		Service: "tiktok-sync-server",
	})
}
