package torrents

import "math"

type Status string

const (
	StatusDownloading Status = "downloading"
	StatusChecking    Status = "checking"
	StatusRechecking  Status = "rechecking"
	StatusSeeding     Status = "seeding"
	StatusDone        Status = "done"
	StatusPaused      Status = "paused"
	StatusQueued      Status = "queued"
	StatusError       Status = "error"
)

const (
	KiB = 1024.0
	MiB = KiB * 1024
	GiB = MiB * 1024
)

type File struct {
	Name string  `json:"name"`
	Size float64 `json:"size"`
	Prio string  `json:"prio"`
	Orig string  `json:"orig,omitempty"`
	Done float64 `json:"done"`
}

type Peer struct {
	IP     string  `json:"ip"`
	Client string  `json:"client"`
	CC     string  `json:"cc"`
	Prog   float64 `json:"prog"`
	W      float64 `json:"w"`
	Enc    bool    `json:"enc"`
	Utp    bool    `json:"utp"`
	Dl     float64 `json:"dl"`
	Ul     float64 `json:"ul"`
}

type History struct {
	Dl []float64 `json:"dl"`
	Ul []float64 `json:"ul"`
}

type Torrent struct {
	ID          int64    `json:"id"`
	Name        string   `json:"name"`
	Hash        string   `json:"hash"`
	Size        float64  `json:"size"`
	Done        float64  `json:"done"`
	Status      Status   `json:"status"`
	Tags        []string `json:"tags"`
	Files       []File   `json:"files"`
	Path        string   `json:"path"`
	Tracker     string   `json:"tracker"`
	Seeds       int      `json:"seeds"`
	Leech       int      `json:"leech"`
	Dl          float64  `json:"dl"`
	Ul          float64  `json:"ul"`
	Uploaded    float64  `json:"uploaded"`
	Queue       int      `json:"queue"`
	Force       bool     `json:"force"`
	Seq         bool     `json:"seq"`
	Flp         bool     `json:"flp"`
	DlLimit     int      `json:"dlLimit"`
	UlLimit     int      `json:"ulLimit"`
	RatioLimit  *float64 `json:"ratioLimit"`
	SeedSince   int64    `json:"seedSince"`
	LastActive  int64    `json:"lastActive"`
	Added       int64    `json:"added"`
	Presets     []int64  `json:"presets"`
	RecheckTo   Status   `json:"recheckTo"`
	CheckLeft   int      `json:"checkLeft"`
	Err         string   `json:"err"`
	PieceSize   float64  `json:"pieceSize"`
	Seed        int      `json:"seed"`
	Target      float64  `json:"target"`
	UpTarget    float64  `json:"upTarget"`
	Peers       []Peer   `json:"peers"`
	Hist        History  `json:"hist"`
	Superseed   bool     `json:"superseed"`
	PausedBy    int64    `json:"pausedBy"`
	Simulated   bool     `json:"simulated"`
	Trackers    []string `json:"trackers"`
	Magnet      string   `json:"magnet,omitempty"`
	Layout      string   `json:"layout,omitempty"`
	Fingerprint string   `json:"-"`
}

type View struct {
	Torrent
	Wanted   float64  `json:"wanted"`
	Progress float64  `json:"progress"`
	Ratio    float64  `json:"ratio"`
	ETA      *float64 `json:"eta"`
	QueuePos *int     `json:"queuePos"`
}

func (t Torrent) Wanted() float64 {
	var sum float64
	for _, f := range t.Files {
		if f.Prio != "skip" {
			sum += f.Size
		}
	}
	if sum == 0 {
		return t.Size
	}
	return sum
}

func (t Torrent) Pct() float64 {
	w := t.Wanted()
	if w <= 0 {
		return 0
	}
	return math.Min(1, t.Done/w)
}

func (t Torrent) RatioValue() float64 {
	return t.Uploaded / math.Max(t.Done, 1)
}

func (t Torrent) ETA() *float64 {
	switch {
	case t.Status == StatusDownloading && t.Dl > 0:
		v := (t.Wanted() - t.Done) / t.Dl
		return &v
	case t.Status == StatusChecking:
		return nil
	}
	zero := 0.0
	return &zero
}

func (t Torrent) Unfinished() bool { return t.Pct() < 1 }

func (t Torrent) CanResume() bool {
	switch t.Status {
	case StatusPaused, StatusDone, StatusError, StatusQueued:
		return true
	}
	return false
}

func (t Torrent) CanPause() bool {
	switch t.Status {
	case StatusDownloading, StatusSeeding, StatusChecking, StatusQueued:
		return true
	}
	return false
}
