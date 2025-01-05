package stats

import (
	"encoding/json"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// Stats holds atomic counters for different query types.
type Stats struct {
	Read         int64
	Write        int64
	Begin        int64
	Commit       int64
	Rollback     int64
	HTTPRequests int64
}

// DetailedStats is the JSON-friendly representation of Stats.
type DetailedStats struct {
	Read         int64 `json:"read"`
	Write        int64 `json:"write"`
	Begin        int64 `json:"begin"`
	Commit       int64 `json:"commit"`
	Rollback     int64 `json:"rollback"`
	HTTPRequests int64 `json:"httpRequests"`
}

// StatsWithMinute links a minute key with its stats.
type StatsWithMinute struct {
	Minute       string `json:"minute"`
	Read         int64  `json:"read"`
	Write        int64  `json:"write"`
	Begin        int64  `json:"begin"`
	Commit       int64  `json:"commit"`
	Rollback     int64  `json:"rollback"`
	HTTPRequests int64  `json:"httpRequests"`
}

// DBStats manages per-minute statistics, total stats,
// queued writes, and queued transactions.
type DBStats struct {
	minutes sync.Map // key: string (minute in RFC3339) -> value: *Stats
	total   Stats

	queuedWrites       int64
	queuedTransactions int64
	queuedHTTPRequests int64

	stopChan chan bool
}

// NewDBStats creates a DBStats instance and starts a cleanup worker
// that runs every 10 seconds to remove stats older than 24 hours.
func NewDBStats() *DBStats {
	db := &DBStats{
		stopChan: make(chan bool),
	}
	go db.runCleanupWorker()
	return db
}

// Close stops the background cleanup worker.
func (db *DBStats) Close() {
	close(db.stopChan)
}

// runCleanupWorker removes old stats every 10 seconds without locking
// each increment operation.
func (db *DBStats) runCleanupWorker() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			cutoff := time.Now().UTC().Add(-24 * time.Hour)
			db.minutes.Range(func(key, value any) bool {
				minuteStr := key.(string)
				t, err := time.Parse(time.RFC3339, minuteStr)
				if err != nil {
					return true
				}
				if t.Before(cutoff) {
					db.minutes.Delete(key)
				}
				return true
			})
		case <-db.stopChan:
			return
		}
	}
}

// MarshalJSON produces the JSON structure:
//
//	{
//	  "totalStats": {
//	    "read": ...,
//	    "write": ...,
//	    "begin": ...,
//	    "commit": ...,
//	    "rollback": ...,
//	    "httpRequests": ...
//	  },
//	  "stats": [
//	    {
//	      "minute": "...",
//	      "read": ...,
//	      "write": ...,
//	      "begin": ...,
//	      "commit": ...,
//	      "rollback": ...,
//	      "httpRequests": ...
//	    }
//	  ],
//	  "queuedWrites": ...,
//	  "queuedTransactions": ...,
//	  "queuedHTTPRequests": ...
//	}
func (db *DBStats) MarshalJSON() ([]byte, error) {
	statsPerMinute := []StatsWithMinute{}

	db.minutes.Range(func(key, val any) bool {
		minuteStr := key.(string)
		s := val.(*Stats)
		statsPerMinute = append(statsPerMinute, StatsWithMinute{
			Minute:       minuteStr,
			Read:         atomic.LoadInt64(&s.Read),
			Write:        atomic.LoadInt64(&s.Write),
			Begin:        atomic.LoadInt64(&s.Begin),
			Commit:       atomic.LoadInt64(&s.Commit),
			Rollback:     atomic.LoadInt64(&s.Rollback),
			HTTPRequests: atomic.LoadInt64(&s.HTTPRequests),
		})
		return true
	})

	sort.Slice(statsPerMinute, func(i, j int) bool {
		ti, _ := time.Parse(time.RFC3339, statsPerMinute[i].Minute)
		tj, _ := time.Parse(time.RFC3339, statsPerMinute[j].Minute)
		// Newest first
		return tj.Before(ti)
	})

	output := struct {
		TotalStats         DetailedStats     `json:"totalStats"`
		Stats              []StatsWithMinute `json:"stats"`
		QueuedWrites       int64             `json:"queuedWrites"`
		QueuedTransactions int64             `json:"queuedTransactions"`
		QueuedHTTPRequests int64             `json:"queuedHTTPRequests"`
	}{
		TotalStats: DetailedStats{
			Read:         atomic.LoadInt64(&db.total.Read),
			Write:        atomic.LoadInt64(&db.total.Write),
			Begin:        atomic.LoadInt64(&db.total.Begin),
			Commit:       atomic.LoadInt64(&db.total.Commit),
			Rollback:     atomic.LoadInt64(&db.total.Rollback),
			HTTPRequests: atomic.LoadInt64(&db.total.HTTPRequests),
		},
		Stats:              statsPerMinute,
		QueuedWrites:       atomic.LoadInt64(&db.queuedWrites),
		QueuedTransactions: atomic.LoadInt64(&db.queuedTransactions),
		QueuedHTTPRequests: atomic.LoadInt64(&db.queuedHTTPRequests),
	}

	return json.Marshal(output)
}

