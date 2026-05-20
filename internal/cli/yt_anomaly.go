// Copyright 2026 ryandacoder. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"math"
	"strings"

	"github.com/spf13/cobra"
)

func newAnomalyCmd(flags *rootFlags) *cobra.Command {
	var channel, metric string
	var window int
	var includePartial bool

	cmd := &cobra.Command{
		Use:         "anomaly",
		Short:       "Flag the latest finalized day when a metric falls outside its trailing baseline",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: strings.Trim(`
  youtube-analytics-pp-cli anomaly --channel ScrollCore
  youtube-analytics-pp-cli anomaly --channel ScrollCore --metric estimatedRevenue --window 28`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if metric == "" {
				metric = "views"
			}
			if window < 2 {
				window = 28
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

			// Pull the trailing window + 1 (the latest day under test).
			end := utcToday().AddDate(0, 0, -1)
			start := end.AddDate(0, 0, -window)
			series := a.dailySeries(channelID, metric, start.Format("2006-01-02"), end.Format("2006-01-02"))
			if len(series) < 3 {
				return fmt.Errorf("not enough archived days for %q (need at least 3, have %d); run 'sync' with a wider --days", metric, len(series))
			}

			latest := series[len(series)-1]
			baseline := series[:len(series)-1]

			var sum float64
			for _, d := range baseline {
				sum += d.Value
			}
			mean := sum / float64(len(baseline))
			var variance float64
			for _, d := range baseline {
				diff := d.Value - mean
				variance += diff * diff
			}
			stddev := math.Sqrt(variance / float64(len(baseline)))

			upper := mean + 2*stddev
			lower := mean - 2*stddev
			verdict := "normal"
			if latest.Value > upper {
				verdict = "spike"
			} else if latest.Value < lower {
				verdict = "drop"
			}

			out := map[string]any{
				"channel":       name,
				"metric":        metric,
				"window_days":   len(baseline),
				"latest_day":    latest.Day,
				"latest_value":  latest.Value,
				"baseline_mean": mean,
				"baseline_std":  stddev,
				"lower_bound":   lower,
				"upper_bound":   upper,
				"verdict":       verdict,
			}

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "Anomaly check for %s — %s\n\n", name, metric)
				tw := newTabWriter(cmd.OutOrStdout())
				fmt.Fprintf(tw, "Latest day\t%s\n", latest.Day)
				fmt.Fprintf(tw, "Latest value\t%s\n", fmtNum(latest.Value))
				fmt.Fprintf(tw, "Baseline mean\t%.2f\n", mean)
				fmt.Fprintf(tw, "Baseline stddev\t%.2f\n", stddev)
				fmt.Fprintf(tw, "Expected range\t%.2f .. %.2f\n", lower, upper)
				fmt.Fprintf(tw, "Verdict\t%s\n", verdict)
				return tw.Flush()
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}

	cmd.Flags().StringVar(&channel, "channel", "", "Channel name or UC... id")
	cmd.Flags().StringVar(&metric, "metric", "views", "Metric to test (e.g. views, estimatedRevenue)")
	cmd.Flags().IntVar(&window, "window", 28, "Trailing days used for the baseline")
	cmd.Flags().BoolVar(&includePartial, "include-partial", false, "Treat today-1 as finalized (default uses today-1)")

	return cmd
}
