package maintenance

type Params struct {
	Days   int     `json:"days"`
	Ratio  float64 `json:"ratio"`
	Files  string  `json:"files"`
	Hours  int     `json:"hours"`
	Action string  `json:"action"`
	GB     int     `json:"gb"`
	Resume bool    `json:"resume"`
}

type Task struct {
	ID       int64  `json:"id"`
	Type     string `json:"type"`
	Enabled  bool   `json:"enabled"`
	Every    int    `json:"every"`
	Unit     string `json:"unit"`
	Tag      string `json:"tag"`
	P        Params `json:"p"`
	LastRun  int64  `json:"lastRun"`
	LastText string `json:"lastText"`
	NextRun  int64  `json:"nextRun"`
}

type View struct {
	Task
	Title   string `json:"title"`
	Desc    string `json:"desc"`
	Summary string `json:"summary"`
	Scoped  bool   `json:"scoped"`
}

type LogEntry struct {
	ID    int64    `json:"id"`
	T     int64    `json:"t"`
	Task  string   `json:"task"`
	Kind  string   `json:"kind"`
	Text  string   `json:"text"`
	Added []string `json:"added"`
	More  int      `json:"more"`
}

type Item struct {
	ID   int64  `json:"-"`
	Ref  int64  `json:"-"`
	Name string `json:"name"`
	What string `json:"what"`
	Why  string `json:"why"`
}

type Preview struct {
	Items []Item `json:"items"`
	Done  string `json:"done"`
}

type OldEntry struct {
	ID    int64
	Label string
	T     int64
}

type Pruner interface {
	Source() string
	Old(cutoff int64) []OldEntry
	Remove(id int64)
}
