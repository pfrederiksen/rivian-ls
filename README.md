# rivian-ls

A production-quality terminal UI (TUI) and headless CLI tool for monitoring Rivian vehicle telemetry in real-time.

> ⚠️ **WARNING**: This tool uses an **unofficial** Rivian API that is not publicly documented or supported. Using this tool may violate Rivian's Terms of Service. Use at your own risk. The API may change or break at any time without notice.

## Features

- 🚗 **Real-time vehicle monitoring** via GraphQL + WebSocket subscriptions
- 📊 **Interactive TUI** powered by Bubble Tea with multiple views
- 🤖 **Headless CLI mode** for scripting and automation
- 💾 **Local persistence** for historical data and analysis
- 🔐 **Secure credential storage** with OS keychain integration
- 📈 **Derived insights**: charging narratives, readiness score, tire drift tracking

## Installation

### Prerequisites

- Go 1.21 or later
- A Rivian account with at least one vehicle

### From Source

```bash
git clone https://github.com/pfrederiksen/rivian-ls.git
cd rivian-ls
make build
```

The binary will be built as `./rivian-ls`.

## Quick Start

### Authentication

First, authenticate with your Rivian account:

```bash
rivian-ls auth
```

You'll be prompted for your email and password. If MFA/OTP is enabled, you'll be prompted for the code. Credentials are stored securely in your OS keychain.

### TUI Mode

Launch the interactive terminal UI:

```bash
rivian-ls run
```

**Navigation:**
- Use `Tab` or number keys to switch between views
- Press `q` to quit
- Press `?` for help

**Views:**
1. **Dashboard**: Battery, range, charging status, locks, closures, cabin temp, tire pressures, ready score
2. **Charge**: Detailed charging session info and history
3. **Health/History**: Tire pressure trends and vehicle timeline

### Headless CLI Mode

The headless mode is designed for scripting, automation, and piping data to other tools.

#### Get a snapshot

```bash
# Human-readable output (default)
rivian-ls status

# JSON output for scripting
rivian-ls status --format json

# YAML output
rivian-ls status --format yaml
```

#### Stream live updates

```bash
# Stream updates continuously (like `tail -f`)
rivian-ls watch

# JSON Lines output (one JSON object per line)
rivian-ls watch --format jsonlines

# Auto-exit after 5 minutes
rivian-ls watch --duration 5m

# Polling fallback with custom interval
rivian-ls watch --interval 30s
```

#### Export historical data

```bash
# Export last 24 hours as JSON
rivian-ls export --format json > vehicle-history.json

# Export last 100 records as CSV
rivian-ls export --limit 100 --format csv > history.csv
```

#### Common Options

- `--vehicle <name|VIN|id>`: Select specific vehicle
- `--format <human|json|jsonlines|csv>`: Output format
- `--no-store`: Don't persist snapshots locally
- `--quiet`: Suppress logs
- `--verbose`: Enable debug logging

#### Exit Codes

- `0`: Success
- `1`: Authentication failure
- `2`: Vehicle not found
- `3`: API unreachable or error
- `4`: Invalid arguments

## Configuration

Configuration can be provided via environment variables or a config file.

### Environment Variables

```bash
export RIVIAN_EMAIL="your.email@example.com"
export RIVIAN_PASSWORD="your-password"
export RIVIAN_TOKEN_STORAGE="/custom/path/to/tokens"
export RIVIAN_POLL_INTERVAL="30s"
```

### Config File

Create `~/.config/rivian-ls/config.yaml`:

```yaml
email: your.email@example.com
# password: leave empty to be prompted interactively (recommended)
token_storage: /custom/path/to/tokens
poll_interval: 30s
```

## Development

See [CLAUDE.md](CLAUDE.md) for development workflow, testing, and architecture details.

## Architecture

```
rivian-ls/
├── cmd/rivian-ls/          # CLI entrypoint
├── internal/
│   ├── rivian/            # GraphQL + WebSocket client for Rivian API
│   ├── model/             # Domain models and state reducer
│   ├── tui/               # Bubble Tea TUI implementation
│   ├── cli/               # Headless CLI formatters and commands
│   └── store/             # Local snapshot persistence (SQLite/BoltDB)
├── docs/                  # Additional documentation
└── .github/workflows/     # CI/CD pipelines
```

## Security & Privacy

- **Credentials**: Never hardcoded. Stored in OS keychain when possible, encrypted at rest otherwise.
- **Tokens**: Access/refresh tokens are stored securely and refreshed automatically.
- **Data**: Vehicle telemetry snapshots are stored locally only (not sent to third parties).
- **Privacy**: Use `--no-store` flag to disable local persistence entirely.

## Troubleshooting

### Authentication fails

- Ensure your email and password are correct
- If you have MFA enabled, ensure you enter the correct OTP code
- Check that you can log in via the official Rivian mobile app

### WebSocket connection fails

- The app will automatically fall back to polling
- Check your network/firewall settings
- Try increasing `--interval` for longer polling periods

### "Vehicle not found"

- Ensure you have at least one vehicle registered in your Rivian account
- Try using `--vehicle <VIN>` to select a specific vehicle

## Contributing

Contributions are welcome! Please:

1. Fork the repository
2. Create a feature branch (`feat/...`, `fix/...`, `chore/...`)
3. Write tests for new functionality
4. Ensure `make check` passes (linting + tests + coverage threshold)
5. Update documentation as needed
6. Open a pull request against `main`

## License

MIT License - see LICENSE file for details.

## Disclaimer

This project is not affiliated with, endorsed by, or connected to Rivian Automotive, LLC. All product names, logos, and brands are property of their respective owners.
