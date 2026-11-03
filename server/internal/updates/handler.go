package updates

import "github.com/gofiber/fiber/v2"

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) RegisterRoutes(r fiber.Router) {
	r.Get("/updates", func(c *fiber.Ctx) error { return c.JSON(h.svc.State()) })
	r.Post("/updates/check", func(c *fiber.Ctx) error { return c.JSON(h.svc.Check(c.Context())) })
}
