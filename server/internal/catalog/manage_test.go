package catalog

import (
	"context"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"

	"bitswarm/server/internal/db"
	"bitswarm/server/internal/docstore"
)

type fakeOwner struct {
	kind string
	tags map[string]int
}

func (f *fakeOwner) TagUse(tag string) Use {
	return Use{Kind: f.kind, Count: f.tags[tag], Names: []string{"a"}}
}

func (f *fakeOwner) RenameTag(from, to string) int {
	n := f.tags[from]
	if n > 0 {
		f.tags[to] += n
		delete(f.tags, from)
	}
	return n
}

func (f *fakeOwner) RemoveTag(tag string) int {
	n := f.tags[tag]
	delete(f.tags, tag)
	return n
}

func newManaged(t *testing.T) (*Service, *fakeOwner, *[]string) {
	t.Helper()
	conn, err := db.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	svc, err := NewService(docstore.NewKV(conn))
	if err != nil {
		t.Fatal(err)
	}
	o := &fakeOwner{kind: "hooks", tags: map[string]int{"4K": 2, "keep": 1}}
	svc.AddOwner(o)
	var logged []string
	svc.SetLog(func(_, _, msg string) { logged = append(logged, msg) })
	return svc, o, &logged
}

func TestAddRenameDelete(t *testing.T) {
	svc, o, logged := newManaged(t)
	ctx := context.Background()

	if name, err := svc.Add(ctx, " #Movies "); err != nil || name != "Movies" {
		t.Fatalf("Add() = %q, %v", name, err)
	}
	for _, bad := range []string{"", "a,b", "Movies"} {
		if _, err := svc.Add(ctx, bad); err == nil {
			t.Errorf("Add(%q) should fail", bad)
		}
	}

	if _, err := svc.Rename(ctx, "4K", "UHD"); err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(svc.TagNames(), "UHD") || slices.Contains(svc.TagNames(), "4K") || o.tags["UHD"] != 2 {
		t.Errorf("rename left tags=%v owner=%v", svc.TagNames(), o.tags)
	}

	if _, err := svc.Rename(ctx, "UHD", "keep"); err != nil {
		t.Fatal(err)
	}
	if slices.Contains(svc.TagNames(), "UHD") || o.tags["keep"] != 3 {
		t.Errorf("merge left tags=%v owner=%v", svc.TagNames(), o.tags)
	}
	if _, err := svc.Rename(ctx, "nope", "x"); err == nil {
		t.Error("renaming a missing tag should fail")
	}
	if name, err := svc.Rename(ctx, "keep", "keep"); err != nil || name != "keep" {
		t.Errorf("renaming to itself = %q, %v", name, err)
	}

	if err := svc.Delete(ctx, "keep"); err != nil || o.tags["keep"] != 0 || slices.Contains(svc.TagNames(), "keep") {
		t.Errorf("Delete() err=%v tags=%v owner=%v", err, svc.TagNames(), o.tags)
	}
	if err := svc.Delete(ctx, "keep"); err == nil {
		t.Error("deleting a missing tag should fail")
	}
	if len(*logged) < 4 || !strings.Contains((*logged)[1], "“4K” into “UHD”") {
		t.Errorf("log = %v", *logged)
	}
}

func TestUsage(t *testing.T) {
	svc, _, _ := newManaged(t)
	u := svc.Usage()
	if len(u["4K"]) != 1 || u["4K"][0].Count != 2 || len(u["large"]) != 0 {
		t.Errorf("Usage() = %+v", u)
	}
}

func TestHandler(t *testing.T) {
	svc, _, _ := newManaged(t)
	app := fiber.New()
	NewHandler(svc).RegisterRoutes(app)
	tests := []struct {
		name, method, path, body string
		want                     int
	}{
		{"get", http.MethodGet, "/catalog", "", 200},
		{"usage", http.MethodGet, "/catalog/usage", "", 200},
		{"add", http.MethodPost, "/catalog/tags", `{"name":"New"}`, 201},
		{"add duplicate", http.MethodPost, "/catalog/tags", `{"name":"New"}`, 400},
		{"add bad json", http.MethodPost, "/catalog/tags", `{`, 400},
		{"rename", http.MethodPut, "/catalog/tags", `{"from":"New","to":"Newer"}`, 200},
		{"rename missing", http.MethodPut, "/catalog/tags", `{"from":"zzz","to":"y"}`, 400},
		{"rename bad json", http.MethodPut, "/catalog/tags", `{`, 400},
		{"delete", http.MethodDelete, "/catalog/tags?name=Newer", "", 204},
		{"delete missing", http.MethodDelete, "/catalog/tags?name=Newer", "", 400},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			resp, err := app.Test(req)
			if err != nil || resp.StatusCode != tc.want {
				t.Errorf("%s %s = %v, %v; want %d", tc.method, tc.path, resp, err, tc.want)
			}
		})
	}
}
