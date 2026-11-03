package torrents

import (
	"context"
	"testing"
)

func TestRepository_RoundTrip(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t, false)
	created, err := f.repo.Create(ctx, Torrent{Name: "a", Tags: []string{"x"}, RatioLimit: ptr(1.5), Files: []File{{Name: "f", Size: 1, Prio: "normal"}}})
	if err != nil {
		t.Fatal(err)
	}
	got, err := f.repo.Get(ctx, created.ID)
	if err != nil || got.Name != "a" || got.Tags[0] != "x" || *got.RatioLimit != 1.5 || got.Files[0].Name != "f" {
		t.Errorf("round trip = %+v, %v", got, err)
	}
}
