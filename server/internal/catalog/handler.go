package catalog

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
	r.Get("/catalog", h.Get)
	r.Get("/catalog/usage", h.Usage)
	r.Post("/catalog/tags", h.Add)
	r.Put("/catalog/tags", h.Rename)
	r.Delete("/catalog/tags", h.Delete)
}

func (h *Handler) Get(c *fiber.Ctx) error {
	return c.JSON(h.svc.Snapshot())
}

func (h *Handler) Usage(c *fiber.Ctx) error {
	return c.JSON(h.svc.Usage())
}

func fail(c *fiber.Ctx, err error) error {
	var inv Invalid
	if errors.As(err, &inv) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": inv.Error()})
	}
	return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "internal server error"})
}

type nameReq struct {
	Name string `json:"name"`
}

type renameReq struct {
	From string `json:"from"`
	To   string `json:"to"`
}

func (h *Handler) Add(c *fiber.Ctx) error {
	var in nameReq
	if err := jsonutil.BindJSON(c, &in); err != nil {
		return fail(c, Invalid(err.Error()))
	}
	name, err := h.svc.Add(c.Context(), in.Name)
	if err != nil {
		return fail(c, err)
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"name": name})
}

func (h *Handler) Rename(c *fiber.Ctx) error {
	var in renameReq
	if err := jsonutil.BindJSON(c, &in); err != nil {
		return fail(c, Invalid(err.Error()))
	}
	name, err := h.svc.Rename(c.Context(), in.From, in.To)
	if err != nil {
		return fail(c, err)
	}
	return c.JSON(fiber.Map{"name": name})
}

func (h *Handler) Delete(c *fiber.Ctx) error {
	if err := h.svc.Delete(c.Context(), c.Query("name")); err != nil {
		return fail(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}
