// Copyright 2026 Ryan Collins. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// reportPreset describes one canned YouTube Analytics report. Constructors
// fill this for the shared runner; the cobra.Command shell (Use, flags) is
// declared literally per command so static scanners can see it.
type reportPreset struct {
	metrics    string
	dimensions string
	sort       string
}

// presetInputs carries the resolved flag values into the shared runner.
type presetInputs struct {
	channel        string
	last           string
	period         string
	start          string
	end            string
	currency       string
	includePartial bool

	// by switches a dimension; empty disables it.
	by        string
	byChoices []string
	byDefault string
	// byAddsDimension: by adds its value rather than replacing dimension[0].
	byAddsDimension bool

	// limit maps to maxResults; <=0 means unset.
	limit int

	// sortChoice maps views|revenue|watch-time to a sort spec.
	sortChoice    string
	hasSortChoice bool

	// videoID, when set, adds a video== filter.
	videoID string

	// extraFilters is a static filter appended to every request.
	extraFilters string
}

// runPreset executes a report preset: resolves channel + dates, runs the
// query, decodes, and prints. Shared by all 15 preset constructors.
func runPreset(cmd *cobra.Command, flags *rootFlags, p reportPreset, in presetInputs) error {
	startDate, endDate, err := resolveDateRange(in.last, in.period, in.start, in.end, in.includePartial)
	if err != nil {
		return err
	}

	metrics := p.metrics
	dimensions := p.dimensions
	sortBy := p.sort

	if in.hasSortChoice {
		switch in.sortChoice {
		case "", "views":
			sortBy = "-views"
		case "revenue":
			sortBy = "-estimatedRevenue"
			if !strings.Contains(metrics, "estimatedRevenue") {
				metrics += ",estimatedRevenue"
			}
		case "watch-time":
			sortBy = "-estimatedMinutesWatched"
		default:
			return usageErr(fmt.Errorf("invalid --sort %q: use views, revenue, or watch-time", in.sortChoice))
		}
	}

	if len(in.byChoices) > 0 {
		chosen := in.by
		if chosen == "" {
			chosen = in.byDefault
		}
		// An empty chosen value is allowed only for additive --by: it means
		// "no extra dimension" (single total row).
		if chosen != "" {
			valid := false
			for _, c := range in.byChoices {
				if c == chosen {
					valid = true
					break
				}
			}
			if !valid {
				return usageErr(fmt.Errorf("invalid --by %q: use one of %s", chosen, strings.Join(in.byChoices, ", ")))
			}
			if in.byAddsDimension {
				if dimensions == "" {
					dimensions = chosen
				} else {
					dimensions = chosen + "," + dimensions
				}
			} else {
				dimensions = chosen
			}
		}
	}

	filters := in.extraFilters
	if in.videoID != "" {
		if filters != "" {
			filters += ";"
		}
		filters += "video==" + in.videoID
	}

	c, err := flags.newClient()
	if err != nil {
		return err
	}

	channelID, _, err := resolveChannel(flags, in.channel)
	if err != nil {
		return err
	}

	raw, err := runReportQuery(c, channelID, reportParams{
		StartDate:  startDate,
		EndDate:    endDate,
		Metrics:    metrics,
		Dimensions: dimensions,
		Filters:    filters,
		Sort:       sortBy,
		MaxResults: in.limit,
		Currency:   in.currency,
	})
	if err != nil {
		return classifyAPIError(err, flags)
	}

	headers, rows, err := decodeReportRows(raw)
	if err != nil {
		return err
	}

	// No dimension -> a single total row; emit one object.
	if dimensions == "" {
		obj := map[string]any{}
		if len(rows) > 0 {
			obj = rows[0]
		}
		if wantsHumanTable(cmd.OutOrStdout(), flags) {
			return printPresetTotalsTable(cmd, headers, obj)
		}
		return printJSONFiltered(cmd.OutOrStdout(), obj, flags)
	}

	if wantsHumanTable(cmd.OutOrStdout(), flags) {
		return printPresetRowsTable(cmd, headers, rows)
	}
	return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
}

