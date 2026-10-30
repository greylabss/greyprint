package routes

import (
	"context"
	"github.com/gofiber/fiber/v2"
)

func New(dbPing, cachePing func(context.Context) error) *fiber.App {
	app := fiber.New()
	app.Get("/health", func(c *fiber.Ctx) error {
		if dbPing != nil {
			if err := dbPing(c.UserContext()); err != nil {
				return c.Status(503).JSON(fiber.Map{"status": "error"})
			}
		}
		if cachePing != nil {
			if err := cachePing(c.UserContext()); err != nil {
				return c.Status(503).JSON(fiber.Map{"status": "error"})
			}
		}
		return c.JSON(fiber.Map{"status": "ok"})
	})
	return app
}
