package renames

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
	r.Get("/renames", h.List)
	r.Post("/renames", h.Create)
	r.Post("/renames/preview", h.Preview)
	r.Get("/renames/log", h.Log)
	r.Post("/renames/log/:id/undo", h.Undo)
	r.Put("/renames/:id", h.Update)
	r.Patch("/renames/:id/enabled", h.Enabled)
	r.Delete("/renames/:id", h.Delete)
	r.Post("/renames/:id/duplicate", h.Duplicate)
	r.Post("/renames/:id/apply", h.Apply)
}

func fail(c *fiber.Ctx, err error) error {
	var inv Invalid
	switch {
	case errors.As(err, &inv):
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": inv.Error()})
	case errors.Is(err, ErrNotFound):
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "preset not found"})
	}
	return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "internal server error"})
}

func id(c *fiber.Ctx) (int64, error) {
	v, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return 0, Invalid("The preset number must be a number.")
	}
	return v, nil
}

func bind[T any](c *fiber.Ctx) (T, error) {
	var v T
	if err := jsonutil.BindJSON(c, &v); err != nil {
		return v, Invalid(err.Error())
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

func (h *Handler) Create(c *fiber.Ctx) error {
	p, err := bind[Preset](c)
	if err != nil {
		return fail(c, err)
	}
	out, err := h.svc.Create(c.Context(), p)
	if err != nil {
		return fail(c, err)
	}
	return c.Status(fiber.StatusCreated).JSON(out)
}

func (h *Handler) Update(c *fiber.Ctx) error {
	pid, err := id(c)
	if err != nil {
		return fail(c, err)
	}
	p, err := bind[Preset](c)
	if err != nil {
		return fail(c, err)
	}
	p.ID = pid
	out, err := h.svc.Update(c.Context(), p)
	if err != nil {
		return fail(c, err)
	}
	return c.JSON(out)
}

type enabledReq struct {
	Enabled bool `json:"enabled"`
}

func (h *Handler) Enabled(c *fiber.Ctx) error {
	pid, err := id(c)
	if err != nil {
		return fail(c, err)
	}
	b, err := bind[enabledReq](c)
	if err != nil {
		return fail(c, err)
	}
	out, err := h.svc.SetEnabled(c.Context(), pid, b.Enabled)
	if err != nil {
		return fail(c, err)
	}
	return c.JSON(out)
}

func (h *Handler) Delete(c *fiber.Ctx) error {
	pid, err := id(c)
	if err != nil {
		return fail(c, err)
	}
	if err := h.svc.Delete(c.Context(), pid); err != nil {
		return fail(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *Handler) Duplicate(c *fiber.Ctx) error {
	pid, err := id(c)
	if err != nil {
		return fail(c, err)
	}
	out, err := h.svc.Duplicate(c.Context(), pid)
	if err != nil {
		return fail(c, err)
	}
	return c.Status(fiber.StatusCreated).JSON(out)
}

type applyReq struct {
	IDs []int64 `json:"ids"`
}

func (h *Handler) Apply(c *fiber.Ctx) error {
	pid, err := id(c)
	if err != nil {
		return fail(c, err)
	}
	b := applyReq{}
	if len(c.Body()) > 0 {
		if b, err = bind[applyReq](c); err != nil {
			return fail(c, err)
		}
	}
	res, err := h.svc.Apply(c.Context(), pid, b.IDs)
	if err != nil {
		return fail(c, err)
	}
	return c.JSON(res)
}

func (h *Handler) Preview(c *fiber.Ctx) error {
	b, err := bind[PreviewReq](c)
	if err != nil {
		return fail(c, err)
	}
	out, err := h.svc.Preview(c.Context(), b)
	if err != nil {
		return fail(c, err)
	}
	return c.JSON(out)
}

func (h *Handler) Log(c *fiber.Ctx) error {
	items, err := h.svc.Log(c.Context())
	if err != nil {
		return fail(c, err)
	}
	return c.JSON(items)
}

func (h *Handler) Undo(c *fiber.Ctx) error {
	lid, err := id(c)
	if err != nil {
		return fail(c, err)
	}
	if err := h.svc.Undo(c.Context(), lid); err != nil {
		return fail(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}
