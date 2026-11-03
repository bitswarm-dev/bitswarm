package renames

type Preset struct {
	ID      int64  `json:"id"`
	Name    string `json:"name"`
	Find    string `json:"find"`
	Replace string `json:"replace"`
	I       bool   `json:"i"`
	G       bool   `json:"g"`
	KeepExt bool   `json:"keepExt"`
	Target  string `json:"target"`
	Tag     string `json:"tag"`
	When    string `json:"when"`
	Enabled bool   `json:"enabled"`
	Count   int    `json:"count"`
}

type LogEntry struct {
	ID        int64  `json:"id"`
	T         int64  `json:"t"`
	Rule      string `json:"rule"`
	From      string `json:"from"`
	To        string `json:"to"`
	Undone    bool   `json:"undone"`
	TorrentID int64  `json:"torrentId"`
	Kind      string `json:"kind"`
	Also      bool   `json:"also"`
}

type Row struct {
	From   string `json:"from"`
	To     string `json:"to"`
	Empty  bool   `json:"empty"`
	Clash  bool   `json:"clash"`
	Src    string `json:"src"`
	Match  bool   `json:"match"`
	Same   bool   `json:"same"`
	Tested string `json:"tested"`
}

type Preview struct {
	Error string `json:"error,omitempty"`
	Total int    `json:"total"`
	OK    int    `json:"ok"`
	Rows  []Row  `json:"rows"`
}

type Example struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type View struct {
	Preset
	Example *Example `json:"example"`
	UsedBy  []string `json:"usedBy"`
}
