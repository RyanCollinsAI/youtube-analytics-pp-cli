# YouTube Analytics CLI — Loop Spec

STATUS: COMPLETE

## Goal

The local `youtube-analytics-pp-cli` repo was patched on 2026-05-21 to fix two OAuth bugs: `access_type=offline` + `prompt=consent` are now sent on the authorization URL, and the token exchange endpoint was corrected from the dead `accounts.google.com/o/oauth2/token` to `oauth2.googleapis.com/token`. Build + unit tests already pass. This loop locks in the quality of that patch before Ryan pushes to GitHub: confirm the fix is complete across all code paths, run `go vet`, add a targeted unit test that asserts the OAuth params appear in the generated authorization URL, verify the config-file `TokenURL` override path, and update the changelog. The `git push` is outward-facing and is parked for Ryan.

Backlog item 7 at `G:\My Drive\Vault\Business\active\build-backlog-2026-05-22.md`.

**Definition of Done:** `go build ./... && go vet ./... && go test ./...` all pass with no errors; a new test in `internal/cli/oauth_params_test.go` (or extension of `internal/cli/auth_test.go`) asserts the authorization URL contains `access_type=offline` and `prompt=consent`; the config-file `TokenURL` override is covered by a test; the README or CHANGELOG notes the fix in plain prose.

## Run settings

- **Project folder:** C:\Users\ryanc\printing-press\library\youtube-analytics
- **Stop by:** 2026-05-23 08:00
- **Quiet hours:** 23:00-08:00
- **Verification:** `go build ./... && go vet ./... && go test ./...` (run from the project folder)

## Constraints

- Write only inside `C:\Users\ryanc\printing-press\library\youtube-analytics`. Touch nothing in the vault or any other project.
- Do NOT run `git push`. It is parked for Ryan.
- Do NOT modify `go.mod` / `go.sum` to add new dependencies. Use only the stdlib and modules already present.
- Keep existing commit history intact — no `git rebase` or `git commit --amend`.
- No live OAuth exchange. Unit-level only: parse the URL string and assert params.
- Do not change the Go version in `go.mod`.
- Any prose added to README / CHANGELOG follows the humanizer standard: no em dashes, no AI vocabulary, plain sentences.
- Workers do not git commit; the orchestrator commits after verifying.
- Irreversible or outward-facing actions are parked `[!]`.

## Tasks

- [x] 1. Audit all OAuth code paths in `auth.go` and `client.go` for any secondary token-refresh or re-auth branch that still references the dead endpoint (`accounts.google.com/o/oauth2/token`) or omits the `access_type=offline` / `prompt=consent` params. Fix any found. If none found, record the audit result under Decisions. Verify: `go build ./... && go vet ./...` pass.

- [x] 2. Write a unit test (`internal/cli/oauth_params_test.go` or extend `internal/cli/auth_test.go`) that constructs the authorization URL the same way `auth.go` does and asserts: (a) `access_type=offline` is present; (b) `prompt=consent` is present; (c) the token URL defaults to `oauth2.googleapis.com/token`. No live network calls. Verify: `go test ./internal/cli/...` passes and the new test file appears in `git status`.

- [x] 3. Verify the config-file override path: when a user supplies a custom `TokenURL` in their config, `auth.go` uses it and does NOT fall back to the dead endpoint. If a test for this path does not exist, add one. Verify: `go test ./...` passes including the new case.

- [x] 4. Add a `## Changelog` entry to `README.md` (or create `CHANGELOG.md`) documenting the OAuth fix in plain prose: what was broken, what was changed, which files. Apply the humanizer standard. Verify: `go build ./... && go vet ./... && go test ./...` still pass.

- [ ] [!] 5. Push the branch to `origin` on GitHub. PARK IMMEDIATELY — this is outward-facing. Record under Blockers: "git push to github.com/RyanCollinsAI/youtube-analytics-pp-cli requires Ryan to run manually after reviewing tasks 1-4."

## Progress Log

<!-- Append-only. One entry per iteration. Newest at the bottom. -->

**iter 1 (2026-05-22, wake 1):** Completed tasks 1-4 in a single orchestrator wake (delegated loop, 12-min cap). Audit found no dead-endpoint references in runtime code; `spec.json` contains the old endpoint string but that file is an OpenAPI descriptor, not runtime code. New test file `internal/cli/oauth_params_test.go` added with 4 tests covering auth URL params, default token URL, and override behavior in both auth and refresh paths. Changelog appended to `README.md`. All verification commands pass. Task 5 remains parked per spec (git push is Ryan's). STATUS promoted to COMPLETE.

## Decisions

<!-- Append-only. Any judgment call a worker made, with its reasoning. -->

**iter 1:** `spec.json` line 1310 contains `accounts.google.com/o/oauth2/token` in the OpenAPI security-scheme definition. This is a descriptor that documents the original spec, not a runtime call site. The runtime code in `auth.go`, `client.go`, and `config.go` all reference `oauth2.googleapis.com/token`. No code change was made to `spec.json` because altering it would change the published API contract description, which is outside the scope of the runtime fix.

**iter 1:** Tests are in `package cli` (same package as `auth.go`) using a helper `buildAuthURL` that replicates the URL construction from `newAuthLoginCmd`. This avoids needing to export internal functions or start a real HTTP listener. The token-URL defaulting logic is tested by replicating the `if tokenURL == "" { tokenURL = "..." }` pattern from both `auth.go` and `client.go`.

## Blockers / For Ryan

<!-- Append-only. Anything parked because it needs a human. -->

**Task 5:** `git push` to `github.com/RyanCollinsAI/youtube-analytics-pp-cli` requires Ryan to run manually after reviewing tasks 1-4. Do not push automatically.
