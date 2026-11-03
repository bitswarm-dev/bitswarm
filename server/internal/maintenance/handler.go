package maintenance

import (
	"errors"
	"strconv"

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
	r.Get("/maintenance", h.List)
	r.Get("/maintenance/log", h.Log)
	r.Put("/maintenance/:id", h.Update)
	r.Patch("/maintenance/:id/enabled", h.Enabled)
	r.Get("/maintenance/:id/preview", h.Preview)
	r.Post("/maintenance/:id/run", h.Run)
}

func fail(c *fiber.Ctx, err error) error {
	var inv Invalid
	switch {
	case errors.As(err, &inv):
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": inv.Error()})
	case errors.Is(err, ErrNotFound):
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "task not found"})
	}
	return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "internal server error"})
}

func id(c *fiber.Ctx) (int64, error) {
	v, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return 0, Invalid("The task number must be a number.")
	}
	return v, nil
}

func (h *Handler) List(c *fiber.Ctx) error {
	v, err := h.svc.List(c.Context())
	if err != nil {
		return fail(c, err)
	}
	return c.JSON(v)
}

func (h *Handler) Log(c *fiber.Ctx) error {
	v, err := h.svc.Log(c.Context())
	if err != nil {
		return fail(c, err)
	}
	return c.JSON(v)
}

func (h *Handler) Update(c *fiber.Ctx) error {
	tid, err := id(c)
	if err != nil {
		return fail(c, err)
	}
	var t Task
	if err := jsonutil.BindJSON(c, &t); err != nil {
		return fail(c, Invalid(err.Error()))
	}
	t.ID = tid
	v, err := h.svc.Update(c.Context(), t)
	if err != nil {
		return fail(c, err)
	}
	return c.JSON(v)
}

type enabledReq struct {
	Enabled bool `json:"enabled"`
}

func (h *Handler) Enabled(c *fiber.Ctx) error {
	tid, err := id(c)
	if err != nil {
		return fail(c, err)
	}
	var b enabledReq
	if err := jsonutil.BindJSON(c, &b); err != nil {
		return fail(c, Invalid(err.Error()))
	}
	v, err := h.svc.SetEnabled(c.Context(), tid, b.Enabled)
	if err != nil {
		return fail(c, err)
	}
	return c.JSON(v)
}

func (h *Handler) Preview(c *fiber.Ctx) error {
	tid, err := id(c)
	if err != nil {
		return fail(c, err)
	}
	v, err := h.svc.Preview(c.Context(), tid)
	if err != nil {
		return fail(c, err)
	}
	return c.JSON(v)
}

func (h *Handler) Run(c *fiber.Ctx) error {
	tid, err := id(c)
	if err != nil {
		return fail(c, err)
	}
	v, err := h.svc.Run(c.Context(), tid)
	if err != nil {
		return fail(c, err)
	}
	return c.JSON(v)
}
