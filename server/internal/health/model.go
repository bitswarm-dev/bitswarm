package health

import "time"

type Check struct {
	ID        int64     `json:"id"`
	CheckedAt time.Time `json:"checked_at"`
}
