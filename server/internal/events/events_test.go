package events

import "testing"

type recorder struct{ got []string }

func (r *recorder) Emit(event string, _ map[string]any) { r.got = append(r.got, event) }

func TestBus_FansOut(t *testing.T) {
	bus := NewBus()
	a, b := &recorder{}, &recorder{}
	bus.Subscribe(a)
	bus.Subscribe(b)

	bus.Emit("torrent.added", nil)
	bus.Emit("torrent.paused", nil)

	for name, r := range map[string]*recorder{"a": a, "b": b} {
		if len(r.got) != 2 || r.got[0] != "torrent.added" {
			t.Errorf("%s got %v", name, r.got)
		}
	}
	Nop{}.Emit("x", nil)
}
