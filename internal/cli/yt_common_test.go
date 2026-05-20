// Copyright 2026 Ryan Collins. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"testing"
)

func TestDecodeReportRows(t *testing.T) {
	tests := []struct {
		name        string
		raw         string
		wantHeaders []string
		wantRows    int
		checkFirst  map[string]any
	}{
		{
			name: "column-array zip with dimension",
			raw: `{"kind":"youtubeAnalytics#resultTable",
				"columnHeaders":[{"name":"day"},{"name":"views"}],
				"rows":[["2026-05-01",12345],["2026-05-02",13002]]}`,
			wantHeaders: []string{"day", "views"},
			wantRows:    2,
			checkFirst:  map[string]any{"day": "2026-05-01", "views": float64(12345)},
		},
		{
			name:        "missing rows yields empty slice",
			raw:         `{"columnHeaders":[{"name":"views"}]}`,
			wantHeaders: []string{"views"},
			wantRows:    0,
		},
		{
			name:        "single total row",
			raw:         `{"columnHeaders":[{"name":"views"},{"name":"likes"}],"rows":[[500,42]]}`,
			wantHeaders: []string{"views", "likes"},
			wantRows:    1,
			checkFirst:  map[string]any{"views": float64(500), "likes": float64(42)},
		},
		{
			name:        "empty rows array",
			raw:         `{"columnHeaders":[{"name":"views"}],"rows":[]}`,
			wantHeaders: []string{"views"},
			wantRows:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			headers, rows, err := decodeReportRows(json.RawMessage(tt.raw))
			if err != nil {
				t.Fatalf("decodeReportRows returned error: %v", err)
			}
			if len(headers) != len(tt.wantHeaders) {
				t.Fatalf("headers = %v, want %v", headers, tt.wantHeaders)
			}
			for i, h := range tt.wantHeaders {
				if headers[i] != h {
					t.Errorf("headers[%d] = %q, want %q", i, headers[i], h)
				}
			}
			if len(rows) != tt.wantRows {
				t.Fatalf("got %d rows, want %d", len(rows), tt.wantRows)
			}
			if tt.checkFirst != nil {
				for k, want := range tt.checkFirst {
					if got := rows[0][k]; got != want {
						t.Errorf("rows[0][%q] = %v (%T), want %v (%T)", k, got, got, want, want)
					}
				}
			}
		})
	}
}

func TestDecodeReportRowsInvalidJSON(t *testing.T) {
	if _, _, err := decodeReportRows(json.RawMessage(`not json`)); err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func TestResolveDateRange(t *testing.T) {
	tests := []struct {
		name      string
		last      string
		period    string
		start     string
		end       string
		partial   bool
		wantStart string
		wantEnd   string
		wantErr   bool
	}{
		{
			name:      "explicit start and end override everything",
			last:      "7d",
			start:     "2026-01-01",
			end:       "2026-01-31",
			wantStart: "2026-01-01",
			wantEnd:   "2026-01-31",
		},
		{
			name:      "last 7d with explicit end is inclusive 7-day window",
			last:      "7d",
			end:       "2026-05-10",
			wantStart: "2026-05-04",
			wantEnd:   "2026-05-10",
		},
		{
			name:      "last 2w resolves to 14 days",
			last:      "2w",
			end:       "2026-05-14",
			wantStart: "2026-05-01",
			wantEnd:   "2026-05-14",
		},
		{
			name:      "last 1m resolves to 30 days",
			last:      "1m",
			end:       "2026-05-30",
			wantStart: "2026-05-01",
			wantEnd:   "2026-05-30",
		},
		{
			name:      "period last-week resolves to an inclusive 7-day window",
			period:    "last-week",
			end:       "2026-05-14",
			wantStart: "2026-05-08",
			wantEnd:   "2026-05-14",
		},
		{
			name:      "period prev-week alias resolves to a 7-day window",
			period:    "prev-week",
			end:       "2026-05-14",
			wantStart: "2026-05-08",
			wantEnd:   "2026-05-14",
		},
		{
			name:      "period last-month is full prior calendar month",
			period:    "last-month",
			end:       "2026-05-10",
			wantStart: "2026-04-01",
			wantEnd:   "2026-04-30",
		},
		{
			name:      "period this-month starts on the 1st",
			period:    "this-month",
			end:       "2026-05-10",
			wantStart: "2026-05-01",
			wantEnd:   "2026-05-10",
		},
		{
			name:      "period ytd starts Jan 1",
			period:    "ytd",
			end:       "2026-03-15",
			wantStart: "2026-01-01",
			wantEnd:   "2026-03-15",
		},
		{
			name:      "period last-28d",
			period:    "last-28d",
			end:       "2026-05-28",
			wantStart: "2026-05-01",
			wantEnd:   "2026-05-28",
		},
		{
			name:    "invalid start date shape errors",
			start:   "2026/01/01",
			wantErr: true,
		},
		{
			name:    "invalid last unit errors",
			last:    "7y",
			wantErr: true,
		},
		{
			name:    "unknown period errors",
			period:  "next-week",
			wantErr: true,
		},
		{
			name:    "start after end errors",
			start:   "2026-05-10",
			end:     "2026-05-01",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start, end, err := resolveDateRange(tt.last, tt.period, tt.start, tt.end, tt.partial)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got start=%q end=%q", start, end)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if start != tt.wantStart {
				t.Errorf("start = %q, want %q", start, tt.wantStart)
			}
			if end != tt.wantEnd {
				t.Errorf("end = %q, want %q", end, tt.wantEnd)
			}
		})
	}
}

func TestResolveDateRangeDefaultIs28Days(t *testing.T) {
	start, end, err := resolveDateRange("", "", "", "2026-05-28", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if end != "2026-05-28" {
		t.Errorf("end = %q, want 2026-05-28", end)
	}
	if start != "2026-05-01" {
		t.Errorf("start = %q, want 2026-05-01 (28-day default window)", start)
	}
}

func TestPriorPeriod(t *testing.T) {
	tests := []struct {
		name       string
		start      string
		end        string
		wantPStart string
		wantPEnd   string
	}{
		{
			name:       "7-day window",
			start:      "2026-05-08",
			end:        "2026-05-14",
			wantPStart: "2026-05-01",
			wantPEnd:   "2026-05-07",
		},
		{
			name:       "single day window",
			start:      "2026-05-10",
			end:        "2026-05-10",
			wantPStart: "2026-05-09",
			wantPEnd:   "2026-05-09",
		},
		{
			name:       "month-long window spanning a boundary",
			start:      "2026-03-01",
			end:        "2026-03-31",
			wantPStart: "2026-01-29",
			wantPEnd:   "2026-02-28",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ps, pe := priorPeriod(tt.start, tt.end)
			if ps != tt.wantPStart {
				t.Errorf("pStart = %q, want %q", ps, tt.wantPStart)
			}
			if pe != tt.wantPEnd {
				t.Errorf("pEnd = %q, want %q", pe, tt.wantPEnd)
			}
		})
	}
}

func TestPriorPeriodInvalidInput(t *testing.T) {
	ps, pe := priorPeriod("bad", "2026-05-14")
	if ps != "" || pe != "" {
		t.Errorf("expected empty strings for invalid input, got %q %q", ps, pe)
	}
}
