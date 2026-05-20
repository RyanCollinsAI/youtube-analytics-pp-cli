// Copyright 2026 ryandacoder. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// moverRow is one video's week-over-week view change.
type moverRow struct {
	Video   string  `json:"video"`
	Current float64 `json:"current_views"`
	Prior   float64 `json:"prior_views"`
	Delta   float64 `json:"delta"`
}

func newMoversCmd(flags *rootFlags) *cobra.Command {
	var channel, period string
	var limit int
	var includePartial bool

	cmd := &cobra.Command{
		Use:         "movers",
		Short:       "Rank videos by week-over-week view delta — risers and faders",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: strings.Trim(`
  youtube-analytics-pp-cli movers --channel ScrollCore
  youtube-analytics-pp-cli movers --channel ScrollCore --period last-28d --limit 20 --json`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if period == "" {
				period = "last-7d"
			}
			if limit < 1 {
				limit = 15
			}
			startDate, endDate, err := resolveDateRange("", period, "", "", includePartial)
			if err != nil {
				return err
			}
			if dryRunOK(flags) {
				return nil
			}
			pStart, pEnd := priorPeriod(startDate, endDate)

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			channelID, name, err := resolveChannel(flags, channel)
			if err != nil {
				return err
			}

			query := func(s, e string) (map[string]float64, error) {
				raw, qerr := runReportQuery(c, channelID, reportParams{
					StartDate:  s,
					EndDate:    e,
					Metrics:    "views",
					Dimensions: "video",
					Sort:       "-views",
					MaxResults: 200,
				})
				if qerr != nil {
					return nil, qerr
				}
				_, rows, derr := decodeReportRows(raw)
				if derr != nil {
					return nil, derr
				}
				out := map[string]float64{}
				for _, r := range rows {
					id, _ := r["video"].(string)
					v, _ := r["views"].(float64)
					if id != "" {
						out[id] = v
					}
				}
				return out, nil
			}

			cur, err := query(startDate, endDate)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			pri, err := query(pStart, pEnd)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			seen := map[string]bool{}
			var all []moverRow
			add := func(id string) {
				if seen[id] {
					return
				}
				seen[id] = true
				all = append(all, moverRow{
					Video:   id,
					Current: cur[id],
					Prior:   pri[id],
					Delta:   cur[id] - pri[id],
				})
			}
			for id := range cur {
				add(id)
			}
			for id := range pri {
				add(id)
			}
			sort.SliceStable(all, func(i, j int) bool { return all[i].Delta > all[j].Delta })

			risers := []moverRow{}
			faders := []moverRow{}
			for _, m := range all {
				if m.Delta > 0 && len(risers) < limit {
					risers = append(risers, m)
				}
			}
			for i := len(all) - 1; i >= 0; i-- {
				if all[i].Delta < 0 && len(faders) < limit {
					faders = append(faders, all[i])
				}
			}

			out := map[string]any{
				"channel":      name,
				"period":       map[string]string{"start": startDate, "end": endDate},
				"prior_period": map[string]string{"start": pStart, "end": pEnd},
				"risers":       risers,
				"faders":       faders,
			}

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "Movers for %s  (%s..%s vs %s..%s)\n", name, startDate, endDate, pStart, pEnd)
				printMoverTable(cmd, "Risers", risers)
				printMoverTable(cmd, "Faders", faders)
				return nil
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}

	cmd.Flags().StringVar(&channel, "channel", "", "Channel name or UC... id")
	cmd.Flags().StringVar(&period, "period", "last-7d", "Current window vs the prior equal window: last-week, last-7d, last-28d, last-30d, this-month, last-month, ytd")
	cmd.Flags().IntVar(&limit, "limit", 15, "Maximum risers and faders to list")
	cmd.Flags().BoolVar(&includePartial, "include-partial", false, "Include the most recent (possibly incomplete) days")

	return cmd
}

func printMoverTable(cmd *cobra.Command, title string, rows []moverRow) {
	fmt.Fprintf(cmd.OutOrStdout(), "\n%s\n", bold(title))
	if len(rows) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "  (none)")
		return
	}
	tw := newTabWriter(cmd.OutOrStdout())
	fmt.Fprintln(tw, bold("VIDEO")+"\t"+bold("CURRENT")+"\t"+bold("PRIOR")+"\t"+bold("DELTA"))
	for _, r := range rows {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", r.Video, fmtNum(r.Current), fmtNum(r.Prior), fmtSignedNum(r.Delta))
	}
	tw.Flush()
}
