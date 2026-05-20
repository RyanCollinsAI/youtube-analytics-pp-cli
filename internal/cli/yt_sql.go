// Copyright 2026 Ryan Collins. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// sqlForbiddenTokens are statement keywords rejected by the read-only sql command.
var sqlForbiddenTokens = []string{
	"INSERT", "UPDATE", "DELETE", "DROP", "ALTER", "CREATE", "REPLACE",
	"ATTACH", "DETACH", "PRAGMA", "VACUUM", "REINDEX", "TRIGGER", "GRANT",
}

// isReadOnlySQL reports whether q is a single read-only SELECT/WITH statement.
func isReadOnlySQL(q string) error {
	trimmed := strings.TrimSpace(q)
	trimmed = strings.TrimRight(trimmed, ";")
	if trimmed == "" {
		return fmt.Errorf("empty query")
	}
	upper := strings.ToUpper(trimmed)
	if !strings.HasPrefix(upper, "SELECT") && !strings.HasPrefix(upper, "WITH") {
		return fmt.Errorf("only SELECT/WITH queries are allowed")
	}
	// Reject any embedded mutating statement (covers `SELECT 1; DROP ...`).
	for _, tok := range sqlForbiddenTokens {
		if containsWord(upper, tok) {
			return fmt.Errorf("statement contains a non-read-only keyword %q", tok)
		}
	}
	return nil
}

// containsWord reports whether upper contains tok as a whole word.
func containsWord(upper, tok string) bool {
	idx := 0
	for {
		i := strings.Index(upper[idx:], tok)
		if i < 0 {
			return false
		}
		start := idx + i
		end := start + len(tok)
		beforeOK := start == 0 || !isWordByte(upper[start-1])
		afterOK := end == len(upper) || !isWordByte(upper[end])
		if beforeOK && afterOK {
			return true
		}
		idx = end
	}
}

func isWordByte(b byte) bool {
	return b == '_' || (b >= '0' && b <= '9') || (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z')
}

func newSQLCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:         "sql <query>",
		Short:       "Run a read-only SQL query against the local analytics archive",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: strings.Trim(`
  youtube-analytics-pp-cli sql "SELECT channel, SUM(value) FROM report_rows WHERE metric='views' GROUP BY channel"
  youtube-analytics-pp-cli sql "SELECT name FROM sqlite_master WHERE type='table'" --json`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}

			query := strings.Join(args, " ")
			if err := isReadOnlySQL(query); err != nil {
				return usageErr(err)
			}

			if dbPath == "" {
				dbPath = defaultDBPath("youtube-analytics-pp-cli")
			}
			a, err := openArchive(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local archive: %w", err)
			}
			defer a.close()

			rows, err := a.db().Query(query)
			if err != nil {
				return fmt.Errorf("query failed: %w", err)
			}
			defer rows.Close()

			cols, err := rows.Columns()
			if err != nil {
				return err
			}

			var out []map[string]any
			for rows.Next() {
				scanTargets := make([]any, len(cols))
				holders := make([]any, len(cols))
				for i := range holders {
					scanTargets[i] = &holders[i]
				}
				if err := rows.Scan(scanTargets...); err != nil {
					return err
				}
				m := make(map[string]any, len(cols))
				for i, c := range cols {
					m[c] = normalizeSQLValue(holders[i])
				}
				out = append(out, m)
			}
			if err := rows.Err(); err != nil {
				return err
			}
			if out == nil {
				out = []map[string]any{}
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/youtube-analytics-pp-cli/data.db)")

	return cmd
}

// normalizeSQLValue converts a generic SQL scan result into a JSON-friendly value.
func normalizeSQLValue(v any) any {
	switch x := v.(type) {
	case []byte:
		return string(x)
	default:
		return v
	}
}