// getMinuteStats returns a *Stats for the current minute (UTC, truncated).
// If it doesn't exist, a new one is stored.
func (db *DBStats) getMinuteStats() *Stats {
	minuteKey := time.Now().UTC().Truncate(time.Minute).Format(time.RFC3339)
	val, ok := db.minutes.Load(minuteKey)
	if !ok {
		statsPtr := &Stats{}
		actual, loaded := db.minutes.LoadOrStore(minuteKey, statsPtr)
		if loaded {
			return actual.(*Stats)
		}
		return statsPtr
	}
	return val.(*Stats)
}

// IncReads increments read queries atomically.
func (db *DBStats) IncReads() {
	s := db.getMinuteStats()
	atomic.AddInt64(&s.Read, 1)
	atomic.AddInt64(&db.total.Read, 1)
}

// IncWrites increments write queries atomically.
func (db *DBStats) IncWrites() {
	s := db.getMinuteStats()
	atomic.AddInt64(&s.Write, 1)
	atomic.AddInt64(&db.total.Write, 1)
}

// IncBegins increments begin queries atomically.
func (db *DBStats) IncBegins() {
	s := db.getMinuteStats()
	atomic.AddInt64(&s.Begin, 1)
	atomic.AddInt64(&db.total.Begin, 1)
}

// IncCommits increments commit queries atomically.
func (db *DBStats) IncCommits() {
	s := db.getMinuteStats()
	atomic.AddInt64(&s.Commit, 1)
	atomic.AddInt64(&db.total.Commit, 1)
}

// IncRollbacks increments rollback queries atomically.
func (db *DBStats) IncRollbacks() {
	s := db.getMinuteStats()
	atomic.AddInt64(&s.Rollback, 1)
	atomic.AddInt64(&db.total.Rollback, 1)
}

// IncHTTPRequests increments HTTP requests atomically.
func (db *DBStats) IncHTTPRequests() {
	s := db.getMinuteStats()
	atomic.AddInt64(&s.HTTPRequests, 1)
	atomic.AddInt64(&db.total.HTTPRequests, 1)
}

// IncQueuedWrites increments the queued writes counter atomically.
func (db *DBStats) IncQueuedWrites() {
	atomic.AddInt64(&db.queuedWrites, 1)
}

// DecQueuedWrites decrements the queued writes counter atomically.
func (db *DBStats) DecQueuedWrites() {
	for {
		old := atomic.LoadInt64(&db.queuedWrites)
		if old <= 0 {
			return
		}
		if atomic.CompareAndSwapInt64(&db.queuedWrites, old, old-1) {
			return
		}
	}
}

// IncQueuedTransactions increments the queued transactions counter atomically.
func (db *DBStats) IncQueuedTransactions() {
	atomic.AddInt64(&db.queuedTransactions, 1)
}

// DecQueuedTransactions decrements the queued transactions counter atomically.
func (db *DBStats) DecQueuedTransactions() {
	for {
		old := atomic.LoadInt64(&db.queuedTransactions)
		if old <= 0 {
			return
		}
		if atomic.CompareAndSwapInt64(&db.queuedTransactions, old, old-1) {
			return
		}
	}
}

// IncQueuedHTTPRequests increments the queued HTTP requests counter atomically.
func (db *DBStats) IncQueuedHTTPRequests() {
	atomic.AddInt64(&db.queuedHTTPRequests, 1)
}

// DecQueuedHTTPRequests decrements the queued HTTP requests counter atomically.
func (db *DBStats) DecQueuedHTTPRequests() {
	for {
		old := atomic.LoadInt64(&db.queuedHTTPRequests)
		if old <= 0 {
			return
		}
		if atomic.CompareAndSwapInt64(&db.queuedHTTPRequests, old, old-1) {
			return
		}
	}
}
