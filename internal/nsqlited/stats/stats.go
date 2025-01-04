package stats

import (
	"encoding/json"
	"sort"
	"sync"
	"time"
)

// Stat holds counters for different query types.
type Stat struct {
	All      int64
	Read     int64
	Write    int64
	Begin    int64
	Commit   int64
	Rollback int64
}

// DetailedStats is used for the JSON representation of each Stat.
type DetailedStats struct {
	All      int64 `json:"all"`
	Read     int64 `json:"read"`
	Write    int64 `json:"write"`
	Begin    int64 `json:"begin"`
	Commit   int64 `json:"commit"`
	Rollback int64 `json:"rollback"`
}

// StatsWithMinute links a specific minute (RFC3339) with its stats.
type StatsWithMinute struct {
	Minute   string `json:"minute"`
	All      int64  `json:"all"`
	Read     int64  `json:"read"`
	Write    int64  `json:"write"`
	Begin    int64  `json:"begin"`
	Commit   int64  `json:"commit"`
	Rollback int64  `json:"rollback"`
}

// DBStats manages per-minute and total stats, plus queued operations.
// A background cleanup removes stats older than 24h at a fixed interval.
type DBStats struct {
	mu sync.Mutex

	stats      map[string]Stat
	totalStats Stat

	queuedWrites       int64
	queuedTransactions int64

	stopCleanupChan chan bool
}

// NewDBStats creates a DBStats instance and starts a background cleanup.
// The cleanup runs every 10s to remove data older than 24 hours.
func NewDBStats() *DBStats {
	db := &DBStats{
		stats:           make(map[string]Stat),
		stopCleanupChan: make(chan bool),
	}
	go db.runCleanupWorker()
	return db
}

// Close stops the background cleanup worker.
func (db *DBStats) Close() {
	close(db.stopCleanupChan)
}

// runCleanupWorker periodically removes stats older than 24 hours.
func (db *DBStats) runCleanupWorker() {
	ticker := time.NewTicker(time.Second * 10)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			db.mu.Lock()
			db.cleanupOldStats()
			db.mu.Unlock()
		case <-db.stopCleanupChan:
			return
		}
	}
}

// getTimeKey returns the current minute in RFC3339 (UTC).
func (db *DBStats) getTimeKey() string {
	now := time.Now().UTC().Truncate(time.Minute)
	return now.Format(time.RFC3339)
}

// addToStats updates the stats for the current minute and totals.
func (db *DBStats) addToStats(updateFunc func(*Stat)) {
	db.mu.Lock()
	defer db.mu.Unlock()

	key := db.getTimeKey()
	current := db.stats[key]
	updateFunc(&current)
	db.stats[key] = current

	updateFunc(&db.totalStats)
}

// cleanupOldStats removes entries older than 24 hours.
func (db *DBStats) cleanupOldStats() {
	cutoff := time.Now().UTC().Add(-24 * time.Hour)
	for minuteStr := range db.stats {
		parsed, err := time.Parse(time.RFC3339, minuteStr)
		if err != nil {
			continue
		}
		if parsed.Before(cutoff) {
			delete(db.stats, minuteStr)
		}
	}
}

// MarshalJSON produces the JSON structure:
//
//	{
//	  "totalStats": {
//	    "all": ...,
//	    "read": ...,
//	    "write": ...,
//	    "begin": ...,
//	    "commit": ...,
//	    "rollback": ...
//	  },
//	  "stats": [
//	    {
//	      "minute": "...",
//	      "all": ...,
//	      "read": ...,
//	      "write": ...,
//	      "begin": ...,
//	      "commit": ...,
//	      "rollback": ...
//	    }
//	  ],
//	  "queuedWrites": ...,
//	  "queuedTransactions": ...
//	}
func (db *DBStats) MarshalJSON() ([]byte, error) {
	db.mu.Lock()
	defer db.mu.Unlock()

	statsArray := []StatsWithMinute{}
	for minuteStr, st := range db.stats {
		statsArray = append(statsArray, StatsWithMinute{
			Minute:   minuteStr,
			All:      st.All,
			Read:     st.Read,
			Write:    st.Write,
			Begin:    st.Begin,
			Commit:   st.Commit,
			Rollback: st.Rollback,
		})
	}

	sort.Slice(statsArray, func(i, j int) bool {
		ti, _ := time.Parse(time.RFC3339, statsArray[i].Minute)
		tj, _ := time.Parse(time.RFC3339, statsArray[j].Minute)
		return tj.Before(ti) // Newest first
	})

	output := struct {
		TotalStats         DetailedStats     `json:"totalStats"`
		Stats              []StatsWithMinute `json:"stats"`
		QueuedWrites       int64             `json:"queuedWrites"`
		QueuedTransactions int64             `json:"queuedTransactions"`
	}{
		TotalStats: DetailedStats{
			All:      db.totalStats.All,
			Read:     db.totalStats.Read,
			Write:    db.totalStats.Write,
			Begin:    db.totalStats.Begin,
			Commit:   db.totalStats.Commit,
			Rollback: db.totalStats.Rollback,
		},
		Stats:              statsArray,
		QueuedWrites:       db.queuedWrites,
		QueuedTransactions: db.queuedTransactions,
	}

	return json.Marshal(output)
}

// IncReads increments the count for read queries.
func (db *DBStats) IncReads() {
	db.addToStats(func(s *Stat) {
		s.Read++
		s.All++
	})
}

// IncWrites increments the count for write queries.
func (db *DBStats) IncWrites() {
	db.addToStats(func(s *Stat) {
		s.Write++
		s.All++
	})
}

// IncBegins increments the count for begin queries.
func (db *DBStats) IncBegins() {
	db.addToStats(func(s *Stat) {
		s.Begin++
		s.All++
	})
}

// IncCommits increments the count for commit queries.
func (db *DBStats) IncCommits() {
	db.addToStats(func(s *Stat) {
		s.Commit++
		s.All++
	})
}

// IncRollbacks increments the count for rollback queries.
func (db *DBStats) IncRollbacks() {
	db.addToStats(func(s *Stat) {
		s.Rollback++
		s.All++
	})
}

// IncQueuedWrites increments the number of queued write queries.
func (db *DBStats) IncQueuedWrites() {
	db.mu.Lock()
	defer db.mu.Unlock()
	db.queuedWrites++
}

// DecQueuedWrites decrements the number of queued write queries.
func (db *DBStats) DecQueuedWrites() {
	db.mu.Lock()
	defer db.mu.Unlock()
	if db.queuedWrites > 0 {
		db.queuedWrites--
	}
}

// IncQueuedTransactions increments the number of queued transactions.
func (db *DBStats) IncQueuedTransactions() {
	db.mu.Lock()
	defer db.mu.Unlock()
	db.queuedTransactions++
}

// DecQueuedTransactions decrements the number of queued transactions.
func (db *DBStats) DecQueuedTransactions() {
	db.mu.Lock()
	defer db.mu.Unlock()
	if db.queuedTransactions > 0 {
		db.queuedTransactions--
	}
}
