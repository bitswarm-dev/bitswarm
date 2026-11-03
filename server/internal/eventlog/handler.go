package eventlog

import (
	"strings"

	"github.com/gofiber/fiber/v2"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) RegisterRoutes(r fiber.Router) {
	r.Get("/log", h.Query)
	r.Delete("/log", h.Clear)
}

func (h *Handler) Query(c *fiber.Ctx) error {
	f := Filter{Src: c.Query("src"), Q: c.Query("q"), Limit: c.QueryInt("limit", 300), Levels: map[string]bool{}}
	if lv := c.Query("levels"); lv != "" {
		for _, l := range strings.Split(lv, ",") {
			f.Levels[strings.TrimSpace(l)] = true
		}
	}
	res, err := h.svc.Query(c.Context(), f)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "could not read the log"})
	}
	return c.JSON(res)
}

func (h *Handler) Clear(c *fiber.Ctx) error {
	if err := h.svc.Clear(c.Context()); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "could not clear the log"})
	}
	h.svc.Add("info", "App", "Cleared the execution log")
	return c.SendStatus(fiber.StatusNoContent)
}
