package stats

import (
	"encoding/json"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// minuteData holds the counters for a specific minute (atomic for thread safety).
type minuteData struct {
	reads        atomic.Int64
	writes       atomic.Int64
	begins       atomic.Int64
	commits      atomic.Int64
	rollbacks    atomic.Int64
	httpRequests atomic.Int64
}

// DBStats holds the stats for the database.
type DBStats struct {
	minutes sync.Map // key: string (minute RFC3339) -> value: *minuteData

	queuedWrites       atomic.Int64
	queuedTransactions atomic.Int64
	queuedHTTPRequests atomic.Int64

	stopChan chan bool
}

// NewDBStats creates a DBStats instance.
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

// runCleanupWorker removes stats older than 24 hours every 10 seconds.
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

// getOrCreateMinuteData returns a *minuteData for the current minute (UTC).
// If none exists, it creates one.
func (db *DBStats) getOrCreateMinuteData() *minuteData {
	minuteKey := time.Now().UTC().Truncate(time.Minute).Format(time.RFC3339)
	val, ok := db.minutes.Load(minuteKey)
	if !ok {
		md := &minuteData{}
		actual, loaded := db.minutes.LoadOrStore(minuteKey, md)
		if loaded {
			return actual.(*minuteData)
		}
		return md
	}
	return val.(*minuteData)
}

// IncReads increments the read counter for the current minute.
func (db *DBStats) IncReads() {
	md := db.getOrCreateMinuteData()
	md.reads.Add(1)
}

// IncWrites increments the write counter for the current minute.
func (db *DBStats) IncWrites() {
	md := db.getOrCreateMinuteData()
	md.writes.Add(1)
}

// IncBegins increments the begin counter for the current minute.
func (db *DBStats) IncBegins() {
	md := db.getOrCreateMinuteData()
	md.begins.Add(1)
}

// IncCommits increments the commit counter for the current minute.
func (db *DBStats) IncCommits() {
	md := db.getOrCreateMinuteData()
	md.commits.Add(1)
}

// IncRollbacks increments the rollback counter for the current minute.
func (db *DBStats) IncRollbacks() {
	md := db.getOrCreateMinuteData()
	md.rollbacks.Add(1)
}

// IncHTTPRequests increments the HTTP requests counter for the current minute.
func (db *DBStats) IncHTTPRequests() {
	md := db.getOrCreateMinuteData()
	md.httpRequests.Add(1)
}

// IncQueuedWrites increments the queued writes counter atomically.
func (db *DBStats) IncQueuedWrites() {
	db.queuedWrites.Add(1)
}

// DecQueuedWrites decrements the queued writes counter atomically.
func (db *DBStats) DecQueuedWrites() {
	db.queuedWrites.Add(-1)
}

// IncQueuedTransactions increments the queued transactions counter atomically.
func (db *DBStats) IncQueuedTransactions() {
	db.queuedTransactions.Add(1)
}

// DecQueuedTransactions decrements the queued transactions counter atomically.
func (db *DBStats) DecQueuedTransactions() {
	db.queuedTransactions.Add(-1)
}

// IncQueuedHTTPRequests increments the queued HTTP requests counter atomically.
func (db *DBStats) IncQueuedHTTPRequests() {
	db.queuedHTTPRequests.Add(1)
}

// DecQueuedHTTPRequests decrements the queued HTTP requests counter atomically.
func (db *DBStats) DecQueuedHTTPRequests() {
	db.queuedHTTPRequests.Add(-1)
}

// MarshalJSON produces the JSON structure:
//
//	{
//	  "stats": [
//	    {
//	      "minute": "...",
//	      "reads": ...,
//	      "writes": ...,
//	      "begins": ...,
//	      "commits": ...,
//	      "rollbacks": ...,
//	      "httpRequests": ...
//	    }
//	  ],
//	  "totals": {
//	    "reads": ...,
//	    "writes": ...,
//	    "begins": ...,
//	    "commits": ...,
//	    "rollbacks": ...,
//	    "httpRequests": ...
//	  },
//	  "queuedWrites": ...,
//	  "queuedTransactions": ...,
//	  "queuedHTTPRequests": ...
//	}
func (db *DBStats) MarshalJSON() ([]byte, error) {
	type minuteEntry struct {
		Minutes      string `json:"minutes"`
		Reads        int64  `json:"reads"`
		Writes       int64  `json:"writes"`
		Begins       int64  `json:"begins"`
		Commits      int64  `json:"commits"`
		Rollbacks    int64  `json:"rollbacks"`
		HTTPRequests int64  `json:"httpRequests"`
	}

	var (
		allEntries        []minuteEntry
		totalReads        int64
		totalWrites       int64
		totalBegins       int64
		totalCommits      int64
		totalRollbacks    int64
		totalHTTPRequests int64
	)

	db.minutes.Range(func(key, value any) bool {
		minuteKey := key.(string)
		md := value.(*minuteData)

		r := md.reads.Load()
		w := md.writes.Load()
		b := md.begins.Load()
		c := md.commits.Load()
		rb := md.rollbacks.Load()
		hr := md.httpRequests.Load()

		allEntries = append(allEntries, minuteEntry{
			Minutes:      minuteKey,
			Reads:        r,
			Writes:       w,
			Begins:       b,
			Commits:      c,
			Rollbacks:    rb,
			HTTPRequests: hr,
		})

		totalReads += r
		totalWrites += w
		totalBegins += b
		totalCommits += c
		totalRollbacks += rb
		totalHTTPRequests += hr

		return true
	})

	sort.Slice(allEntries, func(i, j int) bool {
		ti, _ := time.Parse(time.RFC3339, allEntries[i].Minutes)
		tj, _ := time.Parse(time.RFC3339, allEntries[j].Minutes)
		return tj.Before(ti)
	})

	totals := map[string]int64{
		"reads":        totalReads,
		"writes":       totalWrites,
		"begins":       totalBegins,
		"commits":      totalCommits,
		"rollbacks":    totalRollbacks,
		"httpRequests": totalHTTPRequests,
	}

	// Final structure
	output := struct {
		Stats              []minuteEntry    `json:"stats"`
		Totals             map[string]int64 `json:"totals"`
		QueuedWrites       int64            `json:"queuedWrites"`
		QueuedTransactions int64            `json:"queuedTransactions"`
		QueuedHTTPRequests int64            `json:"queuedHTTPRequests"`
	}{
		Stats:              allEntries,
		Totals:             totals,
		QueuedWrites:       db.queuedWrites.Load(),
		QueuedTransactions: db.queuedTransactions.Load(),
		QueuedHTTPRequests: db.queuedHTTPRequests.Load(),
	}

	return json.Marshal(output)
}
