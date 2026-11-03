package jsonutil

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
)

type testPayload struct {
	Name string `json:"name"`
	Age  int    `json:"age"`
}

func TestUnmarshal(t *testing.T) {
	tests := []struct {
		name      string
		data      string
		wantErr   bool
		wantField string
	}{
		{name: "valid payload", data: `{"name":"a","age":30}`},
		{name: "unknown field", data: `{"name":"a","nickname":"b"}`, wantErr: true},
		{name: "wrong type", data: `{"name":"a","age":"thirty"}`, wantErr: true, wantField: "age"},
		{name: "malformed json", data: `{not json`, wantErr: true},
		{name: "empty body", data: ``, wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var dst testPayload
			err := Unmarshal([]byte(tc.data), &dst)

			if (err != nil) != tc.wantErr {
				t.Fatalf("Unmarshal() error = %v, wantErr %v", err, tc.wantErr)
			}

			if tc.wantField != "" {
				var decErr *DecodeError
				if !errors.As(err, &decErr) {
					t.Fatalf("expected *DecodeError, got %T", err)
				}
				if decErr.Field != tc.wantField {
					t.Errorf("DecodeError.Field = %q, want %q", decErr.Field, tc.wantField)
				}
			}
		})
	}
}

func TestUnmarshalStored(t *testing.T) {
	type doc struct {
		Name string `json:"name"`
	}
	var d doc
	if err := UnmarshalStored([]byte(`{"name":"x","removedField":1}`), &d); err != nil || d.Name != "x" {
		t.Errorf("stored data with a removed field = %+v, %v", d, err)
	}
	if err := Unmarshal([]byte(`{"name":"x","removedField":1}`), &d); err == nil {
		t.Error("request bodies must still reject unknown fields")
	}
	if err := UnmarshalStored([]byte(`{`), &d); err == nil {
		t.Error("broken stored JSON should still fail")
	}
}

func TestMarshal(t *testing.T) {
	got, err := Marshal(testPayload{Name: "a", Age: 30})
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	want := `{"name":"a","age":30}`
	if string(got) != want {
		t.Errorf("Marshal() = %s, want %s", got, want)
	}
}

func TestBindJSON(t *testing.T) {
	app := fiber.New()
	app.Post("/test", func(c *fiber.Ctx) error {
		var dst testPayload
		if err := BindJSON(c, &dst); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(dst)
	})

	tests := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{name: "valid body", body: `{"name":"a","age":30}`, wantStatus: fiber.StatusOK},
		{name: "invalid body", body: `{not json`, wantStatus: fiber.StatusBadRequest},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")

			resp, err := app.Test(req)
			if err != nil {
				t.Fatalf("app.Test() error = %v", err)
			}
			defer resp.Body.Close()
			io.Copy(io.Discard, resp.Body)

			if resp.StatusCode != tc.wantStatus {
				t.Errorf("status = %d, want %d", resp.StatusCode, tc.wantStatus)
			}
		})
	}
}

func TestDecodeError_Error(t *testing.T) {
	tests := []struct {
		name string
		err  *DecodeError
		want string
	}{
		{name: "with field", err: &DecodeError{Field: "age", Message: "expected number"}, want: `field "age": expected number`},
		{name: "without field", err: &DecodeError{Message: "invalid JSON body"}, want: "invalid JSON body"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.err.Error(); got != tc.want {
				t.Errorf("Error() = %q, want %q", got, tc.want)
			}
		})
	}
}
