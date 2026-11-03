package torrents

import (
	"errors"
	"strconv"
	"strings"
	"time"

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
	r.Get("/torrents", h.List)
	r.Post("/torrents", h.Add)
	r.Post("/torrents/batch", h.Batch)
	r.Get("/torrents/contents", h.Contents)
	r.Post("/torrents/contents", h.ContentsFile)
	r.Patch("/torrents/:id/files/:idx", h.Priority)
	r.Post("/torrents/:id/retry", h.Retry)
	r.Get("/stats", h.Stats)
	r.Put("/stats/alt", h.Alt)
}

func (h *Handler) List(c *fiber.Ctx) error {
	items, err := h.svc.List(c.Context())
	if err != nil {
		return fail(c, err)
	}
	return c.JSON(items)
}

func (h *Handler) Add(c *fiber.Ctx) error {
	a := AddSpec{Start: true}
	if err := jsonutil.BindJSON(c, &a); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	v, err := h.svc.Add(c.Context(), a)
	if err != nil {
		return fail(c, err)
	}
	return c.Status(fiber.StatusCreated).JSON(v)
}

func (h *Handler) Batch(c *fiber.Ctx) error {
	var r BatchReq
	if err := jsonutil.BindJSON(c, &r); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	msg, err := h.svc.Batch(c.Context(), r)
	if err != nil {
		return fail(c, err)
	}
	return c.JSON(fiber.Map{"message": msg})
}

const previewWait = 60 * time.Second

func (h *Handler) Contents(c *fiber.Ctx) error {
	if h.svc.Real() {
		m := strings.TrimSpace(c.Query("magnet"))
		if m == "" {
			return fail(c, Invalid("Paste a magnet link or choose a .torrent file first."))
		}
		if _, _, err := NameFromMagnet(m); err != nil {
			return fail(c, err)
		}
		return h.preview(c, Source{Magnet: m})
	}
	name := c.Query("name")
	if m := c.Query("magnet"); m != "" {
		n, _, err := NameFromMagnet(m)
		if err != nil {
			return fail(c, err)
		}
		name = n
	}
	if name == "" {
		return fail(c, Invalid("Paste a magnet link or choose a .torrent file first."))
	}
	return c.JSON(fiber.Map{"name": name, "contents": GenContents(name)})
}

type fileReq struct {
	FileName string `json:"fileName"`
	FileData []byte `json:"fileData"`
}

func (h *Handler) ContentsFile(c *fiber.Ctx) error {
	var b fileReq
	if err := jsonutil.BindJSON(c, &b); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	if !strings.HasSuffix(strings.ToLower(b.FileName), ".torrent") {
		return fail(c, Invalid("That file isn’t a .torrent file. Choose a file ending in .torrent."))
	}
	if !h.svc.Real() {
		name := strings.TrimSuffix(b.FileName, b.FileName[len(b.FileName)-len(".torrent"):])
		return c.JSON(fiber.Map{"name": name, "contents": GenContents(name)})
	}
	if len(b.FileData) == 0 {
		return fail(c, Invalid("That file is empty."))
	}
	return h.preview(c, Source{Torrent: b.FileData})
}

func (h *Handler) preview(c *fiber.Ctx, src Source) error {
	name, contents, err := h.svc.Preview(c.Context(), src, previewWait)
	if err != nil {
		return fail(c, err)
	}
	return c.JSON(fiber.Map{"name": name, "contents": contents})
}

type prioReq struct {
	Prio string `json:"prio"`
}

func (h *Handler) Priority(c *fiber.Ctx) error {
	id, err1 := strconv.ParseInt(c.Params("id"), 10, 64)
	idx, err2 := strconv.Atoi(c.Params("idx"))
	if err1 != nil || err2 != nil {
		return fail(c, Invalid("Torrent and file numbers must be numbers."))
	}
	var b prioReq
	if err := jsonutil.BindJSON(c, &b); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	if err := h.svc.SetPriority(c.Context(), id, idx, b.Prio); err != nil {
		return fail(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *Handler) Retry(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return fail(c, Invalid("Torrent number must be a number."))
	}
	if err := h.svc.Retry(c.Context(), id); err != nil {
		return fail(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *Handler) Stats(c *fiber.Ctx) error {
	st, err := h.svc.Stats(c.Context())
	if err != nil {
		return fail(c, err)
	}
	return c.JSON(st)
}

type altReq struct {
	On bool `json:"on"`
}

func (h *Handler) Alt(c *fiber.Ctx) error {
	var b altReq
	if err := jsonutil.BindJSON(c, &b); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	h.svc.SetAlt(b.On)
	return c.SendStatus(fiber.StatusNoContent)
}

func fail(c *fiber.Ctx, err error) error {
	var inv Invalid
	switch {
	case errors.As(err, &inv):
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": inv.Error()})
	case errors.Is(err, ErrNotFound):
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "torrent not found"})
	}
	return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "internal server error"})
}
