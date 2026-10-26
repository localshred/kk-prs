# kk-prs usage

kk-prs is a [koolkit](https://github.com/localshred/koolkit) plugin. Install
it with `kk install prs`; once installed it runs as `kk prs`. It shells out
to the [GitHub CLI](https://cli.github.com/) (`gh`), so `gh` must be on your
`PATH` and authenticated.

## Default: list your open PRs

```
kk prs
```

Fetches the current `gh` user, then lists open PRs authored by that user,
sorted by creation date. Output is a table with one row per PR: a linked
`owner/repo#number`, the PR's age, an overall check-status emoji, and the
title.

```
🔍 Fetching open PRs for @bj...

📋 Found 2 open PR(s):
localshred/kk-prs#7   3d   ✅   Add watch subcommand
localshred/kk-prs#5   1w   🔄   Fix render sort order
```

The overall status emoji summarizes that PR's checks (Snyk checks are
ignored):

- `✅` all checks passing
- `❌` at least one check failed
- `🔄` at least one check still pending/running
- blank when there are no checks

If there are no open PRs, kk-prs prints `📭 No open PRs found authored by
you.` instead of a table.

## `--reviews` / `-r`: PRs awaiting your review

```
kk prs --reviews
```

Lists open PRs where you're requested as a reviewer, sorted by creation
date. Each row is a linked `owner/repo#number`, age, the PR author, and the
title:

```
👀 Fetching review requests for @bj...

📋 Found 1 review request(s):
localshred/kk-prs#9   2d   @teammate   Refactor snapshot diffing
```

If there are none, kk-prs prints `📭 No review requests found.`.

## `--verbose` / `-v`: expanded check detail

```
kk prs --verbose
```

Only applies to the default (authored) listing, not `--reviews`. Instead of
a compact table, each PR gets an expanded block with a linked title, repo,
age, and one line per check (sorted by status priority, then name):

```
🔀 PR #7: Add watch subcommand
   📦 localshred/kk-prs
   🕐 opened 3d ago
   📊 Checks:
   ✅ build
   🔄 test
   ❌ lint
```

Check symbols: `✅` success, `❌` error/failure, `🔄` pending/in-progress,
`⏳` queued, `⏭️` skipped, `🚫` cancelled, `❓` anything else. A PR with no
checks prints `ℹ️  No checks found` instead of the checks list.

## `watch`: track specific PRs and get notified on change

`kk prs watch` maintains a small watchlist (stored under
`$XDG_DATA_HOME/koolkit/prs-watch`, or `~/.local/share/koolkit/prs-watch` if
`XDG_DATA_HOME` is unset) and diffs each watched PR's state on every check,
firing a desktop notification when something notable changes.

### Add a PR to the watchlist

```
kk prs watch <pr-url-or-owner/repo#number>
```

Accepts either a full GitHub PR URL or the short `owner/repo#number` form,
for example:

```
kk prs watch https://github.com/localshred/kk-prs/pull/7
kk prs watch localshred/kk-prs#7
```

On success it seeds a snapshot of the PR's current state (no notification
fires for the initial add) and prints:

```
👀 now watching localshred/kk-prs#7 — Add watch subcommand
```

Adding a PR that's already watched is a no-op that prints `already watching
<key>`. Adding a PR that's already merged or closed is refused with
`<key> is already closed — not adding`.

The first successful `watch` add also installs (and loads) a macOS launchd
agent (`com.localshred.kk-prs-watch`) so `kk prs watch tick` runs
periodically in the background without you having to schedule it yourself.

### List watched PRs

```
kk prs watch
kk prs watch list
```

With no arguments, `kk prs watch` lists the watchlist (equivalent to `kk prs
watch list`). Each line shows the PR key, its last-known check summary, and
its title:

```
localshred/kk-prs#7  passing  Add watch subcommand
```

If nothing is being watched, it prints `📭 Not watching any PRs. Add one
with: kk prs watch <pr-url>`.

### Check watched PRs now

```
kk prs watch tick
```

Re-fetches every watched PR, compares the new state against the last saved
snapshot, and fires a desktop notification for each notable change:

- PR merged or closed
- new comments
- review decision changed to approved or changes requested (or, absent a
  decision change, new reviews)
- CI status changed (failing / passing / running)
- merge conflicts appeared or were resolved

Merged or closed PRs are dropped from the watchlist after their final
notification. A fetch error for one PR leaves that PR's watchlist entry and
snapshot untouched and does not fail the rest of the tick. This is the
subcommand the installed launchd agent runs on a schedule; you can also run
it manually to force an immediate check.

### Remove a PR from the watchlist

```
kk prs watch rm <pr-url-or-owner/repo#number>
```

Prints `🗑️  stopped watching <key>` on success, or `not watching <key>` if
the PR wasn't on the watchlist.
