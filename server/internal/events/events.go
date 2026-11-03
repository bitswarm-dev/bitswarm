package events

import "sync"

type Emitter interface {
	Emit(event string, data map[string]any)
}

type Bus struct {
	mu   sync.RWMutex
	subs []Emitter
}

func NewBus() *Bus {
	return &Bus{}
}

func (b *Bus) Subscribe(e Emitter) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.subs = append(b.subs, e)
}

func (b *Bus) Emit(event string, data map[string]any) {
	b.mu.RLock()
	subs := append([]Emitter(nil), b.subs...)
	b.mu.RUnlock()
	for _, s := range subs {
		s.Emit(event, data)
	}
}

type Nop struct{}

func (Nop) Emit(string, map[string]any) {}
