// Copyright 2026 ryandacoder. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newChannelsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "channels",
		Short: "Map friendly channel names to YouTube channel IDs",
		Long: `Register channels by a friendly name so every other command can take
--channel <name> instead of a raw UC... id. The registry is stored in the
local SQLite archive.`,
		RunE: parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newChannelsAddCmd(flags))
	cmd.AddCommand(newChannelsListCmd(flags))
	cmd.AddCommand(newChannelsRemoveCmd(flags))
	return cmd
}

func newChannelsAddCmd(flags *rootFlags) *cobra.Command {
	var name, channelID string

	cmd := &cobra.Command{
		Use:   "add",
		Short: "Register a channel name -> channel ID mapping",
		Example: strings.Trim(`
  youtube-analytics-pp-cli channels add --name ScrollCore --channel-id UCabcdefghijklmnopqrstuv`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if name == "" && channelID == "" {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if name == "" || channelID == "" {
				return usageErr(fmt.Errorf("both --name and --channel-id are required"))
			}
			if !channelIDRE.MatchString(channelID) {
				return usageErr(fmt.Errorf("invalid --channel-id %q: expected a UC... YouTube channel id", channelID))
			}

			a, err := openArchive(cmd.Context(), defaultDBPath("youtube-analytics-pp-cli"))
			if err != nil {
				return fmt.Errorf("opening local archive: %w", err)
			}
			defer a.close()

			if err := a.addChannel(name, channelID); err != nil {
				return fmt.Errorf("registering channel: %w", err)
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
				"status":     "registered",
				"name":       name,
				"channel_id": channelID,
			}, flags)
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "Friendly channel name (e.g. ScrollCore)")
	cmd.Flags().StringVar(&channelID, "channel-id", "", "YouTube channel id (UC...)")
	return cmd
}

func newChannelsListCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List YouTube channels registered in the local archive, with name, channel id, and date added",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: strings.Trim(`
  youtube-analytics-pp-cli channels list
  youtube-analytics-pp-cli channels list --json`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			a, err := openArchive(cmd.Context(), defaultDBPath("youtube-analytics-pp-cli"))
			if err != nil {
				return fmt.Errorf("opening local archive: %w", err)
			}
			defer a.close()

			chans := a.listChannels()
			rows := make([]map[string]any, 0, len(chans))
			for _, c := range chans {
				rows = append(rows, map[string]any{
					"name":       c.Name,
					"channel_id": c.ChannelID,
					"added_at":   c.AddedAt,
				})
			}

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				if len(rows) == 0 {
					fmt.Fprintln(cmd.OutOrStdout(), "No channels registered. Add one with 'channels add --name <name> --channel-id UC...'.")
					return nil
				}
				tw := newTabWriter(cmd.OutOrStdout())
				fmt.Fprintln(tw, bold("NAME")+"\t"+bold("CHANNEL ID")+"\t"+bold("ADDED"))
				for _, c := range chans {
					fmt.Fprintf(tw, "%s\t%s\t%s\n", c.Name, c.ChannelID, c.AddedAt)
				}
				return tw.Flush()
			}
			return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
		},
	}
	return cmd
}

func newChannelsRemoveCmd(flags *rootFlags) *cobra.Command {
	var name string

	cmd := &cobra.Command{
		Use:   "remove",
		Short: "Remove a registered channel",
		Example: strings.Trim(`
  youtube-analytics-pp-cli channels remove --name ScrollCore`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if name == "" {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			a, err := openArchive(cmd.Context(), defaultDBPath("youtube-analytics-pp-cli"))
			if err != nil {
				return fmt.Errorf("opening local archive: %w", err)
			}
			defer a.close()

			removed, err := a.removeChannel(name)
			if err != nil {
				return fmt.Errorf("removing channel: %w", err)
			}
			if !removed {
				if flags.ignoreMissing {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
						"status": "noop",
						"name":   name,
					}, flags)
				}
				return notFoundErr(fmt.Errorf("channel %q is not registered", name))
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
				"status": "removed",
				"name":   name,
			}, flags)
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "Friendly channel name to remove")
	return cmd
}
