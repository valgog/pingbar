// Package store keeps ping samples and outages in a local SQLite file so
// history survives restarts and can be shared between the menu-bar process
// and any number of terminal viewers.
package store

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/valgog/pingbar/internal/stats"
	_ "modernc.org/sqlite"
)

const schema = `
CREATE TABLE IF NOT EXISTS samples (
	ts     INTEGER NOT NULL, -- unix milliseconds
	target TEXT    NOT NULL,
	rtt_us INTEGER           -- NULL when the ping was lost
);
CREATE INDEX IF NOT EXISTS samples_ts ON samples(ts);
CREATE TABLE IF NOT EXISTS outages (
	id       INTEGER PRIMARY KEY,
	target   TEXT    NOT NULL,
	start_ts INTEGER NOT NULL,
	end_ts   INTEGER          -- NULL while ongoing
);
CREATE INDEX IF NOT EXISTS outages_start ON outages(start_ts);
`

type Store struct{ db *sql.DB }

// Open opens (creating if needed) the database at path.
func Open(path string) (*Store, error) {
	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("init schema: %w", err)
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error { return s.db.Close() }

func ms(t time.Time) int64 { return t.UnixMilli() }

// Insert stores samples.
func (s *Store) Insert(samples ...stats.Sample) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	st, err := tx.Prepare(`INSERT INTO samples(ts, target, rtt_us) VALUES (?, ?, ?)`)
	if err != nil {
		return err
	}
	defer st.Close()
	for _, x := range samples {
		var rtt any
		if x.OK {
			rtt = x.RTT.Microseconds()
		}
		if _, err := st.Exec(ms(x.TS), x.Target, rtt); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// Since returns samples of target taken at or after t, oldest first.
func (s *Store) Since(target string, t time.Time) ([]stats.Sample, error) {
	rows, err := s.db.Query(`SELECT ts, rtt_us FROM samples WHERE target = ? AND ts >= ? ORDER BY ts`, target, ms(t))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []stats.Sample
	for rows.Next() {
		var ts int64
		var rtt sql.NullInt64
		if err := rows.Scan(&ts, &rtt); err != nil {
			return nil, err
		}
		x := stats.Sample{TS: time.UnixMilli(ts), Target: target, OK: rtt.Valid}
		if rtt.Valid {
			x.RTT = time.Duration(rtt.Int64) * time.Microsecond
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

// Buckets aggregates samples of target into n consecutive buckets of the given
// width starting at start. Buckets with no samples have Sent == 0.
func (s *Store) Buckets(target string, start time.Time, width time.Duration, n int) ([]stats.Bucket, error) {
	out := make([]stats.Bucket, n)
	for i := range out {
		out[i].Start = start.Add(time.Duration(i) * width)
	}
	w := width.Milliseconds()
	if w <= 0 {
		return nil, fmt.Errorf("bucket width too small: %v", width)
	}
	end := start.Add(time.Duration(n) * width)
	rows, err := s.db.Query(`
		SELECT (ts - ?) / ? AS b, COUNT(*), COUNT(rtt_us), COALESCE(AVG(rtt_us), 0)
		FROM samples WHERE target = ? AND ts >= ? AND ts < ?
		GROUP BY b`, ms(start), w, target, ms(start), ms(end))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var b int64
		var sent, recv int
		var avg float64
		if err := rows.Scan(&b, &sent, &recv, &avg); err != nil {
			return nil, err
		}
		if b < 0 || b >= int64(n) {
			continue
		}
		out[b].Sent, out[b].Recv = sent, recv
		out[b].AvgRTT = time.Duration(avg) * time.Microsecond
	}
	return out, rows.Err()
}

// StartOutage records the beginning of an outage and returns its id.
func (s *Store) StartOutage(target string, start time.Time) (int64, error) {
	res, err := s.db.Exec(`INSERT INTO outages(target, start_ts) VALUES (?, ?)`, target, ms(start))
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// EndOutage marks an outage as finished.
func (s *Store) EndOutage(id int64, end time.Time) error {
	_, err := s.db.Exec(`UPDATE outages SET end_ts = ? WHERE id = ?`, ms(end), id)
	return err
}

// CloseDangling ends outages left open by a process that died, using the time
// of the last sample stored for their target (or their start if none).
func (s *Store) CloseDangling() error {
	_, err := s.db.Exec(`
		UPDATE outages SET end_ts = COALESCE(
			(SELECT MAX(ts) FROM samples WHERE samples.target = outages.target AND samples.ts >= outages.start_ts),
			start_ts)
		WHERE end_ts IS NULL`)
	return err
}

// Outages returns up to limit outages that started at or after since, newest first.
func (s *Store) Outages(since time.Time, limit int) ([]stats.Outage, error) {
	rows, err := s.db.Query(`SELECT id, target, start_ts, end_ts FROM outages WHERE start_ts >= ? ORDER BY start_ts DESC LIMIT ?`, ms(since), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []stats.Outage
	for rows.Next() {
		var o stats.Outage
		var start int64
		var end sql.NullInt64
		if err := rows.Scan(&o.ID, &o.Target, &start, &end); err != nil {
			return nil, err
		}
		o.Start = time.UnixMilli(start)
		if end.Valid {
			o.End = time.UnixMilli(end.Int64)
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

// Prune deletes samples older than before. Outages are kept forever; they are tiny.
func (s *Store) Prune(before time.Time) error {
	_, err := s.db.Exec(`DELETE FROM samples WHERE ts < ?`, ms(before))
	return err
}
