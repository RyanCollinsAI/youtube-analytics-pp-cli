---
name: pp-youtube-analytics
description: "YouTube Analytics CLI with a local archive: period-over-period deltas, multi-channel rollups, RPM tracking, and anomaly detection. Trigger phrases: `how did ScrollCore do last week`, `youtube revenue this month`, `top performing youtube video this month`, `check my youtube channel analytics`, `youtube rpm trend`, `use youtube-analytics`, `run youtube-analytics`."
author: "RyanCollinsAI"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - youtube-analytics-pp-cli
---

# YouTube Analytics CLI

## Prerequisites: Install the CLI

This skill drives the `youtube-analytics-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install youtube-analytics --cli-only
   ```
2. Verify: `youtube-analytics-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

The YouTube Analytics API is really one endpoint, `reports.query`, and it answers with a column-array response that is awkward to read. This CLI wraps that endpoint in named reports (overview, top-videos, traffic, retention, revenue, demographics) and prints clean labeled tables instead. It also syncs your daily rows into a local SQLite database. That local copy is what makes period deltas, whatchanged diffs, anomaly checks, and multi-channel rollups work without re-querying the API every time.

## When to Use This CLI

Use this CLI when an agent or operator needs YouTube channel performance numbers (views, watch time, revenue, retention, traffic sources, top videos) without opening Studio or parsing raw API JSON. It is a good fit for recurring questions like "how did this channel do last week", "what is my RPM trend", or "which videos are rising", because the local archive answers them with the deltas already computed. It does not manage content: it reads analytics, it does not edit videos.

## Unique Capabilities

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

## Command Reference

**group-items** commands (manage group items):

- `youtube-analytics-pp-cli group-items delete` removes an item from a group.
- `youtube-analytics-pp-cli group-items insert` creates a group item.
- `youtube-analytics-pp-cli group-items list` returns the group items that match the API request parameters.

**groups** commands (manage groups):

- `youtube-analytics-pp-cli groups delete` deletes a group.
- `youtube-analytics-pp-cli groups insert` creates a group.
- `youtube-analytics-pp-cli groups list` returns the groups that match the API request parameters.
- `youtube-analytics-pp-cli groups update` modifies a group, for example to change its title.

**reports** command (query reports):

- `youtube-analytics-pp-cli reports` retrieves your YouTube Analytics reports.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
youtube-analytics-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match, so fall back to `--help` or use a narrower query.

## Recipes


### Morning multi-channel check

```bash
youtube-analytics-pp-cli sync && youtube-analytics-pp-cli rollup --last 7d
```

Refresh the archive for every channel, then print the whole portfolio side by side.

### Is the dip real

```bash
youtube-analytics-pp-cli whatchanged --channel ScrollCore --period last-week
```

Compare last week against the week before across every dimension and rank what moved.

### Top videos for an agent

```bash
youtube-analytics-pp-cli top-videos --channel ScrollCore --last 28d --sort revenue --agent --select video,views,estimatedRevenue
```

Compact, field-filtered output an agent can read without burning context on full rows.

### Settled monthly RPM

```bash
youtube-analytics-pp-cli rpm --channel ScrollCore --by month
```

Monthly RPM after the trailing re-sync absorbs late revenue adjustments.

### Off-preset aggregation

```bash
youtube-analytics-pp-cli sql "SELECT day, views, estimatedRevenue FROM report_rows WHERE channel='ScrollCore' ORDER BY day DESC LIMIT 14"
```

Drop to SQL when no preset matches the question.

## Auth Setup

YouTube Analytics is private, per-channel data, so there is no API-key path. The CLI authenticates with an OAuth 2.0 refresh token: you set `YOUTUBE_CLIENT_ID`, `YOUTUBE_CLIENT_SECRET`, and `YOUTUBE_REFRESH_TOKEN` from a Google Cloud project, and the CLI trades the refresh token for a short-lived access token on every run. There is no browser step at runtime and no token file to keep track of.

Recommended path: export three environment variables from a Google Cloud OAuth 2.0 client.

```bash
export YOUTUBE_CLIENT_ID=<your-oauth-client-id>
export YOUTUBE_CLIENT_SECRET=<your-oauth-client-secret>
export YOUTUBE_REFRESH_TOKEN=<your-refresh-token>
```

Scope the refresh token to `https://www.googleapis.com/auth/yt-analytics.readonly`; add `yt-analytics-monetary.readonly` for revenue metrics.

