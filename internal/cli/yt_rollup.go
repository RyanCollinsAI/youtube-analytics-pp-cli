// Copyright 2026 Ryan Collins. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

func newRollupCmd(flags *rootFlags) *cobra.Command {
	var last, period, start, end, sortBy string
	var includePartial bool

	cmd := &cobra.Command{
		Use:         "rollup",
		Short:       "Every registered channel side by side for one period",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: strings.Trim(`
  youtube-analytics-pp-cli rollup --period last-week
  youtube-analytics-pp-cli rollup --last 28d --sort revenue --json`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			startDate, endDate, err := resolveDateRange(last, period, start, end, includePartial)
			if err != nil {
				return err
			}
			switch sortBy {
			case "", "views", "revenue":
			default:
				return usageErr(fmt.Errorf("invalid --sort %q: use views or revenue", sortBy))
			}
			if dryRunOK(flags) {
				return nil
			}

			a, err := openArchive(cmd.Context(), defaultDBPath("youtube-analytics-pp-cli"))
			if err != nil {
				return fmt.Errorf("opening local archive: %w", err)
			}
			defer a.close()

			chans := a.listChannels()
			if len(chans) == 0 {
				return usageErr(fmt.Errorf("no channels registered; run 'channels add --name <name> --channel-id UC...'"))
			}

			rows := make([]map[string]any, 0, len(chans))
			for _, ch := range chans {
				views, _ := a.sumMetric(ch.ChannelID, "views", startDate, endDate)
				watch, _ := a.sumMetric(ch.ChannelID, "estimatedMinutesWatched", startDate, endDate)
				gained, _ := a.sumMetric(ch.ChannelID, "subscribersGained", startDate, endDate)
				lost, _ := a.sumMetric(ch.ChannelID, "subscribersLost", startDate, endDate)
				revenue, _ := a.sumMetric(ch.ChannelID, "estimatedRevenue", startDate, endDate)
				rows = append(rows, map[string]any{
					"channel":            ch.Name,
					"views":              views,
					"watch_time_minutes": watch,
					"subscribers_net":    gained - lost,
					"estimated_revenue":  revenue,
				})
			}

			key := "views"
			if sortBy == "revenue" {
				key = "estimated_revenue"
			}
			sort.SliceStable(rows, func(i, j int) bool {
				return rows[i][key].(float64) > rows[j][key].(float64)
			})

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "Rollup  (%s to %s)\n\n", startDate, endDate)
				tw := newTabWriter(cmd.OutOrStdout())
				fmt.Fprintln(tw, bold("CHANNEL")+"\t"+bold("VIEWS")+"\t"+bold("WATCH MIN")+"\t"+bold("SUBS NET")+"\t"+bold("REVENUE"))
				for _, r := range rows {
					fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n",
						r["channel"], fmtNum(r["views"].(float64)), fmtNum(r["watch_time_minutes"].(float64)),
						fmtSignedNum(r["subscribers_net"].(float64)), fmtNum(r["estimated_revenue"].(float64)))
				}
				return tw.Flush()
			}
			return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
		},
	}

	cmd.Flags().StringVar(&last, "last", "", "Trailing window, e.g. 7d, 4w, 3m")
	cmd.Flags().StringVar(&period, "period", "", "Named period: last-week, last-7d, last-28d, last-30d, this-month, last-month, ytd")
	cmd.Flags().StringVar(&start, "start", "", "Explicit start date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&end, "end", "", "Explicit end date (YYYY-MM-DD)")
	cmd.Flags().BoolVar(&includePartial, "include-partial", false, "Include the most recent (possibly incomplete) days")
	cmd.Flags().StringVar(&sortBy, "sort", "views", "Sort channels by: views or revenue")

	return cmd
}
