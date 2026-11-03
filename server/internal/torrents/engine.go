package torrents

import "context"

type Source struct {
	Magnet  string
	Torrent []byte
}

type Meta struct {
	Hash      string
	Name      string
	Size      float64
	PieceSize float64
	Files     []File
	Trackers  []string
}

type Live struct {
	HaveInfo  bool
	Verifying bool
	Meta      Meta
	FileDone  []float64
	Dl, Ul    float64
	DlDelta   float64
	UlDelta   float64
	Seeds     int
	Leech     int
	Peers     []Peer
	Err       string
}

type EngineTotals struct {
	Conns    int
	DHTNodes int
	Dl, Ul   float64
}

type AddOptions struct {
	Dir    string
	Layout string
	Prios  []string
}

type RestoreOptions struct {
	AddOptions
	Magnet string
	Names  []string
}

type Engine interface {
	Name() string
	Port() int
	Hash(src Source) (string, error)
	Meta(ctx context.Context, src Source) (Meta, error)
	Add(src Source, o AddOptions) (Meta, error)
	Restore(hash string, o RestoreOptions) error
	Live(hash string) (Live, bool)
	Loaded(hash string) bool
	Unload(hash string)
	Forget(hash string, deleteData bool) error
	Recheck(hash string) error
	SetPriorities(hash string, prios []string)
	SetNames(hash string, names []string) ([]string, error)
	Move(hash, dir string, moveFiles bool) error
	Reannounce(hash string)
	SetRates(dl, ul float64)
	Totals() EngineTotals
	Close() error
}