The auth subcommands:

- `youtube-analytics-pp-cli auth setup` prints the OAuth-app registration steps (`--launch` opens the page).
- `youtube-analytics-pp-cli auth login --client-id <id> --client-secret <secret>` runs the interactive browser OAuth flow and stores tokens in the config file.
- `youtube-analytics-pp-cli auth status` shows the current credential state.
- `youtube-analytics-pp-cli auth logout` clears the stored tokens.

`YOUTUBE_ANALYTICS_OAUTH2C` (a raw pre-minted access token) is accepted as a fallback only. It expires fast and cannot be refreshed.

Run `youtube-analytics-pp-cli doctor` to verify the setup.

## Do Not Use For

This CLI reads YouTube Analytics only. It does not:

- upload, edit, or delete videos
- manage, post, or moderate comments
- change channel, playlist, or video settings

It is not the YouTube Data API. For any video or comment change, use the YouTube Data API instead.

## Agent Mode

Add `--agent` to any command. It expands to `--json --compact --no-input --no-color --yes`.

- **Pipeable**: JSON on stdout, errors on stderr.
- **Filterable**: `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. This keeps context small on verbose APIs:

  ```bash
  youtube-analytics-pp-cli group-items list --agent --select id,name,status
  ```
- **Previewable**: `--dry-run` shows the request without sending it.
- **Offline-friendly**: sync and search commands use the local SQLite store when it is available.
- **Non-interactive**: it never prompts; every input is a flag.
- **Explicit retries**: use `--idempotent` only when an already-existing create should count as success, and `--ignore-missing` only when a missing delete target should count as success.

### Response envelope

Commands that read from the local store or the API wrap output in a provenance envelope:

```json
{
  "meta": {"source": "live" | "local", "synced_at": "...", "reason": "..."},
  "results": <data>
}
```

Parse `.results` for data and `.meta.source` to know whether it is live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal and no machine-format flag (`--json`, `--csv`, `--compact`, `--quiet`, `--plain`, `--select`) is set. Piped and agent consumers, and explicit-format runs, get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
youtube-analytics-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
youtube-analytics-pp-cli feedback --stdin < notes.txt
youtube-analytics-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.youtube-analytics-pp-cli/feedback.jsonl`. They are never POSTed unless `YOUTUBE_ANALYTICS_FEEDBACK_ENDPOINT` is set and either `--send` is passed or `YOUTUBE_ANALYTICS_FEEDBACK_AUTO_SEND=true`. The default is local-only.

Write what *surprised* you, not a bug report. Short, specific, one line: that is the part that compounds.

## Output Delivery

Every command accepts `--deliver <sink>`. The output goes to the named sink in addition to (or instead of) stdout, so agents can route command results without hand-piping. Three sinks are supported:

| Sink | Effect |
|------|--------|
| `stdout` | Default; write to stdout only |
| `file:<path>` | Atomically write output to `<path>` (tmp + rename) |
| `webhook:<url>` | POST the output body to the URL (`application/json`, or `application/x-ndjson` when `--compact`) |

Unknown schemes are refused with a structured error naming the supported set. Webhook failures return non-zero and log the URL and HTTP status on stderr.

## Named Profiles

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled agent calls the same command every run with the same configuration.

```
youtube-analytics-pp-cli profile save briefing --json
youtube-analytics-pp-cli --profile briefing group-items list
youtube-analytics-pp-cli profile list --json
youtube-analytics-pp-cli profile show briefing
youtube-analytics-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles`, so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 4 | Authentication required |
| 5 | API error (upstream issue) |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `youtube-analytics-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as a CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add youtube-analytics-pp-mcp -- youtube-analytics-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which youtube-analytics-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   youtube-analytics-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `youtube-analytics-pp-cli <command> --help`.
