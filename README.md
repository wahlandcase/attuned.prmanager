# attpr

TUI for creating and managing GitHub release PRs across multiple repositories.

## Install

### Prerequisites

- [Go 1.22+](https://go.dev/doc/install)
- [GitHub CLI](https://cli.github.com/) (`gh`) authenticated

### Quick Install

```bash
go install github.com/wahlandcase/attuned.prmanager/cmd/attpr@latest
```

This installs to `$GOPATH/bin` (usually `~/go/bin`). Make sure it's in your PATH.

### From Source

```bash
git clone https://github.com/wahlandcase/attuned.prmanager.git
cd attuned.prmanager
go build -o ~/.local/bin/attpr ./cmd/attpr
```

Make sure `~/.local/bin` is in your PATH.

## Usage

```bash
attpr              # Normal mode
attpr --dry-run    # Test without GitHub access
```

### Navigation

| Key | Action |
|-----|--------|
| `↑/↓` | Navigate lists |
| `←/→` | Switch columns (batch/merge views) |
| `Enter` | Select/Confirm |
| `Space` | Toggle selection |
| `Esc` | Go back |
| `q` | Quit |

## Features

- **Single PR**: Create a release PR for one repo (dev → staging or staging → main)
- **Batch PR**: Create release PRs across multiple repos at once
- **View/Merge PRs**: See open release PRs and merge them
- **Ticket Extraction**: Automatically extracts ticket IDs from commit messages

## Configuration

Config is created on first run:
- **Linux**: `~/.config/attpr.toml`
- **macOS**: `~/Library/Application Support/attpr.toml`

```toml
[paths]
# Parent directory containing your repositories
attuned_dir = "~/Programming/my-org"

# Glob patterns for discovering repos (relative to attuned_dir)
frontend_glob = "frontend/*"
backend_glob = "backend/*"

[tickets]
# Regex pattern for extracting ticket IDs from commits
pattern = "PROJ-[0-9]+"
```

### Example Directory Structure

```
~/Programming/my-org/
├── frontend/
│   ├── web-app/
│   ├── mobile-app/
│   └── admin-portal/
└── backend/
    ├── api-service/
    ├── auth-service/
    └── worker-service/
```

With the default globs, attpr will discover all repos under `frontend/*` and `backend/*`.
