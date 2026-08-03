package runtime

import (
	"fmt"
	"sync/atomic"
	"time"
)

var runtimeIDCounter atomic.Uint64

// newRuntimeID returns a process-unique, chronologically sortable id.
//
// UnixNano alone is not unique on coarse-clock platforms (the Windows wall
// clock can tick in ~0.5–15ms steps) and parallel delegate_agent calls mint
// child turn ids concurrently, so a bare timestamp can collide across
// goroutines — colliding turn ids would share one JSONL file and violate the
// turns primary key. The fixed-width counter suffix keeps lexicographic order
// aligned with creation order, which turn history replay (sorted ListTurnIDs)
// and the compaction retain cursor (`id < retainFrom`) rely on.
func newRuntimeID(prefix string) string {
	return fmt.Sprintf("%s-%d-%06d", prefix, time.Now().UnixNano(), runtimeIDCounter.Add(1)%1_000_000)
}
