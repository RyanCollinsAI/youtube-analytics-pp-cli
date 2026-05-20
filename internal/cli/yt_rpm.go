// Copyright 2026 Ryan Collins. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

func newRpmCmd(flags *rootFlags) *cobra.Command {
	var channel, by, last, period, start, end string
	var includePartial bool

	cmd := &cobra.Command{
		Use:         "rpm",
		Short:       "RPM, CPM, and monetized playbacks per period bucket from the local archive",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: strings.Trim(`
  youtube-analytics-pp-cli rpm --channel ScrollCore --period ytd
  youtube-analytics-pp-cli rpm --channel ScrollCore --by day --last 28d --json`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if by == "" {
				by = "month"
			}
			if by != "month" && by != "day" {
				return usageErr(fmt.Errorf("invalid --by %q: use month or day", by))
			}
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

			// Aggregate daily series into period buckets.
			type bucket struct {
				revenue   float64
				views     float64
				monetized float64
				cpmSum    float64
				cpmDays   int
			}
			buckets := map[string]*bucket{}
			order := []string{}
			get := func(key string) *bucket {
				b, ok := buckets[key]
				if !ok {
					b = &bucket{}
					buckets[key] = b
					order = append(order, key)
				}
				return b
			}
			bucketKey := func(day string) string {
				if by == "day" {
					return day
				}
				if len(day) >= 7 {
					return day[:7]
				}
				return day
			}

			for _, d := range a.dailySeries(channelID, "estimatedRevenue", startDate, endDate) {
				get(bucketKey(d.Day)).revenue += d.Value
			}
			for _, d := range a.dailySeries(channelID, "views", startDate, endDate) {
				get(bucketKey(d.Day)).views += d.Value
			}
			for _, d := range a.dailySeries(channelID, "monetizedPlaybacks", startDate, endDate) {
				get(bucketKey(d.Day)).monetized += d.Value
			}
			for _, d := range a.dailySeries(channelID, "cpm", startDate, endDate) {
				b := get(bucketKey(d.Day))
				b.cpmSum += d.Value
				b.cpmDays++
			}
			sort.Strings(order)

			rows := make([]map[string]any, 0, len(order))
			for _, key := range order {
				b := buckets[key]
				var rpm, cpm float64
				if b.views > 0 {
					rpm = b.revenue / (b.views / 1000)
				}
				if b.cpmDays > 0 {
					cpm = b.cpmSum / float64(b.cpmDays)
				}
				rows = append(rows, map[string]any{
					"period":              key,
					"estimated_revenue":   b.revenue,
					"views":               b.views,
					"monetized_playbacks": b.monetized,
					"rpm":                 rpm,
					"cpm":                 cpm,
				})
			}

			if len(rows) == 0 {
				return fmt.Errorf("no revenue rows in the archive for %q over %s..%s", name, startDate, endDate)
			}

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "RPM for %s  (%s to %s)\n\n", name, startDate, endDate)
				tw := newTabWriter(cmd.OutOrStdout())
				fmt.Fprintln(tw, bold("PERIOD")+"\t"+bold("REVENUE")+"\t"+bold("VIEWS")+"\t"+bold("MONETIZED")+"\t"+bold("RPM")+"\t"+bold("CPM"))
				for _, r := range rows {
					fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%.2f\t%.2f\n",
						r["period"], fmtNum(r["estimated_revenue"].(float64)), fmtNum(r["views"].(float64)),
						fmtNum(r["monetized_playbacks"].(float64)), r["rpm"].(float64), r["cpm"].(float64))
				}
				return tw.Flush()
			}
			return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
		},
	}

	cmd.Flags().StringVar(&channel, "channel", "", "Channel name or UC... id")
	cmd.Flags().StringVar(&by, "by", "month", "Bucket by: month or day")
	cmd.Flags().StringVar(&last, "last", "", "Trailing window, e.g. 7d, 4w, 3m")
	cmd.Flags().StringVar(&period, "period", "", "Named period: last-week, last-7d, last-28d, last-30d, this-month, last-month, ytd")
	cmd.Flags().StringVar(&start, "start", "", "Explicit start date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&end, "end", "", "Explicit end date (YYYY-MM-DD)")
	cmd.Flags().BoolVar(&includePartial, "include-partial", false, "Include the most recent (possibly incomplete) days")

	return cmd
}
