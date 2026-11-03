package settings

import (
	"github.com/gofiber/fiber/v2"

	"bitswarm/server/internal/jsonutil"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) RegisterRoutes(r fiber.Router) {
	r.Get("/settings", h.Get)
	r.Get("/settings/defaults", h.Defaults)
	r.Put("/settings", h.Save)
}

func (h *Handler) Get(c *fiber.Ctx) error {
	return c.JSON(h.svc.Get())
}

func (h *Handler) Defaults(c *fiber.Ctx) error {
	return c.JSON(Defaults())
}

func (h *Handler) Save(c *fiber.Ctx) error {
	next := h.svc.Get()
	if err := jsonutil.BindJSON(c, &next); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	saved, errs := h.svc.Save(c.Context(), next)
	if len(errs) > 0 {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{"errors": errs})
	}
	return c.JSON(saved)
}
