package settings

import "strings"

type Settings struct {
	SavePath          string  `json:"savePath"`
	UseIncomplete     bool    `json:"useIncomplete"`
	IncompletePath    string  `json:"incompletePath"`
	MoveCompleted     bool    `json:"moveCompleted"`
	CompletedPath     string  `json:"completedPath"`
	AppendExt         bool    `json:"appendExt"`
	Preallocate       bool    `json:"preallocate"`
	StartAuto         bool    `json:"startAuto"`
	CreateSubfolder   bool    `json:"createSubfolder"`
	DeleteTorrentFile bool    `json:"deleteTorrentFile"`
	WatchEnabled      bool    `json:"watchEnabled"`
	WatchPath         string  `json:"watchPath"`
	SearchDelay       int     `json:"searchDelay"`
	SpeedWindow       int     `json:"speedWindow"`
	Port              int     `json:"port"`
	Upnp              bool    `json:"upnp"`
	MaxConn           int     `json:"maxConn"`
	MaxConnPer        int     `json:"maxConnPer"`
	MaxUpSlots        int     `json:"maxUpSlots"`
	MaxUpSlotsPer     int     `json:"maxUpSlotsPer"`
	ProxyType         string  `json:"proxyType"`
	ProxyHost         string  `json:"proxyHost"`
	ProxyPort         int     `json:"proxyPort"`
	ProxyPeersOnly    bool    `json:"proxyPeersOnly"`
	ProxyAuth         bool    `json:"proxyAuth"`
	ProxyUser         string  `json:"proxyUser"`
	ProxyPass         string  `json:"proxyPass"`
	DlLimit           int     `json:"dlLimit"`
	UlLimit           int     `json:"ulLimit"`
	AltDl             int     `json:"altDl"`
	AltUl             int     `json:"altUl"`
	AltSchedule       bool    `json:"altSchedule"`
	AltFrom           string  `json:"altFrom"`
	AltTo             string  `json:"altTo"`
	AltDays           string  `json:"altDays"`
	LimitUtp          bool    `json:"limitUtp"`
	LimitLan          bool    `json:"limitLan"`
	Dht               bool    `json:"dht"`
	Pex               bool    `json:"pex"`
	Lsd               bool    `json:"lsd"`
	Encryption        string  `json:"encryption"`
	Anonymous         bool    `json:"anonymous"`
	Queueing          bool    `json:"queueing"`
	MaxActiveDl       int     `json:"maxActiveDl"`
	MaxActiveUp       int     `json:"maxActiveUp"`
	MaxActive         int     `json:"maxActive"`
	IgnoreSlow        bool    `json:"ignoreSlow"`
	RatioOn           bool    `json:"ratioOn"`
	Ratio             float64 `json:"ratio"`
	SeedTimeOn        bool    `json:"seedTimeOn"`
	SeedTime          int     `json:"seedTime"`
	SeedAction        string  `json:"seedAction"`
	WebPort           int     `json:"webPort"`
	WebHttps          bool    `json:"webHttps"`
	WebUser           string  `json:"webUser"`
	WebPass           string  `json:"webPass"`
	WebLocalBypass    bool    `json:"webLocalBypass"`
	AutoUpdate        bool    `json:"autoUpdate"`
}

var downloadDir = "/downloads"

func SetDownloadDir(dir string) { downloadDir = strings.TrimRight(dir, "/") }

func Defaults() Settings {
	return Settings{
		SavePath: downloadDir, IncompletePath: downloadDir + "/incomplete", CompletedPath: downloadDir + "/complete",
		StartAuto: true, CreateSubfolder: true, WatchPath: downloadDir + "/watch", SearchDelay: 5, SpeedWindow: 90,
		Port: 51413, Upnp: true, MaxConn: 500, MaxConnPer: 100, MaxUpSlots: 20, MaxUpSlotsPer: 4,
		ProxyType: "none", ProxyPort: 1080,
		AltDl: 2048, AltUl: 512, AltFrom: "08:00", AltTo: "18:00", AltDays: "weekdays", LimitUtp: true,
		Dht: true, Pex: true, Lsd: true, Encryption: "prefer",
		Queueing: true, MaxActiveDl: 3, MaxActiveUp: 5, MaxActive: 8, IgnoreSlow: true,
		Ratio: 2, SeedTime: 1440, SeedAction: "pause",
		WebPort: 8080, WebUser: "admin", WebLocalBypass: true, AutoUpdate: true,
	}
}
