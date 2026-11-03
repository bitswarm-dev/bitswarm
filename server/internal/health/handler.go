package health

import (
	"github.com/gofiber/fiber/v2"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) RegisterRoutes(r fiber.Router) {
	r.Get("/health", h.Latest)
	r.Post("/health/check", h.Check)
}

func (h *Handler) Check(c *fiber.Ctx) error {
	check, err := h.svc.Check(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "health check failed"})
	}
	return c.JSON(check)
}

func (h *Handler) Latest(c *fiber.Ctx) error {
	check, err := h.svc.Latest(c.Context())
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "no health checks recorded"})
	}
	return c.JSON(check)
}
