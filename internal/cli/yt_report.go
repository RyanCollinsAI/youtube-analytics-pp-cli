// Copyright 2026 ryandacoder. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// metricDelta is one headline metric with its current and prior values.
type metricDelta struct {
	Metric    string  `json:"metric"`
	Current   float64 `json:"current"`
	Prior     float64 `json:"prior"`
	Delta     float64 `json:"delta"`
	PctChange float64 `json:"pct_change"`
}

// reportHeadlineMetrics are the metrics summed for the channel report card.
var reportHeadlineMetrics = []string{
	"views",
	"estimatedMinutesWatched",
	"subscribersGained",
	"subscribersLost",
	"estimatedRevenue",
}

// pctChange returns the percent change from prior to current. A zero prior
// with a non-zero current is reported as 100%.
func pctChange(current, prior float64) float64 {
	if prior == 0 {
		if current == 0 {
			return 0
		}
		return 100
	}
	return (current - prior) / prior * 100
}

func newReportCmd(flags *rootFlags) *cobra.Command {
	var channel, last, period, start, end string
	var includePartial bool

	cmd := &cobra.Command{
		Use:         "report",
		Short:       "Channel report card: headline metrics with period-over-period deltas",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: strings.Trim(`
  youtube-analytics-pp-cli report --channel ScrollCore --period last-week
  youtube-analytics-pp-cli report --channel ScrollCore --last 28d --json`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			startDate, endDate, err := resolveDateRange(last, period, start, end, includePartial)
			if err != nil {
				return err
			}
			if dryRunOK(flags) {
				return nil
			}

			channelID, name, err := resolveChannel(flags, channel)
			if err != nil {
				return err
			}

			a, err := openArchive(cmd.Context(), defaultDBPath("youtube-analytics-pp-cli"))
			if err != nil {
				return fmt.Errorf("opening local archive: %w", err)
			}
			defer a.close()

			if a.channelRowCount(channelID) == 0 {
				return fmt.Errorf("no archived rows for channel %q; run 'sync' first", name)
			}

			pStart, pEnd := priorPeriod(startDate, endDate)

			deltas := make([]metricDelta, 0, len(reportHeadlineMetrics))
			for _, m := range reportHeadlineMetrics {
				cur, _ := a.sumMetric(channelID, m, startDate, endDate)
				pri, _ := a.sumMetric(channelID, m, pStart, pEnd)
				deltas = append(deltas, metricDelta{
					Metric:    m,
					Current:   cur,
					Prior:     pri,
					Delta:     cur - pri,
					PctChange: pctChange(cur, pri),
				})
			}

			out := map[string]any{
				"channel":      name,
				"channel_id":   channelID,
				"period":       map[string]string{"start": startDate, "end": endDate},
				"prior_period": map[string]string{"start": pStart, "end": pEnd},
				"metrics":      deltas,
			}

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "Report for %s  (%s to %s)\n\n", name, startDate, endDate)
				tw := newTabWriter(cmd.OutOrStdout())
				fmt.Fprintln(tw, bold("METRIC")+"\t"+bold("CURRENT")+"\t"+bold("PRIOR")+"\t"+bold("DELTA")+"\t"+bold("CHANGE"))
				for _, d := range deltas {
					fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%+.1f%%\n",
						d.Metric, fmtNum(d.Current), fmtNum(d.Prior), fmtSignedNum(d.Delta), d.PctChange)
				}
				return tw.Flush()
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}

	cmd.Flags().StringVar(&channel, "channel", "", "Channel name or UC... id")
	cmd.Flags().StringVar(&last, "last", "", "Trailing window, e.g. 7d, 4w, 3m")
	cmd.Flags().StringVar(&period, "period", "", "Named period: last-week, last-7d, last-28d, last-30d, this-month, last-month, ytd")
	cmd.Flags().StringVar(&start, "start", "", "Explicit start date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&end, "end", "", "Explicit end date (YYYY-MM-DD)")
	cmd.Flags().BoolVar(&includePartial, "include-partial", false, "Include the most recent (possibly incomplete) days")

	return cmd
}

// fmtNum renders a float without trailing decimals when it is whole.
func fmtNum(v float64) string {
	if v == float64(int64(v)) {
		return fmt.Sprintf("%d", int64(v))
	}
	return fmt.Sprintf("%.2f", v)
}

// fmtSignedNum renders a delta with an explicit sign.
func fmtSignedNum(v float64) string {
	if v == float64(int64(v)) {
		return fmt.Sprintf("%+d", int64(v))
	}
	return fmt.Sprintf("%+.2f", v)
}