// printPresetTotalsTable prints a single total row as a two-column table.
func printPresetTotalsTable(cmd *cobra.Command, headers []string, obj map[string]any) error {
	tw := newTabWriter(cmd.OutOrStdout())
	fmt.Fprintln(tw, bold("METRIC")+"\t"+bold("VALUE"))
	keys := headers
	if len(keys) == 0 {
		keys = sortedKeys(obj)
	}
	for _, k := range keys {
		fmt.Fprintf(tw, "%s\t%s\n", k, formatCellValue(obj[k]))
	}
	return tw.Flush()
}

// printPresetRowsTable prints dimension rows as a table.
func printPresetRowsTable(cmd *cobra.Command, headers []string, rows []map[string]any) error {
	if len(rows) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No data for the requested period.")
		return nil
	}
	tw := newTabWriter(cmd.OutOrStdout())
	hdr := make([]string, len(headers))
	for i, c := range headers {
		hdr[i] = bold(strings.ToUpper(c))
	}
	fmt.Fprintln(tw, strings.Join(hdr, "\t"))
	for _, r := range rows {
		cells := make([]string, len(headers))
		for i, c := range headers {
			cells[i] = formatCellValue(r[c])
		}
		fmt.Fprintln(tw, strings.Join(cells, "\t"))
	}
	return tw.Flush()
}

func sortedKeys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

const overviewMetrics = "views,estimatedMinutesWatched,averageViewDuration,averageViewPercentage,subscribersGained,subscribersLost,likes,comments,shares"

