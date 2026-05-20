# YouTube Analytics CLI

A command-line tool for the YouTube Analytics API. It keeps a local copy of your daily numbers, which is what lets it compare one period against another, line several channels up side by side, and flag a day that looks off. The plain API can't do any of that on its own.

The YouTube Analytics API is really one endpoint, `reports.query`, and it answers with a column-array response that is awkward to read. This CLI wraps that endpoint in named reports (overview, top-videos, traffic, retention, revenue, demographics) and prints clean labeled tables instead. It also syncs your daily rows into a local SQLite database. That local copy is the part that matters: it is what makes period deltas, whatchanged diffs, anomaly checks, and multi-channel rollups work without re-querying the API every time.

Learn more about the API in [Google's YouTube Analytics docs](https://developers.google.com/youtube/analytics).

Built by [@RyanCollinsAI](https://github.com/RyanCollinsAI).

## Install

The recommended path installs both the `youtube-analytics-pp-cli` binary and the `pp-youtube-analytics` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press install youtube-analytics
```

For the CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install youtube-analytics --cli-only
```

For the skill only (this skips the CLI binary, so use it to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press install youtube-analytics --skill-only
```

To limit the skill install to one or more specific agents (repeatable; agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press install youtube-analytics --agent claude-code
npx -y @mvanhorn/printing-press install youtube-analytics --agent claude-code --agent codex
```

### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/youtube-analytics-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-youtube-analytics --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-youtube-analytics --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-youtube-analytics skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-youtube-analytics. The skill defines how its required CLI can be installed.
```

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle, which is Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/youtube-analytics-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `YOUTUBE_CLIENT_ID`, `YOUTUBE_CLIENT_SECRET`, and `YOUTUBE_REFRESH_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`). For other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "youtube-analytics": {
      "command": "youtube-analytics-pp-mcp",
      "env": {
        "YOUTUBE_CLIENT_ID": "<your-oauth-client-id>",
        "YOUTUBE_CLIENT_SECRET": "<your-oauth-client-secret>",
        "YOUTUBE_REFRESH_TOKEN": "<your-refresh-token>"
      }
    }
  }
}
```

</details>

## Authentication

YouTube Analytics is private, per-channel data, so there is no API-key path. The CLI authenticates with an OAuth 2.0 refresh token. You give it a client ID, a client secret, and a refresh token, and it trades the refresh token for a short-lived access token on every run. There is no browser step at runtime and no token file to keep track of.

**Recommended: environment variables.** Create an OAuth 2.0 client in a Google Cloud project, then export three variables:

```bash
export YOUTUBE_CLIENT_ID=<your-oauth-client-id>
export YOUTUBE_CLIENT_SECRET=<your-oauth-client-secret>
export YOUTUBE_REFRESH_TOKEN=<your-refresh-token>
```

Scope the refresh token to `https://www.googleapis.com/auth/yt-analytics.readonly`. Add `https://www.googleapis.com/auth/yt-analytics-monetary.readonly` if you want revenue metrics.

**Interactive alternative: `auth login`.** If you would rather not mint a refresh token by hand, run the browser OAuth flow and let the CLI store the tokens in its config file:

```bash
youtube-analytics-pp-cli auth login --client-id <id> --client-secret <secret>
```

The auth subcommands:

- `youtube-analytics-pp-cli auth setup` prints the steps for registering a Google Cloud OAuth app (add `--launch` to open the page).
- `youtube-analytics-pp-cli auth login --client-id <id> --client-secret <secret>` runs the browser OAuth flow and saves the tokens.
- `youtube-analytics-pp-cli auth status` shows the current credential state.
- `youtube-analytics-pp-cli auth logout` clears the stored tokens.

`YOUTUBE_ANALYTICS_OAUTH2C` (a raw, pre-minted access token) is also accepted as a fallback, but it is not the path to use day to day. It expires fast and the CLI cannot refresh it.

## Quick Start

```bash
# Confirm credentials are loaded and the API is reachable before anything else.
youtube-analytics-pp-cli doctor


# Register a channel under a friendly name so later commands take --channel ScrollCore.
youtube-analytics-pp-cli channels add --name ScrollCore --channel-id UCxxxxxxxxxxxxxxxxxxxxxx


# Build the local archive. Every trend and comparison reads from it.
youtube-analytics-pp-cli sync --channel ScrollCore


# The headline answer: last week's numbers, with the change vs the prior week.
youtube-analytics-pp-cli report --channel ScrollCore --last 7d


# All registered channels side by side, sorted by revenue.
youtube-analytics-pp-cli rollup --last 28d --sort revenue

```

## Unique Features

No other tool for this API does these.

### Local archive that compounds
- **`sync`** pulls your daily YouTube Analytics rows into a local SQLite archive, so trends and comparisons work offline.

  _Run this first. Every report, comparison, and anomaly command reads the archive it builds._

  ```bash
  youtube-analytics-pp-cli sync --channel ScrollCore
  ```
- **`report`** shows views, watch time, subscribers, and revenue for a period, with the prior equal period and the change next to each metric.

  _The fastest way to see how a channel did and whether it is up or down. The deltas are already computed._

  ```bash
  youtube-analytics-pp-cli report --channel ScrollCore --last 7d
  ```
- **`whatchanged`** compares two periods across every synced dimension and ranks the biggest movers: traffic sources, videos, revenue.

  _Tells you whether a dip is real and what moved, without diffing two dashboards by eye._

  ```bash
  youtube-analytics-pp-cli whatchanged --channel ScrollCore --period last-week
  ```
- **`anomaly`** flags any metric in the latest finalized period that falls outside its trailing mean and standard deviation.

  _Catches an RPM or view collapse early, before it eats a month of income._

  ```bash
  youtube-analytics-pp-cli anomaly --channel ScrollCore
  ```
- **`rpm`** reports monthly RPM, CPM, and monetized playbacks as a stable series, after the trailing re-sync settles late revenue adjustments.

  _Answers "what was my RPM in February" with a number that has stopped moving._

  ```bash
  youtube-analytics-pp-cli rpm --channel ScrollCore --by month
  ```
- **`movers`** ranks videos by week-over-week change in views, showing both the ones gaining and the ones fading.

  _Tells you which videos have momentum, not just which ones are biggest._

  ```bash
  youtube-analytics-pp-cli movers --channel ScrollCore --period last-week
  ```

### Multi-channel command
- **`rollup`** shows every registered channel side by side for one period, one row per channel, sorted by views or revenue.

  _Replaces logging into YouTube Studio once per channel. The whole portfolio in one table._

  ```bash
  youtube-analytics-pp-cli rollup --last 28d --sort revenue
  ```
- **`channels`** maps friendly names like ScrollCore to channel IDs, so every command can take `--channel` by name.

  _Register a channel once, then use its name instead of a UC... ID everywhere._

  ```bash
  youtube-analytics-pp-cli channels list
  ```

### Agent-native plumbing
- **`sql`** runs read-only SQL straight against the local analytics archive, for any aggregation the presets do not cover.

  _Reach for this when a question does not match a preset. Arbitrary slicing, no extra API call._

  ```bash
  youtube-analytics-pp-cli sql "SELECT day, views FROM report_rows ORDER BY day DESC LIMIT 7"
  ```

## Usage

Run `youtube-analytics-pp-cli --help` for the full command reference and flag list.

## Commands

### group-items

Manage group items

- **`youtube-analytics-pp-cli group-items delete`** - Removes an item from a group.
- **`youtube-analytics-pp-cli group-items insert`** - Creates a group item.
- **`youtube-analytics-pp-cli group-items list`** - Returns a collection of group items that match the API request parameters.

### groups

Manage groups

- **`youtube-analytics-pp-cli groups delete`** - Deletes a group.
- **`youtube-analytics-pp-cli groups insert`** - Creates a group.
- **`youtube-analytics-pp-cli groups list`** - Returns a collection of groups that match the API request parameters. For example, you can retrieve all groups that the authenticated user owns, or you can retrieve one or more groups by their unique IDs.
- **`youtube-analytics-pp-cli groups update`** - Modifies a group. For example, you could change a group's title.

### reports

Manage reports

- **`youtube-analytics-pp-cli reports`** - Retrieve your YouTube Analytics reports.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
youtube-analytics-pp-cli group-items list

# JSON for scripting and agents
youtube-analytics-pp-cli group-items list --json

# Filter to specific fields
youtube-analytics-pp-cli group-items list --json --select id,name,status

# Dry run: show the request without sending it
youtube-analytics-pp-cli group-items list --dry-run

# Agent mode: JSON, compact, and no prompts in one flag
youtube-analytics-pp-cli group-items list --agent
```

## Agent Usage

This CLI is built for AI agents to call:

- **Non-interactive**: it never prompts; every input is a flag.
- **Pipeable**: `--json` output to stdout, errors to stderr.
- **Filterable**: `--select id,name` returns only the fields you ask for.
- **Previewable**: `--dry-run` shows the request without sending it.
- **Explicit retries**: add `--idempotent` to create retries and `--ignore-missing` to delete retries when a no-op should count as success.
- **Confirmable**: `--yes` confirms a destructive action explicitly.
- **Offline-friendly**: sync and search commands use the local SQLite store when it is available.
- **Agent-safe by default**: no colors or formatting unless you pass `--human-friendly`.

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
youtube-analytics-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/youtube-analytics-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Required | Description |
| --- | --- | --- |
| `YOUTUBE_CLIENT_ID` | Yes (recommended path) | OAuth 2.0 client ID from your Google Cloud project. |
| `YOUTUBE_CLIENT_SECRET` | Yes (recommended path) | OAuth 2.0 client secret paired with the client ID. |
| `YOUTUBE_REFRESH_TOKEN` | Yes (recommended path) | Refresh token scoped `yt-analytics.readonly` (add `yt-analytics-monetary.readonly` for revenue). The CLI exchanges it for an access token automatically. |
| `YOUTUBE_ACCESS_TOKEN` | No | Pre-minted short-lived access token; used as-is when present. |
| `YOUTUBE_ANALYTICS_OAUTH2C` | No (advanced fallback) | A raw pre-minted access token. Not the recommended path: it expires fast and cannot be refreshed. |

Instead of environment variables you can run `youtube-analytics-pp-cli auth login --client-id <id> --client-secret <secret>` once; the CLI stores the resulting tokens in `config.toml`.

## Troubleshooting

**Authentication errors (exit code 4)**
- Run `youtube-analytics-pp-cli doctor` to check credentials.
- Verify the OAuth variables are set: `echo $YOUTUBE_CLIENT_ID $YOUTUBE_CLIENT_SECRET`, or run `youtube-analytics-pp-cli auth status`.
- If you have not set up credentials yet, run `youtube-analytics-pp-cli auth login --client-id <id> --client-secret <secret>`.

**Not found errors (exit code 3)**
- Check that the resource ID is correct.
- Run the `list` command to see available items.

### API-specific

- **Revenue metrics return 403 insufficient scope.** Re-issue the refresh token with the `yt-analytics-monetary.readonly` scope, not just `yt-analytics.readonly`.
- **The last 2-3 days show zero or are missing.** This is expected. YouTube finalizes data on a 2-3 day lag, so date presets like `--last 7d` already end the window 3 days back. Pass `--include-partial` to override that.
- **HTTP 400 with an unknown metric or dimension error.** That metric and dimension combination is not a valid report. Use a named preset, or check that the metric is allowed with that dimension.
- **401 Unauthorized on every call.** The refresh token is expired or revoked. Generate a new one in the Google Cloud console and update `YOUTUBE_REFRESH_TOKEN`.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**googleapis/google-api-go-client**](https://github.com/googleapis/google-api-go-client) (Go, 4200 stars)
- [**dogfrogfog/youtube-analytics-mcp**](https://github.com/dogfrogfog/youtube-analytics-mcp) (TypeScript, 3 stars)

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press).
