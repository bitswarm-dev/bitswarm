package schedules

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
	r.Get("/schedules", h.List)
	r.Post("/schedules", h.Create)
	r.Post("/schedules/test", h.Test)
	r.Get("/schedules/log", h.Log)
	r.Put("/schedules/:id", h.Update)
	r.Patch("/schedules/:id/enabled", h.Enabled)
	r.Delete("/schedules/:id", h.Delete)
	r.Post("/schedules/:id/run", h.Run)
	r.Get("/schedules/:id/preview", h.Preview)
	r.Post("/schedules/:id/download", h.Download)
}

func fail(c *fiber.Ctx, err error) error {
	var inv Invalid
	switch {
	case errors.As(err, &inv):
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": inv.Error()})
	case errors.Is(err, ErrNotFound):
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "schedule not found"})
	}
	return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "internal server error"})
}

func id(c *fiber.Ctx) (int64, error) {
	v, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return 0, Invalid("The schedule number must be a number.")
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
	r, err := bind[Rule](c)
	if err != nil {
		return fail(c, err)
	}
	v, err := h.svc.Create(c.Context(), r)
	if err != nil {
		return fail(c, err)
	}
	return c.Status(fiber.StatusCreated).JSON(v)
}

func (h *Handler) Update(c *fiber.Ctx) error {
	rid, err := id(c)
	if err != nil {
		return fail(c, err)
	}
	r, err := bind[Rule](c)
	if err != nil {
		return fail(c, err)
	}
	r.ID = rid
	v, err := h.svc.Update(c.Context(), r)
	if err != nil {
		return fail(c, err)
	}
	return c.JSON(v)
}

type enabledReq struct {
	Enabled bool `json:"enabled"`
}

func (h *Handler) Enabled(c *fiber.Ctx) error {
	rid, err := id(c)
	if err != nil {
		return fail(c, err)
	}
	b, err := bind[enabledReq](c)
	if err != nil {
		return fail(c, err)
	}
	v, err := h.svc.SetEnabled(c.Context(), rid, b.Enabled)
	if err != nil {
		return fail(c, err)
	}
	return c.JSON(v)
}

func (h *Handler) Delete(c *fiber.Ctx) error {
	rid, err := id(c)
	if err != nil {
		return fail(c, err)
	}
	if err := h.svc.Delete(c.Context(), rid); err != nil {
		return fail(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *Handler) Run(c *fiber.Ctx) error {
	rid, err := id(c)
	if err != nil {
		return fail(c, err)
	}
	v, err := h.svc.Run(c.Context(), rid, "manual")
	if err != nil {
		return fail(c, err)
	}
	return c.JSON(v)
}

func (h *Handler) Preview(c *fiber.Ctx) error {
	rid, err := id(c)
	if err != nil {
		return fail(c, err)
	}
	v, err := h.svc.Preview(c.Context(), rid)
	if err != nil {
		return fail(c, err)
	}
	return c.JSON(v)
}

type downloadReq struct {
	Keys []string `json:"keys"`
}

func (h *Handler) Download(c *fiber.Ctx) error {
	rid, err := id(c)
	if err != nil {
		return fail(c, err)
	}
	b, err := bind[downloadReq](c)
	if err != nil {
		return fail(c, err)
	}
	n, err := h.svc.DownloadFromPreview(c.Context(), rid, b.Keys)
	if err != nil {
		return fail(c, err)
	}
	return c.JSON(fiber.Map{"added": n})
}

func (h *Handler) Test(c *fiber.Ctx) error {
	r, err := bind[Rule](c)
	if err != nil {
		return fail(c, err)
	}
	v, err := h.svc.Test(c.Context(), r)
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
