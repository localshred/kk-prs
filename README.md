# kk-prs

A [koolkit](https://github.com/localshred/koolkit) plugin that shows your open
GitHub pull requests with check statuses, lists PRs where you're requested as
a reviewer, and can watch specific PRs for changes with desktop
notifications.

## Install

```
kk install prs
```

This requires the [`kk`](https://github.com/localshred/koolkit) CLI. Once
installed, kk-prs runs as a `kk` subcommand:

```
kk prs
```

## Requirements

kk-prs shells out to the [GitHub CLI](https://cli.github.com/) (`gh`). Make
sure `gh` is installed and authenticated (`gh auth login`) before using any
kk-prs command.

## Quick usage

```
kk prs
```

Lists your open PRs with age and overall check status:

```
🔍 Fetching open PRs for @bj...

📋 Found 2 open PR(s):
localshred/kk-prs#7   3d   ✅   Add watch subcommand
localshred/kk-prs#5   1w   🔄   Fix render sort order
```

See [docs/usage.md](docs/usage.md) for the full command and flag reference,
including `--reviews`, `--verbose`, and the `watch` subcommand.
