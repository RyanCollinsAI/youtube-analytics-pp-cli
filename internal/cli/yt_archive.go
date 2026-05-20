// Copyright 2026 ryandacoder. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"database/sql"

	"youtube-analytics-pp-cli/internal/store"
)

// archive is the local report-row archive. It shares the same SQLite file as
// the generated store and adds its own channels + report_rows tables.
type archive struct {
	st *store.Store
}

// channelRow is one row of the channels registry table.
type channelRow struct {
	Name      string `json:"name"`
	ChannelID string `json:"channel_id"`
	AddedAt   string `json:"added_at"`
}

// dayValue is one (day, value) pair from a metric series.
type dayValue struct {
	Day   string  `json:"day"`
	Value float64 `json:"value"`
}

const archiveSchema = `
CREATE TABLE IF NOT EXISTS channels (
    name TEXT PRIMARY KEY,
    channel_id TEXT NOT NULL,
    added_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS report_rows (
    channel TEXT NOT NULL,
    channel_id TEXT NOT NULL,
    day TEXT NOT NULL,
    metric TEXT NOT NULL,
    value REAL NOT NULL,
    synced_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (channel_id, day, metric)
);
CREATE INDEX IF NOT EXISTS idx_report_rows_day ON report_rows(day);
`

// openArchive opens the local store at dbPath and ensures the archive tables
// exist. A nil ctx is treated as context.Background().
func openArchive(ctx context.Context, dbPath string) (*archive, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	st, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		return nil, err
	}
	if _, err := st.DB().ExecContext(ctx, archiveSchema); err != nil {
		st.Close()
		return nil, err
	}
	return &archive{st: st}, nil
}

func (a *archive) close() error { return a.st.Close() }

func (a *archive) db() *sql.DB { return a.st.DB() }

// upsertReportRow inserts or overwrites a single (channel, day, metric) cell.
func (a *archive) upsertReportRow(channel, channelID, day, metric string, value float64) error {
	_, err := a.st.DB().Exec(
		`INSERT INTO report_rows (channel, channel_id, day, metric, value, synced_at)
		 VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		 ON CONFLICT(channel_id, day, metric)
		 DO UPDATE SET value = excluded.value, channel = excluded.channel, synced_at = CURRENT_TIMESTAMP`,
		channel, channelID, day, metric, value,
	)
	return err
}

// reportCell is one cell for bulk insertion.
type reportCell struct {
	Day    string
	Metric string
	Value  float64
}

// bulkUpsertReportRows writes many cells for one channel in a single transaction.
func (a *archive) bulkUpsertReportRows(channel, channelID string, cells []reportCell) error {
	tx, err := a.st.DB().Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmt, err := tx.Prepare(
		`INSERT INTO report_rows (channel, channel_id, day, metric, value, synced_at)
		 VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		 ON CONFLICT(channel_id, day, metric)
		 DO UPDATE SET value = excluded.value, channel = excluded.channel, synced_at = CURRENT_TIMESTAMP`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, c := range cells {
		if _, err := stmt.Exec(channel, channelID, c.Day, c.Metric, c.Value); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// addChannel registers a friendly name -> channel id mapping.
func (a *archive) addChannel(name, id string) error {
	_, err := a.st.DB().Exec(
		`INSERT INTO channels (name, channel_id, added_at)
		 VALUES (?, ?, CURRENT_TIMESTAMP)
		 ON CONFLICT(name) DO UPDATE SET channel_id = excluded.channel_id`,
		name, id,
	)
	return err
}

// listChannels returns all registered channels ordered by name.
func (a *archive) listChannels() []channelRow {
	rows, err := a.st.DB().Query(`SELECT name, channel_id, COALESCE(added_at, '') FROM channels ORDER BY name`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []channelRow
	for rows.Next() {
		var c channelRow
		if err := rows.Scan(&c.Name, &c.ChannelID, &c.AddedAt); err == nil {
			out = append(out, c)
		}
	}
	return out
}

// removeChannel deletes a registered channel. The bool reports whether a row
// was actually removed.
func (a *archive) removeChannel(name string) (bool, error) {
	res, err := a.st.DB().Exec(`DELETE FROM channels WHERE name = ?`, name)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// lookupChannel resolves a friendly name (case-insensitive) to a channel id.
func (a *archive) lookupChannel(name string) (string, bool) {
	var id string
	err := a.st.DB().QueryRow(
		`SELECT channel_id FROM channels WHERE LOWER(name) = LOWER(?)`, name,
	).Scan(&id)
	if err != nil {
		return "", false
	}
	return id, true
}

// channelCount returns the number of registered channels.
func (a *archive) channelCount() int {
	var n int
	a.st.DB().QueryRow(`SELECT COUNT(*) FROM channels`).Scan(&n)
	return n
}

// sumMetric returns SUM(value) for a metric over a day range. ok is false when
// no rows exist.
func (a *archive) sumMetric(channelID, metric, startDate, endDate string) (float64, bool) {
	var sum sql.NullFloat64
	var cnt int
	err := a.st.DB().QueryRow(
		`SELECT SUM(value), COUNT(*) FROM report_rows
		 WHERE channel_id = ? AND metric = ? AND day >= ? AND day <= ?`,
		channelID, metric, startDate, endDate,
	).Scan(&sum, &cnt)
	if err != nil || cnt == 0 {
		return 0, false
	}
	return sum.Float64, true
}

// dailySeries returns the per-day values of a metric over a day range,
// ordered ascending by day.
func (a *archive) dailySeries(channelID, metric, startDate, endDate string) []dayValue {
	rows, err := a.st.DB().Query(
		`SELECT day, value FROM report_rows
		 WHERE channel_id = ? AND metric = ? AND day >= ? AND day <= ?
		 ORDER BY day`,
		channelID, metric, startDate, endDate,
	)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []dayValue
	for rows.Next() {
		var d dayValue
		if err := rows.Scan(&d.Day, &d.Value); err == nil {
			out = append(out, d)
		}
	}
	return out
}

// channelRowCount returns how many report_rows exist for a channel.
func (a *archive) channelRowCount(channelID string) int {
	var n int
	a.st.DB().QueryRow(`SELECT COUNT(*) FROM report_rows WHERE channel_id = ?`, channelID).Scan(&n)
	return n
}
