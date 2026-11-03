package torrents

import (
	"math"
	"regexp"
	"strings"
)

var (
	clients = []string{"qBittorrent 5.1.2", "Transmission 4.1.0", "Deluge 2.2.0", "libtorrent 2.0.11", "rTorrent 0.15.3", "BiglyBT 3.9", "KTorrent 24.12"}
	cc      = []string{"DE", "NL", "US", "JP", "FR", "SE", "CA", "BR", "PH", "FI", "PL", "GB", "KR", "AU"}
	ranges  = []string{"203.0.113.", "198.51.100.", "192.0.2."}
	slugRe  = regexp.MustCompile(`\s+`)
)

type spec struct {
	Name      string
	Size      float64
	Pct       float64
	Status    Status
	Folder    string
	Tags      []string
	Tracker   string
	Target    float64
	UpTarget  float64
	Seeds     int
	Leech     int
	AgeHours  float64
	CheckLeft int
	Uploaded  *float64
	Err       string
	Files     []fileSpec
}

type fileSpec struct {
	Name string
	Size float64
}

func frac(name string, f float64, size float64) fileSpec { return fileSpec{name, size * f} }

func pieceSizeFor(size float64) float64 {
	switch {
	case size > 8*GiB:
		return 16 * MiB
	case size > 2*GiB:
		return 4 * MiB
	case size > 500*MiB:
		return 2 * MiB
	}
	return 1 * MiB
}

func newTorrent(id int64, o spec, nowMs int64) Torrent {
	r := newRng(id*7919 + int64(len(o.Name))*31)
	files := o.Files
	if len(files) == 0 {
		files = []fileSpec{{o.Name, o.Size}}
	}
	out := make([]File, len(files))
	for i, f := range files {
		out[i] = File{Name: f.Name, Size: f.Size, Prio: "normal"}
	}
	done := o.Size * o.Pct
	uploaded := done * (0.2 + r.next()*1.8)
	if o.Uploaded != nil {
		uploaded = *o.Uploaded
	}
	age := o.AgeHours
	if age == 0 {
		age = 1
	}
	t := Torrent{
		Simulated: true, ID: id, Name: o.Name, Size: o.Size, Done: done, Status: o.Status,
		Target: o.Target, UpTarget: o.UpTarget, Uploaded: uploaded, Seeds: o.Seeds, Leech: o.Leech, Err: o.Err,
		Tracker: o.Tracker, Added: nowMs - int64(age*3600e3), Hash: r.hex(40), PieceSize: pieceSizeFor(o.Size),
		Files: out, Seed: int(r.next() * 1e6), CheckLeft: o.CheckLeft, LastActive: nowMs, Queue: int(id),
		Tags: append([]string{}, o.Tags...), Presets: []int64{},
		Path: "/downloads/" + slugRe.ReplaceAllString(strings.ToLower(o.Folder), "-"),
	}
	if o.Status == StatusSeeding {
		t.SeedSince = nowMs - int64(age*1800e3)
	}
	for i := 0; i < 20; i++ {
		t.Peers = append(t.Peers, newPeer(r))
	}
	return t
}

