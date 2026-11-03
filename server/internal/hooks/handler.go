package hooks

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
	r.Get("/hooks", h.List)
	r.Post("/hooks", h.Create)
	r.Get("/hooks/config", h.Config)
	r.Get("/hooks/deliveries", h.Deliveries)
	r.Post("/hooks/preview", h.Preview)
	r.Post("/hooks/parse-curl", h.ParseCurl)
	r.Put("/hooks/:id", h.Update)
	r.Patch("/hooks/:id/enabled", h.Enabled)
	r.Delete("/hooks/:id", h.Delete)
	r.Post("/hooks/:id/duplicate", h.Duplicate)
	r.Post("/hooks/:id/test", h.Test)
}

func fail(c *fiber.Ctx, err error) error {
	var inv Invalid
	switch {
	case errors.As(err, &inv):
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": inv.Error()})
	case errors.Is(err, ErrNotFound):
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "hook not found"})
	}
	return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "internal server error"})
}

func id(c *fiber.Ctx) (int64, error) {
	v, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return 0, Invalid("The hook number must be a number.")
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

func (h *Handler) Config(c *fiber.Ctx) error { return c.JSON(h.svc.Config()) }

func (h *Handler) ParseCurl(c *fiber.Ctx) error {
	req, err := bind[CurlRequest](c)
	if err != nil {
		return fail(c, err)
	}
	out, err := ParseCurl(req.Command)
	if err != nil {
		return fail(c, err)
	}
	return c.JSON(out)
}

func (h *Handler) Deliveries(c *fiber.Ctx) error {
	v, err := h.svc.Deliveries(c.Context())
	if err != nil {
		return fail(c, err)
	}
	return c.JSON(v)
}

func (h *Handler) Create(c *fiber.Ctx) error {
	hk, err := bind[Hook](c)
	if err != nil {
		return fail(c, err)
	}
	v, err := h.svc.Create(c.Context(), hk)
	if err != nil {
		return fail(c, err)
	}
	return c.Status(fiber.StatusCreated).JSON(v)
}

func (h *Handler) Update(c *fiber.Ctx) error {
	hid, err := id(c)
	if err != nil {
		return fail(c, err)
	}
	hk, err := bind[Hook](c)
	if err != nil {
		return fail(c, err)
	}
	hk.ID = hid
	v, err := h.svc.Update(c.Context(), hk)
	if err != nil {
		return fail(c, err)
	}
	return c.JSON(v)
}

type enabledReq struct {
	Enabled bool `json:"enabled"`
}

func (h *Handler) Enabled(c *fiber.Ctx) error {
	hid, err := id(c)
	if err != nil {
		return fail(c, err)
	}
	b, err := bind[enabledReq](c)
	if err != nil {
		return fail(c, err)
	}
	v, err := h.svc.SetEnabled(c.Context(), hid, b.Enabled)
	if err != nil {
		return fail(c, err)
	}
	return c.JSON(v)
}

func (h *Handler) Delete(c *fiber.Ctx) error {
	hid, err := id(c)
	if err != nil {
		return fail(c, err)
	}
	if err := h.svc.Delete(c.Context(), hid); err != nil {
		return fail(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *Handler) Duplicate(c *fiber.Ctx) error {
	hid, err := id(c)
	if err != nil {
		return fail(c, err)
	}
	v, err := h.svc.Duplicate(c.Context(), hid)
	if err != nil {
		return fail(c, err)
	}
	return c.Status(fiber.StatusCreated).JSON(v)
}

func (h *Handler) Test(c *fiber.Ctx) error {
	hid, err := id(c)
	if err != nil {
		return fail(c, err)
	}
	v, err := h.svc.Test(c.Context(), hid)
	if err != nil {
		return fail(c, err)
	}
	return c.JSON(v)
}

func (h *Handler) Preview(c *fiber.Ctx) error {
	hk, err := bind[Hook](c)
	if err != nil {
		return fail(c, err)
	}
	return c.JSON(h.svc.Preview(c.Context(), hk))
}
