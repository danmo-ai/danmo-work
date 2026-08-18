package service

import (
	"fmt"
	"sync/atomic"
	"time"
)

var serviceIDCounter atomic.Uint64

// NewID returns a process-unique, chronologically sortable id.
//
// UnixNano alone is not unique on coarse-clock platforms (the Windows wall
// clock can tick in ~0.5–15ms steps), so concurrent creation (e.g. two
// sessions minted in one HTTP burst) could collide on a bare timestamp and
// violate primary keys. The fixed-width counter suffix keeps lexicographic
// order aligned with creation order.
func NewID(prefix string) string {
	return fmt.Sprintf("%s-%d-%06d", prefix, time.Now().UnixNano(), serviceIDCounter.Add(1)%1_000_000)
}
