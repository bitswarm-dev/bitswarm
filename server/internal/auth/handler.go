package auth

import (
	"context"
	"net"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"bitswarm/server/internal/jsonutil"
)

const cookieName = "swarm_session"

type Handler struct {
	svc     *Service
	verify  func(ctx context.Context, raw string) (string, bool)
	allowed func(scope, method, path string) bool
}

func (h *Handler) SetTokens(verify func(ctx context.Context, raw string) (string, bool), allowed func(scope, method, path string) bool) {
	h.verify, h.allowed = verify, allowed
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) RegisterPublic(r fiber.Router) {
	r.Get("/auth/status", h.Status)
	r.Post("/auth/login", h.Login)
	r.Post("/auth/logout", h.Logout)
}

type loginReq struct {
	User string `json:"user"`
	Pass string `json:"pass"`
	Keep bool   `json:"keep"`
}

func isLocal(c *fiber.Ctx) bool {
	ip := c.IP()
	if host, _, err := net.SplitHostPort(ip); err == nil {
		ip = host
	}
	return ip == "127.0.0.1" || ip == "::1"
}

func (h *Handler) Authenticated(c *fiber.Ctx) bool {
	if h.svc.LocalBypass() && isLocal(c) {
		return true
	}
	return h.svc.Valid(c.Cookies(cookieName))
}

func (h *Handler) Status(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"signedIn":    h.Authenticated(c),
		"demo":        h.svc.UsingDemoPassword(),
		"user":        h.svc.Username(),
		"localBypass": h.svc.LocalBypass() && isLocal(c),
	})
}

func (h *Handler) Login(c *fiber.Ctx) error {
	var b loginReq
	if err := jsonutil.BindJSON(c, &b); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	res := h.svc.Login(strings.TrimSpace(b.User), b.Pass)
	if !res.OK {
		status := fiber.StatusUnauthorized
		if res.LockedFor > 0 {
			status = fiber.StatusTooManyRequests
		}
		return c.Status(status).JSON(fiber.Map{"error": res.Message, "triesLeft": res.TriesLeft, "lockedFor": int(res.LockedFor.Seconds()) + 1})
	}
	cookie := &fiber.Cookie{Name: cookieName, Value: res.Token, HTTPOnly: true, SameSite: "Lax", Path: "/"}
	if b.Keep {
		cookie.Expires = time.Now().Add(SessionTTL)
	}
	c.Cookie(cookie)
	return c.JSON(fiber.Map{"ok": true})
}

func (h *Handler) Logout(c *fiber.Ctx) error {
	h.svc.Logout(c.Cookies(cookieName))
	c.ClearCookie(cookieName)
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *Handler) Require() fiber.Handler {
	return func(c *fiber.Ctx) error {
		if h.Authenticated(c) {
			return c.Next()
		}
		if raw, ok := strings.CutPrefix(c.Get("Authorization"), "Bearer "); ok && h.verify != nil {
			scope, valid := h.verify(c.Context(), strings.TrimSpace(raw))
			if !valid {
				return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "That token isn’t valid, or it has expired."})
			}
			if h.allowed == nil || !h.allowed(scope, c.Method(), c.Path()) {
				return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "This token doesn’t have access to that."})
			}
			return c.Next()
		}
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Sign in to continue."})
	}
}
