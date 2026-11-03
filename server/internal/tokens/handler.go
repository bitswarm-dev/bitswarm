package tokens

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
	r.Get("/tokens", h.List)
	r.Post("/tokens", h.Create)
	r.Delete("/tokens/:id", h.Revoke)
}

func fail(c *fiber.Ctx, err error) error {
	var inv Invalid
	switch {
	case errors.As(err, &inv):
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": inv.Error()})
	case errors.Is(err, ErrNotFound):
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "token not found"})
	}
	return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "internal server error"})
}

func (h *Handler) List(c *fiber.Ctx) error {
	v, err := h.svc.List(c.Context())
	if err != nil {
		return fail(c, err)
	}
	return c.JSON(v)
}

func (h *Handler) Create(c *fiber.Ctx) error {
	var r CreateReq
	if err := jsonutil.BindJSON(c, &r); err != nil {
		return fail(c, Invalid(err.Error()))
	}
	v, err := h.svc.Create(c.Context(), r)
	if err != nil {
		return fail(c, err)
	}
	return c.Status(fiber.StatusCreated).JSON(v)
}

func (h *Handler) Revoke(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return fail(c, Invalid("The token number must be a number."))
	}
	if err := h.svc.Revoke(c.Context(), id); err != nil {
		return fail(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}
