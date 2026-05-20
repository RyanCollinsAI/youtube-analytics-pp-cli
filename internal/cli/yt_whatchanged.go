// Copyright 2026 ryandacoder. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// whatchangedMetrics is the fixed set of metrics diffed between two windows.
var whatchangedMetrics = []string{
	"views",
	"estimatedMinutesWatched",
	"subscribersGained",
	"subscribersLost",
	"likes",
	"comments",
	"shares",
	"estimatedRevenue",
}

func newWhatChangedCmd(flags *rootFlags) *cobra.Command {
	var channel, period string
	var includePartial bool

	cmd := &cobra.Command{
		Use:         "whatchanged",
		Short:       "Biggest metric movers between the current period and the prior equal window",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: strings.Trim(`
  youtube-analytics-pp-cli whatchanged --channel ScrollCore
  youtube-analytics-pp-cli whatchanged --channel ScrollCore --period last-28d --json`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if period == "" {
				period = "last-7d"
			}
			startDate, endDate, err := resolveDateRange("", period, "", "", includePartial)
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

			rows := make([]map[string]any, 0, len(whatchangedMetrics))
			for _, m := range whatchangedMetrics {
				cur, _ := a.sumMetric(channelID, m, startDate, endDate)
				pri, _ := a.sumMetric(channelID, m, pStart, pEnd)
				pc := pctChange(cur, pri)
				rows = append(rows, map[string]any{
					"metric":     m,
					"current":    cur,
					"prior":      pri,
					"delta":      cur - pri,
					"pct_change": pc,
				})
			}
			sort.SliceStable(rows, func(i, j int) bool {
				return math.Abs(rows[i]["pct_change"].(float64)) > math.Abs(rows[j]["pct_change"].(float64))
			})

			out := map[string]any{
				"channel":      name,
				"period":       map[string]string{"start": startDate, "end": endDate},
				"prior_period": map[string]string{"start": pStart, "end": pEnd},
				"changes":      rows,
			}

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "What changed for %s  (%s..%s vs %s..%s)\n\n", name, startDate, endDate, pStart, pEnd)
				tw := newTabWriter(cmd.OutOrStdout())
				fmt.Fprintln(tw, bold("METRIC")+"\t"+bold("CURRENT")+"\t"+bold("PRIOR")+"\t"+bold("DELTA")+"\t"+bold("CHANGE"))
				for _, r := range rows {
					fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%+.1f%%\n",
						r["metric"], fmtNum(r["current"].(float64)), fmtNum(r["prior"].(float64)),
						fmtSignedNum(r["delta"].(float64)), r["pct_change"].(float64))
				}
				return tw.Flush()
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}

	cmd.Flags().StringVar(&channel, "channel", "", "Channel name or UC... id")
	cmd.Flags().StringVar(&period, "period", "last-7d", "Current window vs the prior equal window: last-week, last-7d, last-28d, last-30d, this-month, last-month, ytd")
	cmd.Flags().BoolVar(&includePartial, "include-partial", false, "Include the most recent (possibly incomplete) days")

	return cmd
}
