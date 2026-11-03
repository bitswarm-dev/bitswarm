package http

import (
	"github.com/gofiber/fiber/v2"

	"bitswarm/server/internal/http/middleware"
	"bitswarm/server/internal/logger"
)

type Router func(app fiber.Router)

func New(log *logger.Logger, routers ...Router) *fiber.App {
	app := fiber.New()

	app.Use(middleware.RequestID())
	app.Use(middleware.Recover(log))
	app.Use(middleware.Logging(log))

	for _, register := range routers {
		register(app)
	}

	return app
}