func newPeer(r *rng) Peer {
	prog := r.next()
	if r.next() < .55 {
		prog = 1
	}
	return Peer{
		IP:     ranges[int(r.next()*3)] + itoa(1+int(r.next()*253)) + ":" + itoa(6881+int(r.next()*4000)),
		Client: clients[int(r.next()*float64(len(clients)))], CC: cc[int(r.next()*float64(len(cc)))],
		Prog: prog, W: .2 + r.next(), Enc: r.next() < .7, Utp: r.next() < .5,
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	if neg {
		return "-" + string(b)
	}
	return string(b)
}

func samples() []spec {
	f := func(v float64) *float64 { return &v }
	return []spec{
		{Name: "ubuntu-26.04-desktop-amd64.iso", Size: 6.18 * GiB, Pct: .624, Status: StatusDownloading, Folder: "Linux ISOs", Tags: []string{"keep"}, Tracker: "https://torrent.ubuntu.com/announce", Target: 9.2 * MiB, UpTarget: 1.1 * MiB, Seeds: 1893, Leech: 214, AgeHours: 2},
		{Name: "debian-13.2.0-amd64-DVD-1.iso", Size: 3.92 * GiB, Pct: .231, Status: StatusDownloading, Folder: "Linux ISOs", Tracker: "http://bttracker.debian.org:6969/announce", Target: 4.6 * MiB, UpTarget: .4 * MiB, Seeds: 612, Leech: 88, AgeHours: 1},
		{Name: "enwiki-20261001-pages-articles-multistream.xml.bz2", Size: 24.8 * GiB, Pct: .081, Status: StatusDownloading, Folder: "Archives", Tags: []string{"large"}, Tracker: "udp://tracker.opentrackr.org:1337/announce", Target: 2.3 * MiB, UpTarget: .2 * MiB, Seeds: 41, Leech: 57, AgeHours: 5,
			Files: []fileSpec{frac("enwiki-20261001-pages-articles-multistream.xml.bz2", .94, 24.8*GiB), frac("enwiki-20261001-pages-articles-multistream-index.txt.bz2", .06, 24.8*GiB)}},
		{Name: "Fedora-Workstation-Live-x86_64-43", Size: 2.41 * GiB, Pct: 1, Status: StatusSeeding, Folder: "Linux ISOs", Tracker: "http://torrent.fedoraproject.org:6969/announce", UpTarget: 1.6 * MiB, Seeds: 744, Leech: 39, AgeHours: 96, Uploaded: f(6.9 * GiB),
			Files: []fileSpec{frac("Fedora-Workstation-Live-x86_64-43-1.6.iso", .998, 2.41*GiB), frac("Fedora-Workstation-43-1.6-x86_64-CHECKSUM", .002, 2.41*GiB)}},
		{Name: "archlinux-2026.10.01-x86_64.iso", Size: 1.33 * GiB, Pct: 1, Status: StatusSeeding, Folder: "Linux ISOs", Tracker: "DHT only", UpTarget: .7 * MiB, Seeds: 2210, Leech: 61, AgeHours: 240, Uploaded: f(4.1 * GiB)},
		{Name: "Big Buck Bunny (2160p, 60fps)", Size: .92 * GiB, Pct: 1, Status: StatusSeeding, Folder: "Open Movies", Tags: []string{"4K", "keep"}, Tracker: "udp://tracker.opentrackr.org:1337/announce", UpTarget: .35 * MiB, Seeds: 318, Leech: 12, AgeHours: 410, Uploaded: f(2.2 * GiB),
			Files: []fileSpec{frac("bbb_sunflower_2160p_60fps_normal.mp4", .92, .92*GiB), frac("bbb-poster.png", .006, .92*GiB), frac("README.txt", .0001, .92*GiB), frac("subtitles/bbb.en.srt", .0001, .92*GiB), frac("bbb_sunflower_2160p_60fps_stereo_abl.mp4.sha256", .0001, .92*GiB)}},
		{Name: "Sintel (Blender Open Movie)", Size: 1.61 * GiB, Pct: .412, Status: StatusPaused, Folder: "Open Movies", Tags: []string{"4K"}, Tracker: "udp://tracker.opentrackr.org:1337/announce", Target: 3.1 * MiB, Seeds: 204, Leech: 18, AgeHours: 30,
			Files: []fileSpec{frac("Sintel.2010.4k.mkv", .88, 1.61*GiB), frac("Sintel.2010.1080p.mkv", .115, 1.61*GiB), frac("sintel_en.srt", .002, 1.61*GiB), frac("sintel_es.srt", .002, 1.61*GiB), frac("poster.jpg", .001, 1.61*GiB)}},
		{Name: "Prelinger Archives — selected reels", Size: 12.1 * GiB, Pct: .553, Status: StatusError, Folder: "Archives", Tracker: "https://bt1.archive.org/announce", Target: 1.8 * MiB, Seeds: 0, Leech: 3, AgeHours: 48,
			Err:   "Tracker unreachable: announce timed out after 30 s",
			Files: []fileSpec{frac("Duck and Cover (1951).mp4", .18, 12.1*GiB), frac("A Is for Atom (1953).mp4", .22, 12.1*GiB), frac("Design for Dreaming (1956).mp4", .31, 12.1*GiB), frac("Hemo the Magnificent (1957).mp4", .29, 12.1*GiB)}},
		{Name: "Tears of Steel (1080p)", Size: .74 * GiB, Pct: 1, Status: StatusDone, Folder: "Open Movies", Tracker: "udp://tracker.opentrackr.org:1337/announce", Seeds: 150, Leech: 4, AgeHours: 720, Uploaded: f(1.6 * GiB)},
		{Name: "LibreOffice_26.2.1_Win_x86-64.msi", Size: .36 * GiB, Pct: 0, Status: StatusQueued, Folder: "Software", Tracker: "udp://tracker.opentrackr.org:1337/announce", Target: 5.5 * MiB, UpTarget: .3 * MiB, Seeds: 132, Leech: 9, AgeHours: .2},
	}
}

type Contents struct {
	Size  float64 `json:"size"`
	Files []File  `json:"files"`
}

var (
	isoRe   = regexp.MustCompile(`(?i)\.(iso|img|msi|exe|dmg)$`)
	videoRe = regexp.MustCompile(`(?i)\.(mkv|mp4)$|(1080p|2160p|720p|4k|bluray|web-?dl|movie|film)`)
	extRe   = regexp.MustCompile(`(?i)\.(iso|img|mkv|mp4|zip|msi|exe|dmg|xz|bz2|7z|tar|gz)$`)
	badRe   = regexp.MustCompile(`[\\/:*?"<>|]`)
)

func GenContents(name string) Contents {
	var h uint32 = 7
	for _, ch := range name {
		h = h*31 + uint32(ch)
	}
	r := newRng(int64(h))
	if h == 0 {
		r = newRng(1)
	}
	base := badRe.ReplaceAllString(extRe.ReplaceAllString(name, ""), "-")
	var parts []fileSpec
	switch {
	case isoRe.MatchString(name):
		parts = []fileSpec{{name, .9985}, {base + ".sha256", .0005}, {"README.txt", .001}}
	case videoRe.MatchString(name):
		parts = []fileSpec{{base + ".mkv", .93}, {base + ".en.srt", .0004}, {base + ".es.srt", .0004}, {"Extras/Behind the scenes.mp4", .0686}, {"poster.jpg", .0006}}
	default:
		parts = []fileSpec{{base + "/part-01.zip", .31}, {base + "/part-02.zip", .31}, {base + "/part-03.zip", .3741}, {base + "/docs/README.md", .002}, {base + "/docs/LICENSE.txt", .0029}, {base + "/SHA256SUMS", .001}}
	}
	size := (0.4 + r.next()*3.6) * GiB
	files := make([]File, len(parts))
	for i, p := range parts {
		files[i] = File{Name: p.Name, Size: math.Round(size * p.Size), Prio: "normal"}
	}
	return Contents{Size: size, Files: files}
}

func LayoutName(n, tname, layout string, files []File) string {
	roots := map[string]bool{}
	hasLoose := false
	for _, f := range files {
		if i := strings.Index(f.Name, "/"); i >= 0 {
			roots[f.Name[:i]] = true
		} else {
			hasLoose = true
		}
	}
	shared := ""
	if len(roots) == 1 && !hasLoose {
		for k := range roots {
			shared = k
		}
	}
	if layout == "nosub" && shared != "" {
		return n[len(shared)+1:]
	}
	if layout == "subfolder" && shared == "" && len(files) > 1 {
		return badRe.ReplaceAllString(tname, "-") + "/" + n
	}
	return n
}
