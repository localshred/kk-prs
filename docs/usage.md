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

`kk prs list` is an explicit name for this same default; it takes the same
`--reviews` and `--verbose` flags.

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

## `released`: where did my PRs land?

```
kk prs released [--weeks N] [--author LOGIN]... [--refresh] [--verbose]
```

Prints a board of swim lanes showing, for your open PRs and the PRs you
merged recently, the earliest release each one reached. It reads release
history from kk-owned bare mirrors of each repo (see below), so it does not
touch your own clones.

```
🚢 Releases for alice (last 4 weeks)

OPEN (1)                  NOT IN A RELEASE (1)      release-A (1)
┌──────────────────────┐  ┌──────────────────────┐  cut ~Oct 24 (1d)
│svc-a#12              │  │svc-a#2               │  ┌──────────────────────┐
│WI-1234               │  │Tune timeouts         │  │svc-a#1               │
│Add caching WI-1234   │  │merged 0m             │  │Add retry logic       │
│opened 1h             │  │on main               │  │merged 0m             │
│@alice                │  │@alice                │  │@alice                │
└──────────────────────┘  └──────────────────────┘  └──────────────────────┘
```

The example above comes from `kk prs released --author alice`. Lanes, left
to right, newest card first within each lane:

- `OPEN`: your open PRs. A check glyph (`✓` passing, `✗` failing, `↻`
  running) precedes the age on the `opened <age>` line.
- `NOT IN A RELEASE`: merged PRs that have not reached any release, with a
  reason line: `on <branch>` (merged to a trunk, not yet released),
  `→ <branch> · waiting` (merged to a non-trunk base), `no releases` (the
  repo has no release branches), or `release status unknown` (the mirror
  could not be fetched; a `⚠ <repo>: <error>` warning is printed under the
  board).
- One lane per release branch, newest release first, headed with its name
  and card count, and a `cut ~<date> (<age>)` line. A PR is placed in the
  earliest release containing its merge commit, or a cherry-pick of it.
  Lanes that do not fit the terminal width are folded into a final
  `+N older releases` line.

Each card shows the linked `repo#number` (the owner is dropped when every
card shares it), an optional linked work item ID, the title, `merged <age>`
or `opened <age>`, the reason (pending lane), `in release <age>` when the
time it reached the release is known, `↳ via hotfix #N` or `↳ via
cherry-pick` when the PR arrived on a release branch some other way, and
`@author` when more than one author is shown or `--author` is not `@me`.

### Flags

- `--weeks N`: how many weeks of merged PRs to include. Default is
  `kk.prs.weeks`, else 4.
- `--author LOGIN`: whose PRs to include; repeat for several. Default is
  `@me`. Duplicates are collapsed by PR URL.
- `--refresh`: fetch every mirror even if it was fetched within the last
  five minutes.
- `--verbose` / `-v`: after the board, print the detailed check block for
  each open PR, as in `kk prs --verbose`.

The board width comes from `$COLUMNS` when set to a positive integer, else
the terminal size, else 120 columns.

### Settings

All settings are read from git config (`git config --get-all`), never from
repo files. Set them with `git config --global`.

- `kk.prs.cloneURL`: remote URL template for mirrors; `{repo}` becomes
  `owner/name`. Default `git@github.com:{repo}.git`.
- `kk.prs.weeks`: default for `--weeks`. Default `4`.
- `kk.prs.ignoreRepo`: `owner/name` of a repo to leave off the board.
  Repeatable, and each value may hold several whitespace-separated names.
  Default none.
- `kk.prs.workItemPattern`: regular expression matching a work item ID,
  such as `WI-[0-9]+`. The first match in the PR title, else the body, is
  shown on the card. An invalid expression is ignored. Default unset.
- `kk.prs.workItemURL`: link target for the work item ID; `{id}` becomes
  the matched ID, such as `https://tracker.example.com/{id}`. Both this and
  the pattern must be set for a card to show a work item. Default unset.
- `kk.trunks`: extra branch names, besides the repo's default branch,
  `main`, and `master`, treated as trunks. Repeatable. Default none.
- `kk.releasePattern`: branch globs that identify release branches.
  Repeatable. `*` does not cross `/`, and a trailing `/**` matches branches
  nested under a prefix. Default `release*` and `release*/**`.
- `kk.prs.deployedCommand`: reserved for a future feature and currently
  ignored.

### Mirrors and cache

Mirrors are bare, commit-only partial clones holding just each repo's
trunks and release branches, stored at
`$XDG_CACHE_HOME/koolkit/prs-mirrors/<owner>/<name>.git` (`~/.cache/...`
when `XDG_CACHE_HOME` is unset). A mirror is fetched at most once every five
minutes unless you pass `--refresh`, and up to four repos are opened at a
time. Because a release containing a commit never changes, the release found
for each merged PR is remembered in
`$XDG_CACHE_HOME/koolkit/prs/placements.json`. Both locations are safe to
delete; they are rebuilt on the next run.

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
