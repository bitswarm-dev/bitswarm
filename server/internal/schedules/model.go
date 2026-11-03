package schedules

type Rule struct {
	ID           int64    `json:"id"`
	Name         string   `json:"name"`
	URL          string   `json:"url"`
	Terms        string   `json:"terms"`
	Selector     string   `json:"selector"`
	Attr         string   `json:"attr"`
	LinkType     string   `json:"linkType"`
	Include      string   `json:"include"`
	Exclude      string   `json:"exclude"`
	Tags         []string `json:"tags"`
	Path         string   `json:"path"`
	Max          int      `json:"max"`
	Start        bool     `json:"start"`
	SkipSeen     bool     `json:"skipSeen"`
	Every        int      `json:"every"`
	Unit         string   `json:"unit"`
	Enabled      bool     `json:"enabled"`
	EndDate      int64    `json:"endDate"`
	Ended        bool     `json:"ended"`
	RenamePreset int64    `json:"renamePreset"`
	LastRun      int64    `json:"lastRun"`
	LastFound    int      `json:"lastFound"`
	LastNew      int      `json:"lastNew"`
	Status       string   `json:"status"`
	LastError    string   `json:"lastError"`
	NextRun      int64    `json:"nextRun"`
	Seen         []string `json:"seen"`
}

type View struct {
	Rule
	Urls      []string `json:"urls"`
	RunsLeft  int      `json:"runsLeft"`
	SeenCount int      `json:"seenCount"`
}

type Item struct {
	Name  string `json:"name"`
	Type  string `json:"type"`
	URL   string `json:"url"`
	Key   string `json:"key"`
	Btih  string `json:"btih"`
	State string `json:"state"`
}

type Extraction struct {
	Error   string `json:"error,omitempty"`
	Matched int    `json:"matched"`
	Items   []Item `json:"items"`
}

type LogEntry struct {
	ID    int64    `json:"id"`
	T     int64    `json:"t"`
	Rule  string   `json:"rule"`
	Kind  string   `json:"kind"`
	Text  string   `json:"text"`
	Added []string `json:"added"`
}

type PreviewResult struct {
	Urls       []string   `json:"urls"`
	Extraction Extraction `json:"extraction"`
}
