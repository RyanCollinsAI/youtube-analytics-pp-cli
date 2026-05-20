// Copyright 2026 Ryan Collins. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"youtube-analytics-pp-cli/internal/client"
)

// queryRow is a single decoded report row: header name -> value.
type queryRow = map[string]any

// reportParams carries the inputs for a YouTube Analytics reports.query call.
type reportParams struct {
	StartDate  string
	EndDate    string
	Metrics    string
	Dimensions string
	Filters    string
	Sort       string
	MaxResults int
	Currency   string
}

// dateShapeRE validates a YYYY-MM-DD date.
var dateShapeRE = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

// channelIDRE matches a raw YouTube channel id (UC... 20-30 chars).
var channelIDRE = regexp.MustCompile(`^UC[0-9A-Za-z_-]{18,28}$`)

// decodeReportRows parses a column-oriented reports.query response. The API
// returns {columnHeaders:[{name}],rows:[[...]]}; this zips each row array with
// the header names by index. A missing rows field yields an empty slice.
func decodeReportRows(raw json.RawMessage) (headers []string, rows []map[string]any, err error) {
	var resp struct {
		ColumnHeaders []struct {
			Name string `json:"name"`
		} `json:"columnHeaders"`
		Rows [][]any `json:"rows"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, nil, fmt.Errorf("decoding report response: %w", err)
	}
	headers = make([]string, len(resp.ColumnHeaders))
	for i, h := range resp.ColumnHeaders {
		headers[i] = h.Name
	}
	rows = make([]map[string]any, 0, len(resp.Rows))
	for _, r := range resp.Rows {
		m := make(map[string]any, len(headers))
		for i, name := range headers {
			if i < len(r) {
				m[name] = r[i]
			}
		}
		rows = append(rows, m)
	}
	return headers, rows, nil
}

// runReportQuery builds the parameter map for /v2/reports and issues the
// request. Errors are returned raw so callers can wrap with classifyAPIError.
func runReportQuery(c *client.Client, channelID string, p reportParams) (json.RawMessage, error) {
	params := map[string]string{
		"ids":       "channel==" + channelID,
		"startDate": p.StartDate,
		"endDate":   p.EndDate,
		"metrics":   p.Metrics,
	}
	if p.Dimensions != "" {
		params["dimensions"] = p.Dimensions
	}
	if p.Filters != "" {
		params["filters"] = p.Filters
	}
	if p.Sort != "" {
		params["sort"] = p.Sort
	}
	if p.MaxResults > 0 {
		params["maxResults"] = strconv.Itoa(p.MaxResults)
	}
	if p.Currency != "" {
		params["currency"] = p.Currency
	}
	return c.Get("/v2/reports", params)
}

// utcToday returns today's date at UTC midnight.
func utcToday() time.Time {
	n := time.Now().UTC()
	return time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, time.UTC)
}

// resolveDateRange computes a startDate/endDate window from the --last,
// --period, --start, --end flags. Explicit start/end override everything.
// The default end is lag-safe: today-3 days (today-1 when includePartial).
func resolveDateRange(last, period, start, end string, includePartial bool) (startDate, endDate string, err error) {
	if start != "" && !dateShapeRE.MatchString(start) {
		return "", "", usageErr(fmt.Errorf("invalid --start %q: expected YYYY-MM-DD", start))
	}
	if end != "" && !dateShapeRE.MatchString(end) {
		return "", "", usageErr(fmt.Errorf("invalid --end %q: expected YYYY-MM-DD", end))
	}

	lag := 3
	if includePartial {
		lag = 1
	}
	defaultEnd := utcToday().AddDate(0, 0, -lag)

	var e time.Time
	if end != "" {
		e, _ = time.Parse("2006-01-02", end)
	} else {
		e = defaultEnd
	}

	// Explicit start wins; otherwise compute from --last or --period.
	if start != "" {
		s, _ := time.Parse("2006-01-02", start)
		if s.After(e) {
			return "", "", usageErr(fmt.Errorf("--start %s is after --end %s", start, e.Format("2006-01-02")))
		}
		return start, e.Format("2006-01-02"), nil
	}

	if last != "" {
		n, unit, perr := parseLast(last)
		if perr != nil {
			return "", "", usageErr(perr)
		}
		days := n
		switch unit {
		case "w":
			days = n * 7
		case "m":
			days = n * 30
		}
		if days < 1 {
			days = 1
		}
		s := e.AddDate(0, 0, -(days - 1))
		return s.Format("2006-01-02"), e.Format("2006-01-02"), nil
	}

	if period != "" {
		s, pe, perr := resolvePeriod(period, e)
		if perr != nil {
			return "", "", usageErr(perr)
		}
		return s, pe, nil
	}

	// Default: last 28 days.
	s := e.AddDate(0, 0, -27)
	return s.Format("2006-01-02"), e.Format("2006-01-02"), nil
}

// parseLast parses a --last value like "7d", "4w", "3m" into (n, unit).
func parseLast(v string) (int, string, error) {
	v = strings.TrimSpace(strings.ToLower(v))
	if v == "" {
		return 0, "", fmt.Errorf("empty --last value")
	}
	unit := v[len(v)-1:]
	if unit != "d" && unit != "w" && unit != "m" {
		return 0, "", fmt.Errorf("invalid --last %q: use Nd, Nw, or Nm (e.g. 28d)", v)
	}
	n, err := strconv.Atoi(v[:len(v)-1])
	if err != nil || n < 1 {
		return 0, "", fmt.Errorf("invalid --last %q: use Nd, Nw, or Nm (e.g. 28d)", v)
	}
	return n, unit, nil
}

// resolvePeriod resolves a named period against an end date.
func resolvePeriod(period string, end time.Time) (string, string, error) {
	switch strings.ToLower(strings.TrimSpace(period)) {
	case "last-week", "last-7d", "prev-week", "previous-week":
		return end.AddDate(0, 0, -6).Format("2006-01-02"), end.Format("2006-01-02"), nil
	case "last-28d":
		return end.AddDate(0, 0, -27).Format("2006-01-02"), end.Format("2006-01-02"), nil
	case "last-30d":
		return end.AddDate(0, 0, -29).Format("2006-01-02"), end.Format("2006-01-02"), nil
	case "this-month":
		s := time.Date(end.Year(), end.Month(), 1, 0, 0, 0, 0, time.UTC)
		return s.Format("2006-01-02"), end.Format("2006-01-02"), nil
	case "last-month":
		firstThis := time.Date(end.Year(), end.Month(), 1, 0, 0, 0, 0, time.UTC)
		lastEnd := firstThis.AddDate(0, 0, -1)
		lastStart := time.Date(lastEnd.Year(), lastEnd.Month(), 1, 0, 0, 0, 0, time.UTC)
		return lastStart.Format("2006-01-02"), lastEnd.Format("2006-01-02"), nil
	case "ytd":
		s := time.Date(end.Year(), 1, 1, 0, 0, 0, 0, time.UTC)
		return s.Format("2006-01-02"), end.Format("2006-01-02"), nil
	default:
		return "", "", fmt.Errorf("unknown --period %q: use last-week, last-7d, last-28d, last-30d, this-month, last-month, or ytd", period)
	}
}

// priorPeriod returns the immediately-preceding window of equal length.
func priorPeriod(startDate, endDate string) (pStart, pEnd string) {
	s, err1 := time.Parse("2006-01-02", startDate)
	e, err2 := time.Parse("2006-01-02", endDate)
	if err1 != nil || err2 != nil {
		return "", ""
	}
	days := int(e.Sub(s).Hours()/24) + 1
	pe := s.AddDate(0, 0, -1)
	ps := pe.AddDate(0, 0, -(days - 1))
	return ps.Format("2006-01-02"), pe.Format("2006-01-02")
}

// resolveChannel resolves a channel argument to (id, friendlyName). An empty
// argument uses the sole registered channel, or errors when there are zero or
// many. A raw UC... id is returned as-is; a friendly name is looked up in the
// local channels archive table.
func resolveChannel(flags *rootFlags, channelArg string) (id string, name string, err error) {
	if channelIDRE.MatchString(channelArg) {
		return channelArg, channelArg, nil
	}

	dbPath := defaultDBPath("youtube-analytics-pp-cli")
	a, aerr := openArchive(nil, dbPath)
	if aerr != nil {
		return "", "", fmt.Errorf("opening local archive: %w", aerr)
	}
	defer a.close()

	if channelArg == "" {
		chans := a.listChannels()
		switch len(chans) {
		case 0:
			return "", "", usageErr(fmt.Errorf("no channel given and none registered; run 'channels add --name <name> --channel-id UC...' or pass --channel UC..."))
		case 1:
			return chans[0].ChannelID, chans[0].Name, nil
		default:
			names := make([]string, len(chans))
			for i, c := range chans {
				names[i] = c.Name
			}
			return "", "", usageErr(fmt.Errorf("multiple channels registered (%s); pass --channel <name>", strings.Join(names, ", ")))
		}
	}

	cid, ok := a.lookupChannel(channelArg)
	if !ok {
		return "", "", notFoundErr(fmt.Errorf("channel %q not found; run 'channels add' or pass a UC... id", channelArg))
	}
	return cid, channelArg, nil
}
