package tokens

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"slices"
	"strings"
	"time"

	"bitswarm/server/internal/docstore"
)

const (
	day      = int64(24 * 3600e3)
	alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	Prefix   = "swm_"
)

var ErrNotFound = docstore.ErrNotFound

type Invalid string

func (e Invalid) Error() string { return string(e) }

type Token struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Scope    string `json:"scope"`
	Created  int64  `json:"created"`
	Expires  int64  `json:"expires"`
	Last4    string `json:"last4"`
	Hash     string `json:"hash"`
	LastUsed int64  `json:"lastUsed"`
}

type View struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Scope    string `json:"scope"`
	Created  int64  `json:"created"`
	Expires  int64  `json:"expires"`
	Last4    string `json:"last4"`
	LastUsed int64  `json:"lastUsed"`
	Expired  bool   `json:"expired"`
}

type CreateReq struct {
	Name  string `json:"name"`
	Scope string `json:"scope"`
	Days  int    `json:"days"`
}

type Created struct {
	Token string `json:"token"`
	View  View   `json:"view"`
}

type Logger interface {
	Add(level, src, msg string)
}

type Service struct {
	col *docstore.Collection[Token]
	log Logger
	now func() time.Time
}

func NewCollection(db docstoreDB) *docstore.Collection[Token] {
	return docstore.NewCollection(db, "tokens", func(t *Token) *int64 { return &t.ID })
}

func NewService(col *docstore.Collection[Token], log Logger) *Service {
	return &Service{col: col, log: log, now: time.Now}
}

var scopeNames = map[string]string{"read": "read only", "control": "control torrents", "admin": "full access"}

func Hash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func generate() (string, error) {
	var b strings.Builder
	b.WriteString(Prefix)
	max := big.NewInt(int64(len(alphabet)))
	for i := 0; i < 40; i++ {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		b.WriteByte(alphabet[n.Int64()])
	}
	return b.String(), nil
}

func (s *Service) view(t Token) View {
	return View{ID: t.ID, Name: t.Name, Scope: t.Scope, Created: t.Created, Expires: t.Expires, Last4: t.Last4, LastUsed: t.LastUsed,
		Expired: t.Expires > 0 && t.Expires < s.now().UnixMilli()}
}

func (s *Service) List(ctx context.Context) ([]View, error) {
	ts, err := s.col.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]View, len(ts))
	for i := len(ts) - 1; i >= 0; i-- {
		out[len(ts)-1-i] = s.view(ts[i])
	}
	return out, nil
}

func (s *Service) Create(ctx context.Context, r CreateReq) (Created, error) {
	r.Name = strings.TrimSpace(r.Name)
	switch {
	case r.Name == "":
		return Created{}, Invalid("Give the token a name, so you know what uses it.")
	case len(r.Name) > 60:
		return Created{}, Invalid("Use 60 characters or fewer for the token name.")
	case !slices.Contains([]string{"read", "control", "admin"}, r.Scope):
		return Created{}, Invalid("Choose an access level.")
	case !slices.Contains([]int{0, 30, 90, 365}, r.Days):
		return Created{}, Invalid("Choose when the token expires.")
	}
	raw, err := generate()
	if err != nil {
		return Created{}, err
	}
	now := s.now().UnixMilli()
	t := Token{Name: r.Name, Scope: r.Scope, Created: now, Last4: raw[len(raw)-4:], Hash: Hash(raw)}
	if r.Days > 0 {
		t.Expires = now + int64(r.Days)*day
	}
	t, err = s.col.Create(ctx, t)
	if err != nil {
		return Created{}, err
	}
	exp := "never expiring"
	if r.Days > 0 {
		exp = fmt.Sprintf("expiring in %d days", r.Days)
	}
	s.log.Add("info", "API tokens", fmt.Sprintf("Created token “%s” with %s access, %s", t.Name, scopeNames[t.Scope], exp))
	return Created{Token: raw, View: s.view(t)}, nil
}

func (s *Service) Revoke(ctx context.Context, id int64) error {
	t, err := s.col.Get(ctx, id)
	if err != nil {
		return err
	}
	if err := s.col.Delete(ctx, id); err != nil {
		return err
	}
	s.log.Add("warn", "API tokens", fmt.Sprintf("Revoked token “%s”", t.Name))
	return nil
}

func (s *Service) ActiveCount(ctx context.Context) int {
	vs, _ := s.List(ctx)
	n := 0
	for _, v := range vs {
		if !v.Expired {
			n++
		}
	}
	return n
}

var ErrInvalidToken = errors.New("That token isn’t valid, or it has expired.")

func (s *Service) Verify(ctx context.Context, raw string) (string, bool) {
	if !strings.HasPrefix(raw, Prefix) {
		return "", false
	}
	h := Hash(raw)
	ts, err := s.col.List(ctx)
	if err != nil {
		return "", false
	}
	for _, t := range ts {
		if t.Hash != h {
			continue
		}
		if t.Expires > 0 && t.Expires < s.now().UnixMilli() {
			return "", false
		}
		t.LastUsed = s.now().UnixMilli()
		_ = s.col.Put(ctx, t)
		return t.Scope, true
	}
	return "", false
}

var readable = []string{"/api/torrents", "/api/stats", "/api/log", "/api/catalog"}

func Allowed(scope, method, path string) bool {
	switch scope {
	case "admin":
		return true
	case "read", "control":
		for _, p := range readable {
			if path == p || strings.HasPrefix(path, p+"/") {
				if scope == "read" || strings.HasPrefix(path, "/api/torrents") {
					return scope == "control" || method == "GET"
				}
				return method == "GET"
			}
		}
	}
	return false
}
