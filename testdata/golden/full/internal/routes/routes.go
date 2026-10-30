package routes

import (
	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
	"net/http"
)

func New(db *gorm.DB, redisClient *redis.Client) *fiber.App {
	app := fiber.New()
	app.Get("/health", func(c *fiber.Ctx) error {
		if err := check(c.UserContext(), db, redisClient); err != nil {
			return c.Status(http.StatusServiceUnavailable).JSON(fiber.Map{"status": "error"})
		}
		return c.JSON(fiber.Map{"status": "ok"})
	})
	return app
}
