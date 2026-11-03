package backup

import (
	"errors"

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
	r.Get("/backup/counts", h.Counts)
	r.Post("/backup/export", h.Export)
	r.Post("/backup/check", h.Check)
	r.Post("/backup/apply", h.Apply)
}

func fail(c *fiber.Ctx, err error) error {
	var inv Invalid
	if errors.As(err, &inv) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": inv.Error()})
	}
	return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "internal server error"})
}

func bind[T any](c *fiber.Ctx) (T, error) {
	var v T
	if err := jsonutil.BindJSON(c, &v); err != nil {
		return v, Invalid(err.Error())
	}
	return v, nil
}

func (h *Handler) Counts(c *fiber.Ctx) error { return c.JSON(h.svc.Counts(c.Context())) }

func (h *Handler) Export(c *fiber.Ctx) error {
	r, err := bind[ExportReq](c)
	if err != nil {
		return fail(c, err)
	}
	out, err := h.svc.Export(c.Context(), r)
	if err != nil {
		return fail(c, err)
	}
	return c.JSON(out)
}

func (h *Handler) Check(c *fiber.Ctx) error {
	r, err := bind[CheckReq](c)
	if err != nil {
		return fail(c, err)
	}
	out, err := h.svc.Check(c.Context(), r.Text)
	if err != nil {
		return fail(c, err)
	}
	return c.JSON(out)
}

func (h *Handler) Apply(c *fiber.Ctx) error {
	r, err := bind[ApplyReq](c)
	if err != nil {
		return fail(c, err)
	}
	out, err := h.svc.Apply(c.Context(), r)
	if err != nil {
		return fail(c, err)
	}
	return c.JSON(out)
}
