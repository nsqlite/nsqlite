package stats

import (
	"sort"
	"time"
)

type LoadedStats struct {
	StartedAt          string `json:"startedAt"`
	Uptime             string `json:"uptime"`
	QueuedWrites       int64  `json:"queuedWrites"`
	QueuedTransactions int64  `json:"queuedTransactions"`
	QueuedHTTPRequests int64  `json:"queuedHttpRequests"`
	Totals             Totals `json:"totals"`
	Stats              []Stat `json:"stats"`
}

type Totals struct {
	Reads        int64 `json:"reads"`
	Writes       int64 `json:"writes"`
	Begins       int64 `json:"begins"`
	Commits      int64 `json:"commits"`
	Rollbacks    int64 `json:"rollbacks"`
	HTTPRequests int64 `json:"httpRequests"`
}

type Stat struct {
	Minute       string `json:"minute"`
	Reads        int64  `json:"reads"`
	Writes       int64  `json:"writes"`
	Begins       int64  `json:"begins"`
	Commits      int64  `json:"commits"`
	Rollbacks    int64  `json:"rollbacks"`
	HTTPRequests int64  `json:"httpRequests"`
}

// LoadStats loads all internal stats into a LoadedStats struct.
func (db *DBStats) LoadStats() LoadedStats {
	var (
		allStats          []Stat = []Stat{}
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

		totalReads += r
		totalWrites += w
		totalBegins += b
		totalCommits += c
		totalRollbacks += rb
		totalHTTPRequests += hr

		allStats = append(allStats, Stat{
			Minute:       minuteKey,
			Reads:        r,
			Writes:       w,
			Begins:       b,
			Commits:      c,
			Rollbacks:    rb,
			HTTPRequests: hr,
		})

		return true
	})

	sort.Slice(allStats, func(i, j int) bool {
		ti, _ := time.Parse(time.RFC3339, allStats[i].Minute)
		tj, _ := time.Parse(time.RFC3339, allStats[j].Minute)
		return tj.Before(ti)
	})

	return LoadedStats{
		Totals: Totals{
			Reads:        totalReads,
			Writes:       totalWrites,
			Begins:       totalBegins,
			Commits:      totalCommits,
			Rollbacks:    totalRollbacks,
			HTTPRequests: totalHTTPRequests,
		},
		Stats:              allStats,
		QueuedWrites:       db.queuedWrites.Load(),
		QueuedTransactions: db.queuedTransactions.Load(),
		QueuedHTTPRequests: db.queuedHTTPRequests.Load(),
		StartedAt:          db.startedAt.Format(time.RFC3339),
		Uptime:             time.Since(db.startedAt).Round(time.Second).String(),
	}
}
