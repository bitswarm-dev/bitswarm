package torrents

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestHandler(t *testing.T) {
	f := newFixture(t, true)
	app := fiber.New()
	NewHandler(f.svc).RegisterRoutes(app)

	tests := []struct {
		name, method, path, body string
		want                     int
	}{
		{"list", http.MethodGet, "/torrents", "", 200},
		{"add magnet", http.MethodPost, "/torrents", `{"magnet":"` + magnet + `","start":true}`, 201},
		{"add duplicate", http.MethodPost, "/torrents", `{"magnet":"` + magnet + `"}`, 400},
		{"add nothing", http.MethodPost, "/torrents", `{}`, 400},
		{"add bad json", http.MethodPost, "/torrents", `{`, 400},
		{"contents by name", http.MethodGet, "/torrents/contents?name=ubuntu.iso", "", 200},
		{"contents by magnet", http.MethodGet, "/torrents/contents?magnet=" + strings.ReplaceAll(magnet, "&", "%26"), "", 200},
		{"contents bad magnet", http.MethodGet, "/torrents/contents?magnet=nope", "", 400},
		{"contents nothing", http.MethodGet, "/torrents/contents", "", 400},
		{"batch pause", http.MethodPost, "/torrents/batch", `{"action":"pause","ids":[1]}`, 200},
		{"batch unknown id", http.MethodPost, "/torrents/batch", `{"action":"pause","ids":[999]}`, 404},
		{"batch unknown action", http.MethodPost, "/torrents/batch", `{"action":"x","ids":[1]}`, 400},
		{"batch bad json", http.MethodPost, "/torrents/batch", `{`, 400},
		{"priority ok", http.MethodPatch, "/torrents/3/files/1", `{"prio":"skip"}`, 204},
		{"priority bad", http.MethodPatch, "/torrents/3/files/1", `{"prio":"x"}`, 400},
		{"priority bad ids", http.MethodPatch, "/torrents/a/files/b", `{"prio":"skip"}`, 400},
		{"priority bad json", http.MethodPatch, "/torrents/3/files/1", `{`, 400},
		{"priority missing", http.MethodPatch, "/torrents/99/files/1", `{"prio":"skip"}`, 404},
		{"retry", http.MethodPost, "/torrents/8/retry", "", 204},
		{"retry bad id", http.MethodPost, "/torrents/x/retry", "", 400},
		{"retry missing", http.MethodPost, "/torrents/99/retry", "", 404},
		{"stats", http.MethodGet, "/stats", "", 200},
		{"alt on", http.MethodPut, "/stats/alt", `{"on":true}`, 204},
		{"alt bad json", http.MethodPut, "/stats/alt", `{`, 400},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			resp, err := app.Test(req)
			if err != nil {
				t.Fatal(err)
			}
			if resp.StatusCode != tc.want {
				t.Errorf("status = %d, want %d", resp.StatusCode, tc.want)
			}
		})
	}
}

func TestHandler_Contents(t *testing.T) {
	tests := []struct {
		name, method, path, body string
		real                     bool
		want                     int
		wantBody                 string
	}{
		{"simulated file", http.MethodPost, "/torrents/contents", `{"fileName":"ubuntu.iso.torrent","fileData":"eA=="}`, false, 200, "ubuntu.iso"},
		{"simulated wrong type", http.MethodPost, "/torrents/contents", `{"fileName":"x.zip","fileData":"eA=="}`, false, 400, "isn’t a .torrent file"},
		{"bad json", http.MethodPost, "/torrents/contents", `{`, false, 400, ""},
		{"real magnet", http.MethodGet, "/torrents/contents?magnet=" + strings.ReplaceAll(magnet, "&", "%26"), "", true, 200, "Album/a.flac"},
		{"real magnet missing", http.MethodGet, "/torrents/contents", "", true, 400, "Paste a magnet link"},
		{"real bad magnet", http.MethodGet, "/torrents/contents?magnet=nope", "", true, 400, "magnet link"},
		{"real file", http.MethodPost, "/torrents/contents", `{"fileName":"a.torrent","fileData":"eA=="}`, true, 200, "Album/b.flac"},
		{"real empty file", http.MethodPost, "/torrents/contents", `{"fileName":"a.torrent","fileData":""}`, true, 400, "empty"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t, false)
			if tc.real {
				f, _ = realFixture(t)
			}
			app := fiber.New()
			NewHandler(f.svc).RegisterRoutes(app)
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			resp, err := app.Test(req, 5000)
			if err != nil {
				t.Fatal(err)
			}
			body := new(strings.Builder)
			buf := make([]byte, 4096)
			for {
				n, rerr := resp.Body.Read(buf)
				body.Write(buf[:n])
				if rerr != nil {
					break
				}
			}
			if resp.StatusCode != tc.want || !strings.Contains(body.String(), tc.wantBody) {
				t.Errorf("status = %d body %q, want %d containing %q", resp.StatusCode, body.String(), tc.want, tc.wantBody)
			}
		})
	}
}
