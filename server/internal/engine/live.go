package engine

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/anacrolix/torrent"
	"github.com/anacrolix/torrent/metainfo"

	"bitswarm/server/internal/torrents"
)

func (e *Engine) Live(hash string) (torrents.Live, bool) {
	e.mu.Lock()
	en, ok := e.items[hash]
	e.mu.Unlock()
	if !ok {
		return torrents.Live{}, false
	}
	t := en.t
	st := t.Stats()
	now := time.Now()
	dlBytes, ulBytes := int64(st.BytesReadUsefulData.Int64()), int64(st.BytesWrittenData.Int64())

	e.mu.Lock()
	defer e.mu.Unlock()
	l := torrents.Live{Verifying: en.verify, Err: en.errText}
	if !en.lastAt.IsZero() {
		if dt := now.Sub(en.lastAt).Seconds(); dt >= 0.5 {
			en.dl = float64(dlBytes-en.lastDl) / dt
			en.ul = float64(ulBytes-en.lastUl) / dt
			l.DlDelta, l.UlDelta = float64(dlBytes-en.lastDl), float64(ulBytes-en.lastUl)
			en.lastAt, en.lastDl, en.lastUl = now, dlBytes, ulBytes
		}
	} else {
		en.lastAt, en.lastDl, en.lastUl = now, dlBytes, ulBytes
	}
	l.Dl, l.Ul = max(0, en.dl), max(0, en.ul)

	info := t.Info()
	if info == nil {
		return l, true
	}
	l.HaveInfo = true
	mi := t.Metainfo()
	l.Meta = metaOf(&mi, info, en.layout)
	files := t.Files()
	l.FileDone = make([]float64, len(files))
	for i, f := range files {
		l.FileDone[i] = float64(f.BytesCompleted())
		if i < len(l.Meta.Files) {
			if v, has := en.renames[l.Meta.Files[i].Name]; has {
				l.Meta.Files[i].Name = v
			}
		}
	}
	for _, pc := range t.PeerConns() {
		ps := pc.Stats()
		prog := 0.0
		if n := t.NumPieces(); n > 0 {
			prog = float64(ps.RemotePieceCount) / float64(n)
		}
		name, _ := pc.PeerClientName.Load().(string)
		ip := pc.RemoteAddr.String()
		if host, _, err := net.SplitHostPort(ip); err == nil {
			ip = host
		}
		p := torrents.Peer{IP: ip, Client: name, Prog: min(1, prog), Dl: ps.DownloadRate, Ul: ps.LastWriteUploadRate}
		p.Utp = pc.Network == "udp"
		l.Peers = append(l.Peers, p)
		if prog >= 1 {
			l.Seeds++
		} else {
			l.Leech++
		}
	}
	return l, true
}

func (e *Engine) Loaded(hash string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	_, ok := e.items[hash]
	return ok
}

func (e *Engine) SetNames(hash string, names []string) ([]string, error) {
	e.mu.Lock()
	en, ok := e.items[hash]
	e.mu.Unlock()
	if !ok || en.t.Info() == nil {
		return nil, errors.New("the torrent has no file list yet, so its files can’t be renamed")
	}
	info := en.t.Info()
	files := info.UpvertedFiles()
	type move struct{ from, to string }
	var moves []move
	final := append([]string(nil), names...)
	taken := map[string]bool{}
	next := map[string]string{}
	for k, v := range en.renames {
		next[k] = v
	}
	for i := range files {
		if i >= len(names) || names[i] == "" {
			continue
		}
		base := filepath.ToSlash(layoutPath(info, &files[i], en.layout))
		cur := base
		if v, has := en.renames[base]; has {
			cur = v
		}
		if cur == names[i] {
			continue
		}
		from, to := filepath.Join(en.dir, filepath.FromSlash(cur)), filepath.Join(en.dir, filepath.FromSlash(names[i]))
		if !isUnder(en.dir, to) {
			return nil, fmt.Errorf("%q is outside the download folder", names[i])
		}
		if _, err := os.Stat(to); err == nil || taken[to] {
			final[i] = freeName(en.dir, names[i], taken)
			to = filepath.Join(en.dir, filepath.FromSlash(final[i]))
		}
		taken[to] = true
		moves = append(moves, move{from, to})
		if final[i] == base {
			delete(next, base)
		} else {
			next[base] = final[i]
		}
	}
	if len(moves) == 0 {
		return final, nil
	}
	mi := en.t.Metainfo()
	en.t.Drop()
	for _, m := range moves {
		if _, err := os.Stat(m.from); err != nil {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(m.to), 0o755); err != nil {
			return nil, err
		}
		if err := os.Rename(m.from, m.to); err != nil {
			return nil, err
		}
	}
	return final, e.reopen(en, &mi, next)
}

