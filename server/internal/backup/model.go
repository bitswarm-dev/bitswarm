package backup

const Version = "0.9.0"

var Parts = []string{"settings", "theme", "tags", "schedules", "presets", "maintenance", "hooks"}

var PartNames = map[string]string{
	"settings": "Settings", "theme": "Theme", "tags": "Tags", "schedules": "Schedules",
	"presets": "Rename presets", "maintenance": "Maintenance tasks", "hooks": "Hooks",
}

type ExportReq struct {
	Parts   []string `json:"parts"`
	Secrets bool     `json:"secrets"`
}

type ExportResult struct {
	JSON string `json:"json"`
	Size int    `json:"size"`
}

type PlanPart struct {
	Key     string `json:"key"`
	Name    string `json:"name"`
	Summary string `json:"summary"`
}

type Plan struct {
	From  string     `json:"from"`
	At    string     `json:"at"`
	Parts []PlanPart `json:"parts"`
	Theme any        `json:"theme"`
}

type CheckReq struct {
	Text string `json:"text"`
}

type ApplyReq struct {
	Text string   `json:"text"`
	Use  []string `json:"use"`
	Mode string   `json:"mode"`
}

type Skipped struct {
	Part   string `json:"part"`
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

type Result struct {
	Done    []string  `json:"done"`
	Skipped []Skipped `json:"skipped"`
	Summary string    `json:"summary"`
}

type Invalid string

func (e Invalid) Error() string { return string(e) }
