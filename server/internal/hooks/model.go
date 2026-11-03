package hooks

type Hook struct {
	ID          int64    `json:"id"`
	Kind        string   `json:"kind"`
	Name        string   `json:"name"`
	Method      string   `json:"method"`
	URL         string   `json:"url"`
	Headers     string   `json:"headers"`
	Body        string   `json:"body"`
	Cmd         string   `json:"cmd"`
	Cwd         string   `json:"cwd"`
	Timeout     int      `json:"timeout"`
	Stdin       bool     `json:"stdin"`
	Events      []string `json:"events"`
	Tag         string   `json:"tag"`
	Secret      string   `json:"secret"`
	ClearSecret bool     `json:"clearSecret"`
	Retries     int      `json:"retries"`
	Enabled     bool     `json:"enabled"`
	Count       int      `json:"count"`
	Last        int64    `json:"last"`
	LastEvent   string   `json:"lastEvent"`
}

type View struct {
	Hook
	HasSecret bool `json:"hasSecret"`
}

type Delivery struct {
	ID       int64       `json:"id"`
	T        int64       `json:"t"`
	HookID   int64       `json:"hookId"`
	Hook     string      `json:"hook"`
	Event    string      `json:"event"`
	Test     bool        `json:"test"`
	Kind     string      `json:"kind"`
	Method   string      `json:"method"`
	URL      string      `json:"url"`
	Headers  [][2]string `json:"headers"`
	Body     string      `json:"body"`
	Command  string      `json:"command"`
	Cwd      string      `json:"cwd"`
	Env      [][2]string `json:"env"`
	Stdin    string      `json:"stdin"`
	Timeout  int         `json:"timeout"`
	Text     string      `json:"text"`
	Copy     string      `json:"copy"`
	Status   string      `json:"status"`
	Attempts int         `json:"attempts"`
	Error    string      `json:"error"`
	Response int         `json:"response"`
	Output   string      `json:"output"`
	Millis   int64       `json:"millis"`
}

type Preview struct {
	Event   string   `json:"event"`
	Text    string   `json:"text"`
	Unknown []string `json:"unknown"`
	Empty   []string `json:"empty"`
	Signed  bool     `json:"signed"`

	MissingSecrets []string `json:"missingSecrets"`
}

type Config struct {
	CommandsEnabled bool `json:"commandsEnabled"`
}

var Events = []string{
	"torrent.added", "torrent.metadata", "torrent.resumed", "torrent.paused", "torrent.completed", "torrent.error", "torrent.removed",
	"torrent.renamed", "torrent.tagged", "torrent.moved",
	"schedule.search", "schedule.links", "schedule.finished",
	"maintenance.run", "settings.saved", "update.available",
}

var Placeholders = []string{"payload", "event", "time", "torrent.name", "torrent.hash", "torrent.tags", "torrent.size", "torrent.bytes", "torrent.path", "torrent.ratio", "torrent.state", "schedule.name", "task.name", "count"}