var numberedRe = regexp.MustCompile(`^(.*?)(\.[A-Za-z0-9]{1,5})?$`)

func freeName(dir, name string, taken map[string]bool) string {
	m := numberedRe.FindStringSubmatch(name)
	for i := 2; ; i++ {
		n := fmt.Sprintf("%s (%d)%s", m[1], i, m[2])
		p := filepath.Join(dir, filepath.FromSlash(n))
		if _, err := os.Stat(p); err != nil && !taken[p] {
			return n
		}
	}
}

func isUnder(base, p string) bool {
	rel, err := filepath.Rel(base, p)
	return err == nil && rel != ".." && !(len(rel) > 2 && rel[:3] == ".."+string(os.PathSeparator))
}

func (e *Engine) Move(hash, dir string, moveFiles bool) error {
	e.mu.Lock()
	en, ok := e.items[hash]
	e.mu.Unlock()
	if !ok {
		return nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	info := en.t.Info()
	mi := en.t.Metainfo()
	en.t.Drop()
	if info != nil && moveFiles {
		files := info.UpvertedFiles()
		for i := range files {
			p := filepath.ToSlash(layoutPath(info, &files[i], en.layout))
			if v, has := en.renames[p]; has {
				p = v
			}
			from, to := filepath.Join(en.dir, filepath.FromSlash(p)), filepath.Join(dir, filepath.FromSlash(p))
			if _, err := os.Stat(from); err != nil {
				continue
			}
			if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
				return err
			}
			if err := moveFile(from, to); err != nil {
				return err
			}
		}
		var gone []string
		for i := range files {
			p := filepath.ToSlash(layoutPath(info, &files[i], en.layout))
			if v, has := en.renames[p]; has {
				p = v
			}
			gone = append(gone, filepath.Join(en.dir, filepath.FromSlash(p)))
		}
		pruneDirs(gone, en.dir)
	}
	en.dir = dir
	return e.reopen(en, &mi, en.renames)
}

func pruneDirs(paths []string, base string) {
	base = filepath.Clean(base)
	dirs := map[string]bool{}
	for _, p := range paths {
		dirs[filepath.Dir(p)] = true
	}
	for d := range dirs {
		for cur := filepath.Clean(d); cur != base && isUnder(base, cur); cur = filepath.Dir(cur) {
			if os.Remove(cur) != nil {
				break
			}
		}
	}
}

func moveFile(from, to string) error {
	if err := os.Rename(from, to); err == nil {
		return nil
	}
	in, err := os.Open(from)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(to)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		os.Remove(to)
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return os.Remove(from)
}

func (e *Engine) reopen(en *entry, mi *metainfo.MetaInfo, renames map[string]string) error {
	if len(mi.PieceLayers) == 0 {
		mi.PieceLayers = nil
	}
	spec, err := torrent.TorrentSpecFromMetaInfoErr(mi)
	if err != nil {
		return err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	en.renames = renames
	delete(e.items, en.hash)
	en.lastAt = time.Time{}
	return e.start(en, spec)
}