func newOverviewCmd(flags *rootFlags) *cobra.Command {
	var channel, last, period, start, end, currency string
	var includePartial bool

	cmd := &cobra.Command{
		Use: "overview",

		Short:       "Channel-level totals: views, watch time, subscribers, and engagement",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: strings.Trim(`
  youtube-analytics-pp-cli overview --channel ScrollCore --last 28d
  youtube-analytics-pp-cli overview --channel ScrollCore --period last-month --json`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			return runPreset(cmd, flags, reportPreset{metrics: overviewMetrics}, presetInputs{
				channel: channel, last: last, period: period, start: start, end: end,
				currency: currency, includePartial: includePartial,
			})
		},
	}
	cmd.Flags().StringVar(&channel, "channel", "", "Channel name or UC... id (default: the sole registered channel)")
	cmd.Flags().StringVar(&last, "last", "", "Trailing window, e.g. 7d, 4w, 3m")
	cmd.Flags().StringVar(&period, "period", "", "Named period: last-week, last-7d, last-28d, last-30d, this-month, last-month, ytd")
	cmd.Flags().StringVar(&start, "start", "", "Explicit start date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&end, "end", "", "Explicit end date (YYYY-MM-DD)")
	cmd.Flags().BoolVar(&includePartial, "include-partial", false, "Include the most recent (possibly incomplete) days")
	cmd.Flags().StringVar(&currency, "currency", "", "3-letter currency code for monetary metrics")
	return cmd
}

func newViewsCmd(flags *rootFlags) *cobra.Command {
	var channel, last, period, start, end, currency, by string
	var includePartial bool

	cmd := &cobra.Command{
		Use: "views",

		Short:       "Views and watch time over time",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: strings.Trim(`
  youtube-analytics-pp-cli views --channel ScrollCore --last 28d
  youtube-analytics-pp-cli views --channel ScrollCore --by month --period ytd`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			return runPreset(cmd, flags, reportPreset{
				metrics: "views,estimatedMinutesWatched", dimensions: "day", sort: "day",
			}, presetInputs{
				channel: channel, last: last, period: period, start: start, end: end,
				currency: currency, includePartial: includePartial,
				by: by, byChoices: []string{"day", "month"}, byDefault: "day",
			})
		},
	}
	cmd.Flags().StringVar(&channel, "channel", "", "Channel name or UC... id (default: the sole registered channel)")
	cmd.Flags().StringVar(&last, "last", "", "Trailing window, e.g. 7d, 4w, 3m")
	cmd.Flags().StringVar(&period, "period", "", "Named period: last-week, last-7d, last-28d, last-30d, this-month, last-month, ytd")
	cmd.Flags().StringVar(&start, "start", "", "Explicit start date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&end, "end", "", "Explicit end date (YYYY-MM-DD)")
	cmd.Flags().BoolVar(&includePartial, "include-partial", false, "Include the most recent (possibly incomplete) days")
	cmd.Flags().StringVar(&currency, "currency", "", "3-letter currency code for monetary metrics")
	cmd.Flags().StringVar(&by, "by", "day", "Group by: day or month")
	return cmd
}

func newTopVideosCmd(flags *rootFlags) *cobra.Command {
	var channel, last, period, start, end, currency, sortChoice string
	var includePartial bool
	var limit int

	cmd := &cobra.Command{
		Use: "top-videos",

		Short:       "Best-performing videos for a period",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: strings.Trim(`
  youtube-analytics-pp-cli top-videos --channel ScrollCore --last 28d --limit 10
  youtube-analytics-pp-cli top-videos --channel ScrollCore --sort revenue --limit 20`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			return runPreset(cmd, flags, reportPreset{
				metrics:    "views,estimatedMinutesWatched,averageViewDuration,subscribersGained",
				dimensions: "video", sort: "-views",
			}, presetInputs{
				channel: channel, last: last, period: period, start: start, end: end,
				currency: currency, includePartial: includePartial,
				limit: limit, sortChoice: sortChoice, hasSortChoice: true,
			})
		},
	}
	cmd.Flags().StringVar(&channel, "channel", "", "Channel name or UC... id (default: the sole registered channel)")
	cmd.Flags().StringVar(&last, "last", "", "Trailing window, e.g. 7d, 4w, 3m")
	cmd.Flags().StringVar(&period, "period", "", "Named period: last-week, last-7d, last-28d, last-30d, this-month, last-month, ytd")
	cmd.Flags().StringVar(&start, "start", "", "Explicit start date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&end, "end", "", "Explicit end date (YYYY-MM-DD)")
	cmd.Flags().BoolVar(&includePartial, "include-partial", false, "Include the most recent (possibly incomplete) days")
	cmd.Flags().StringVar(&currency, "currency", "", "3-letter currency code for monetary metrics")
	cmd.Flags().StringVar(&sortChoice, "sort", "views", "Sort by: views, revenue, watch-time")
	cmd.Flags().IntVar(&limit, "limit", 10, "Maximum rows to return")
	return cmd
}

func newVideoCmd(flags *rootFlags) *cobra.Command {
	var channel, last, period, start, end, currency string
	var includePartial bool

	cmd := &cobra.Command{
		Use: "video <videoId>",

		Short:       "Totals for a single video",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: strings.Trim(`
  youtube-analytics-pp-cli video dQw4w9WgXcQ --channel ScrollCore --last 28d
  youtube-analytics-pp-cli video dQw4w9WgXcQ --channel ScrollCore --period last-month`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			return runPreset(cmd, flags, reportPreset{
				metrics: "views,estimatedMinutesWatched,averageViewDuration,averageViewPercentage,likes,comments,shares,subscribersGained",
			}, presetInputs{
				channel: channel, last: last, period: period, start: start, end: end,
				currency: currency, includePartial: includePartial, videoID: args[0],
			})
		},
	}
	cmd.Flags().StringVar(&channel, "channel", "", "Channel name or UC... id (default: the sole registered channel)")
	cmd.Flags().StringVar(&last, "last", "", "Trailing window, e.g. 7d, 4w, 3m")
	cmd.Flags().StringVar(&period, "period", "", "Named period: last-week, last-7d, last-28d, last-30d, this-month, last-month, ytd")
	cmd.Flags().StringVar(&start, "start", "", "Explicit start date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&end, "end", "", "Explicit end date (YYYY-MM-DD)")
	cmd.Flags().BoolVar(&includePartial, "include-partial", false, "Include the most recent (possibly incomplete) days")
	cmd.Flags().StringVar(&currency, "currency", "", "3-letter currency code for monetary metrics")
	return cmd
}

func newRetentionCmd(flags *rootFlags) *cobra.Command {
	var channel, last, period, start, end, currency string
	var includePartial bool

	cmd := &cobra.Command{
		Use: "retention <videoId>",

		Short:       "Audience retention curve for a single video",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: strings.Trim(`
  youtube-analytics-pp-cli retention dQw4w9WgXcQ --channel ScrollCore --last 28d
  youtube-analytics-pp-cli retention dQw4w9WgXcQ --channel ScrollCore --json`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			return runPreset(cmd, flags, reportPreset{
				metrics:    "audienceWatchRatio,relativeRetentionPerformance",
				dimensions: "elapsedVideoTimeRatio", sort: "elapsedVideoTimeRatio",
			}, presetInputs{
				channel: channel, last: last, period: period, start: start, end: end,
				currency: currency, includePartial: includePartial, videoID: args[0],
			})
		},
	}
	cmd.Flags().StringVar(&channel, "channel", "", "Channel name or UC... id (default: the sole registered channel)")
	cmd.Flags().StringVar(&last, "last", "", "Trailing window, e.g. 7d, 4w, 3m")
	cmd.Flags().StringVar(&period, "period", "", "Named period: last-week, last-7d, last-28d, last-30d, this-month, last-month, ytd")
	cmd.Flags().StringVar(&start, "start", "", "Explicit start date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&end, "end", "", "Explicit end date (YYYY-MM-DD)")
	cmd.Flags().BoolVar(&includePartial, "include-partial", false, "Include the most recent (possibly incomplete) days")
	cmd.Flags().StringVar(&currency, "currency", "", "3-letter currency code for monetary metrics")
	return cmd
}

func newDemographicsCmd(flags *rootFlags) *cobra.Command {
	var channel, last, period, start, end, currency string
	var includePartial bool

	cmd := &cobra.Command{
		Use: "demographics",

		Short:       "Viewer age and gender breakdown",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: strings.Trim(`
  youtube-analytics-pp-cli demographics --channel ScrollCore --last 28d
  youtube-analytics-pp-cli demographics --channel ScrollCore --period last-month --json`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			return runPreset(cmd, flags, reportPreset{
				metrics: "viewerPercentage", dimensions: "ageGroup,gender",
			}, presetInputs{
				channel: channel, last: last, period: period, start: start, end: end,
				currency: currency, includePartial: includePartial,
			})
		},
	}
	cmd.Flags().StringVar(&channel, "channel", "", "Channel name or UC... id (default: the sole registered channel)")
	cmd.Flags().StringVar(&last, "last", "", "Trailing window, e.g. 7d, 4w, 3m")
	cmd.Flags().StringVar(&period, "period", "", "Named period: last-week, last-7d, last-28d, last-30d, this-month, last-month, ytd")
	cmd.Flags().StringVar(&start, "start", "", "Explicit start date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&end, "end", "", "Explicit end date (YYYY-MM-DD)")
	cmd.Flags().BoolVar(&includePartial, "include-partial", false, "Include the most recent (possibly incomplete) days")
	cmd.Flags().StringVar(&currency, "currency", "", "3-letter currency code for monetary metrics")
	return cmd
}

func newTrafficCmd(flags *rootFlags) *cobra.Command {
	var channel, last, period, start, end, currency string
	var includePartial bool

	cmd := &cobra.Command{
		Use: "traffic",

		Short:       "Views by traffic source type",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: strings.Trim(`
  youtube-analytics-pp-cli traffic --channel ScrollCore --last 28d
  youtube-analytics-pp-cli traffic --channel ScrollCore --period last-month`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			return runPreset(cmd, flags, reportPreset{
				metrics: "views,estimatedMinutesWatched", dimensions: "insightTrafficSourceType", sort: "-views",
			}, presetInputs{
				channel: channel, last: last, period: period, start: start, end: end,
				currency: currency, includePartial: includePartial,
			})
		},
	}
	cmd.Flags().StringVar(&channel, "channel", "", "Channel name or UC... id (default: the sole registered channel)")
	cmd.Flags().StringVar(&last, "last", "", "Trailing window, e.g. 7d, 4w, 3m")
	cmd.Flags().StringVar(&period, "period", "", "Named period: last-week, last-7d, last-28d, last-30d, this-month, last-month, ytd")
	cmd.Flags().StringVar(&start, "start", "", "Explicit start date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&end, "end", "", "Explicit end date (YYYY-MM-DD)")
	cmd.Flags().BoolVar(&includePartial, "include-partial", false, "Include the most recent (possibly incomplete) days")
	cmd.Flags().StringVar(&currency, "currency", "", "3-letter currency code for monetary metrics")
	return cmd
}

func newEngagementCmd(flags *rootFlags) *cobra.Command {
	var channel, last, period, start, end, currency string
	var includePartial bool

	cmd := &cobra.Command{
		Use: "engagement",

		Short:       "Likes, dislikes, comments, shares, and subscriber changes",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: strings.Trim(`
  youtube-analytics-pp-cli engagement --channel ScrollCore --last 28d
  youtube-analytics-pp-cli engagement --channel ScrollCore --period this-month --json`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			return runPreset(cmd, flags, reportPreset{
				metrics: "likes,dislikes,comments,shares,subscribersGained,subscribersLost",
			}, presetInputs{
				channel: channel, last: last, period: period, start: start, end: end,
				currency: currency, includePartial: includePartial,
			})
		},
	}
	cmd.Flags().StringVar(&channel, "channel", "", "Channel name or UC... id (default: the sole registered channel)")
	cmd.Flags().StringVar(&last, "last", "", "Trailing window, e.g. 7d, 4w, 3m")
	cmd.Flags().StringVar(&period, "period", "", "Named period: last-week, last-7d, last-28d, last-30d, this-month, last-month, ytd")
	cmd.Flags().StringVar(&start, "start", "", "Explicit start date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&end, "end", "", "Explicit end date (YYYY-MM-DD)")
	cmd.Flags().BoolVar(&includePartial, "include-partial", false, "Include the most recent (possibly incomplete) days")
	cmd.Flags().StringVar(&currency, "currency", "", "3-letter currency code for monetary metrics")
	return cmd
}

func newGeographyCmd(flags *rootFlags) *cobra.Command {
	var channel, last, period, start, end, currency, by string
	var includePartial bool

	cmd := &cobra.Command{
		Use: "geography",

		Short:       "Views by country, province, or city",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: strings.Trim(`
  youtube-analytics-pp-cli geography --channel ScrollCore --last 28d
  youtube-analytics-pp-cli geography --channel ScrollCore --by city --period last-month`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			return runPreset(cmd, flags, reportPreset{
				metrics: "views,estimatedMinutesWatched,averageViewDuration", dimensions: "country", sort: "-views",
			}, presetInputs{
				channel: channel, last: last, period: period, start: start, end: end,
				currency: currency, includePartial: includePartial,
				by: by, byChoices: []string{"country", "province", "city"}, byDefault: "country",
			})
		},
	}
	cmd.Flags().StringVar(&channel, "channel", "", "Channel name or UC... id (default: the sole registered channel)")
	cmd.Flags().StringVar(&last, "last", "", "Trailing window, e.g. 7d, 4w, 3m")
	cmd.Flags().StringVar(&period, "period", "", "Named period: last-week, last-7d, last-28d, last-30d, this-month, last-month, ytd")
	cmd.Flags().StringVar(&start, "start", "", "Explicit start date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&end, "end", "", "Explicit end date (YYYY-MM-DD)")
	cmd.Flags().BoolVar(&includePartial, "include-partial", false, "Include the most recent (possibly incomplete) days")
	cmd.Flags().StringVar(&currency, "currency", "", "3-letter currency code for monetary metrics")
	cmd.Flags().StringVar(&by, "by", "country", "Group by: country, province, or city")
	return cmd
}

func newPlaybackLocationsCmd(flags *rootFlags) *cobra.Command {
	var channel, last, period, start, end, currency string
	var includePartial bool

	cmd := &cobra.Command{
		Use: "playback-locations",

		Short:       "Views by playback location type",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: strings.Trim(`
  youtube-analytics-pp-cli playback-locations --channel ScrollCore --last 28d
  youtube-analytics-pp-cli playback-locations --channel ScrollCore --period last-month`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			return runPreset(cmd, flags, reportPreset{
				metrics: "views,estimatedMinutesWatched", dimensions: "insightPlaybackLocationType", sort: "-views",
			}, presetInputs{
				channel: channel, last: last, period: period, start: start, end: end,
				currency: currency, includePartial: includePartial,
			})
		},
	}
	cmd.Flags().StringVar(&channel, "channel", "", "Channel name or UC... id (default: the sole registered channel)")
	cmd.Flags().StringVar(&last, "last", "", "Trailing window, e.g. 7d, 4w, 3m")
	cmd.Flags().StringVar(&period, "period", "", "Named period: last-week, last-7d, last-28d, last-30d, this-month, last-month, ytd")
	cmd.Flags().StringVar(&start, "start", "", "Explicit start date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&end, "end", "", "Explicit end date (YYYY-MM-DD)")
	cmd.Flags().BoolVar(&includePartial, "include-partial", false, "Include the most recent (possibly incomplete) days")
	cmd.Flags().StringVar(&currency, "currency", "", "3-letter currency code for monetary metrics")
	return cmd
}

func newDevicesCmd(flags *rootFlags) *cobra.Command {
	var channel, last, period, start, end, currency, by string
	var includePartial bool

	cmd := &cobra.Command{
		Use: "devices",

		Short:       "Views by device type or operating system",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: strings.Trim(`
  youtube-analytics-pp-cli devices --channel ScrollCore --last 28d
  youtube-analytics-pp-cli devices --channel ScrollCore --by operatingSystem`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			return runPreset(cmd, flags, reportPreset{
				metrics: "views,estimatedMinutesWatched", dimensions: "deviceType", sort: "-views",
			}, presetInputs{
				channel: channel, last: last, period: period, start: start, end: end,
				currency: currency, includePartial: includePartial,
				by: by, byChoices: []string{"deviceType", "operatingSystem"}, byDefault: "deviceType",
			})
		},
	}
	cmd.Flags().StringVar(&channel, "channel", "", "Channel name or UC... id (default: the sole registered channel)")
	cmd.Flags().StringVar(&last, "last", "", "Trailing window, e.g. 7d, 4w, 3m")
	cmd.Flags().StringVar(&period, "period", "", "Named period: last-week, last-7d, last-28d, last-30d, this-month, last-month, ytd")
	cmd.Flags().StringVar(&start, "start", "", "Explicit start date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&end, "end", "", "Explicit end date (YYYY-MM-DD)")
	cmd.Flags().BoolVar(&includePartial, "include-partial", false, "Include the most recent (possibly incomplete) days")
	cmd.Flags().StringVar(&currency, "currency", "", "3-letter currency code for monetary metrics")
	cmd.Flags().StringVar(&by, "by", "deviceType", "Group by: deviceType or operatingSystem")
	return cmd
}

func newRevenueCmd(flags *rootFlags) *cobra.Command {
	var channel, last, period, start, end, currency, by string
	var includePartial bool

	cmd := &cobra.Command{
		Use: "revenue",

		Short:       "Estimated revenue, ad revenue, CPM, and monetized playbacks",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: strings.Trim(`
  youtube-analytics-pp-cli revenue --channel ScrollCore --last 28d
  youtube-analytics-pp-cli revenue --channel ScrollCore --by month --period ytd`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			return runPreset(cmd, flags, reportPreset{
				metrics: "estimatedRevenue,estimatedAdRevenue,grossRevenue,cpm,playbackBasedCpm,monetizedPlaybacks,adImpressions",
			}, presetInputs{
				channel: channel, last: last, period: period, start: start, end: end,
				currency: currency, includePartial: includePartial,
				by: by, byChoices: []string{"day", "month"}, byDefault: "", byAddsDimension: true,
			})
		},
	}
	cmd.Flags().StringVar(&channel, "channel", "", "Channel name or UC... id (default: the sole registered channel)")
	cmd.Flags().StringVar(&last, "last", "", "Trailing window, e.g. 7d, 4w, 3m")
	cmd.Flags().StringVar(&period, "period", "", "Named period: last-week, last-7d, last-28d, last-30d, this-month, last-month, ytd")
	cmd.Flags().StringVar(&start, "start", "", "Explicit start date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&end, "end", "", "Explicit end date (YYYY-MM-DD)")
	cmd.Flags().BoolVar(&includePartial, "include-partial", false, "Include the most recent (possibly incomplete) days")
	cmd.Flags().StringVar(&currency, "currency", "", "3-letter currency code for monetary metrics")
	cmd.Flags().StringVar(&by, "by", "", "Group by: day or month (default: single total row)")
	return cmd
}

func newSubscribersCmd(flags *rootFlags) *cobra.Command {
	var channel, last, period, start, end, currency, by string
	var includePartial bool

	cmd := &cobra.Command{
		Use: "subscribers",

		Short:       "Subscribers gained and lost over time",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: strings.Trim(`
  youtube-analytics-pp-cli subscribers --channel ScrollCore --last 28d
  youtube-analytics-pp-cli subscribers --channel ScrollCore --by month --period ytd`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			return runPreset(cmd, flags, reportPreset{
				metrics: "subscribersGained,subscribersLost", dimensions: "day", sort: "day",
			}, presetInputs{
				channel: channel, last: last, period: period, start: start, end: end,
				currency: currency, includePartial: includePartial,
				by: by, byChoices: []string{"day", "month"}, byDefault: "day",
			})
		},
	}
	cmd.Flags().StringVar(&channel, "channel", "", "Channel name or UC... id (default: the sole registered channel)")
	cmd.Flags().StringVar(&last, "last", "", "Trailing window, e.g. 7d, 4w, 3m")
	cmd.Flags().StringVar(&period, "period", "", "Named period: last-week, last-7d, last-28d, last-30d, this-month, last-month, ytd")
	cmd.Flags().StringVar(&start, "start", "", "Explicit start date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&end, "end", "", "Explicit end date (YYYY-MM-DD)")
	cmd.Flags().BoolVar(&includePartial, "include-partial", false, "Include the most recent (possibly incomplete) days")
	cmd.Flags().StringVar(&currency, "currency", "", "3-letter currency code for monetary metrics")
	cmd.Flags().StringVar(&by, "by", "day", "Group by: day or month")
	return cmd
}

func newSharingCmd(flags *rootFlags) *cobra.Command {
	var channel, last, period, start, end, currency string
	var includePartial bool

	cmd := &cobra.Command{
		Use: "sharing",

		Short:       "Shares by sharing service",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: strings.Trim(`
  youtube-analytics-pp-cli sharing --channel ScrollCore --last 28d
  youtube-analytics-pp-cli sharing --channel ScrollCore --period last-month --json`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			return runPreset(cmd, flags, reportPreset{
				metrics: "shares", dimensions: "sharingService", sort: "-shares",
			}, presetInputs{
				channel: channel, last: last, period: period, start: start, end: end,
				currency: currency, includePartial: includePartial,
			})
		},
	}
	cmd.Flags().StringVar(&channel, "channel", "", "Channel name or UC... id (default: the sole registered channel)")
	cmd.Flags().StringVar(&last, "last", "", "Trailing window, e.g. 7d, 4w, 3m")
	cmd.Flags().StringVar(&period, "period", "", "Named period: last-week, last-7d, last-28d, last-30d, this-month, last-month, ytd")
	cmd.Flags().StringVar(&start, "start", "", "Explicit start date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&end, "end", "", "Explicit end date (YYYY-MM-DD)")
	cmd.Flags().BoolVar(&includePartial, "include-partial", false, "Include the most recent (possibly incomplete) days")
	cmd.Flags().StringVar(&currency, "currency", "", "3-letter currency code for monetary metrics")
	return cmd
}

func newPlaylistsCmd(flags *rootFlags) *cobra.Command {
	var channel, last, period, start, end, currency string
	var includePartial bool

	cmd := &cobra.Command{
		Use: "playlists",

		Short:       "Playlist views and engagement",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: strings.Trim(`
  youtube-analytics-pp-cli playlists --channel ScrollCore --last 28d
  youtube-analytics-pp-cli playlists --channel ScrollCore --period last-month`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			return runPreset(cmd, flags, reportPreset{
				metrics:    "views,estimatedMinutesWatched,playlistStarts,viewsPerPlaylistStart,averageTimeInPlaylist",
				dimensions: "playlist", sort: "-views",
			}, presetInputs{
				channel: channel, last: last, period: period, start: start, end: end,
				currency: currency, includePartial: includePartial, extraFilters: "isCurated==1",
			})
		},
	}
	cmd.Flags().StringVar(&channel, "channel", "", "Channel name or UC... id (default: the sole registered channel)")
	cmd.Flags().StringVar(&last, "last", "", "Trailing window, e.g. 7d, 4w, 3m")
	cmd.Flags().StringVar(&period, "period", "", "Named period: last-week, last-7d, last-28d, last-30d, this-month, last-month, ytd")
	cmd.Flags().StringVar(&start, "start", "", "Explicit start date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&end, "end", "", "Explicit end date (YYYY-MM-DD)")
	cmd.Flags().BoolVar(&includePartial, "include-partial", false, "Include the most recent (possibly incomplete) days")
	cmd.Flags().StringVar(&currency, "currency", "", "3-letter currency code for monetary metrics")
	return cmd
}
