package engine

import (
	"net"

	"github.com/anacrolix/torrent/bencode"
)

type torrentAddr string

func (a torrentAddr) Network() string { return "tcp" }
func (a torrentAddr) String() string  { return string(a) }

var _ net.Addr = torrentAddr("")

func bencodeMarshal(v any) ([]byte, error) { return bencode.Marshal(v) }
