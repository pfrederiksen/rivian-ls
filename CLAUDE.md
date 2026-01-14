# CLAUDE.md - Developer Documentation

This document contains development workflow, architecture decisions, and implementation notes for the `rivian-ls` project.

## Development Workflow

### Branch Strategy

**IMPORTANT**: Always work on feature branches, never commit directly to `main`.

Branch naming conventions:
- `feat/...` - New features (e.g., `feat/auth`, `feat/tui-live`)
- `fix/...` - Bug fixes (e.g., `fix/websocket-reconnect`)
- `chore/...` - Maintenance tasks (e.g., `chore/ci`, `chore/deps`)
- `docs/...` - Documentation updates

### Development Commands

```bash
# Build the binary
make build

# Run tests
make test

# Run tests with coverage report (generates coverage.html)
make coverage

# Run linter
make lint

# Format code
make fmt

# Run all checks (fmt + vet + lint + test)
make check

# Clean build artifacts
make clean

# Install dev tools (golangci-lint)
make install-tools
```

### CI/CD Pipeline

GitHub Actions runs on every push and PR to `main`:

1. **Lint**: `golangci-lint` with strict settings
2. **Test**: Full test suite with race detector
3. **Coverage Gate**: Minimum 80% coverage required (blocking)
4. **Build**: Compile binary and verify it runs

**Coverage threshold**: The CI will **fail** if coverage drops below 80%. Aim for 90%+ where reasonable.

### Running Locally

```bash
# Build and run
make build
./rivian-ls

# Or use Make directly
make run

# Run with verbose logging
./rivian-ls --verbose

# Run specific command
./rivian-ls auth
./rivian-ls status --format json
./rivian-ls watch
```

## Architecture

### Package Structure

```
internal/
├── rivian/      # Rivian API client
│   ├── auth.go          # Authentication flow (login, OTP, tokens)
│   ├── graphql.go       # GraphQL queries and mutations
│   ├── websocket.go     # WebSocket subscription client
│   └── client.go        # Main client interface
├── model/       # Domain models
│   ├── vehicle.go       # Vehicle state struct
│   ├── reducer.go       # State reducer (merges API events)
│   └── insights.go      # Derived metrics (ready score, etc.)
├── tui/         # Bubble Tea TUI
│   ├── model.go         # Bubble Tea model
│   ├── dashboard.go     # Dashboard view
│   ├── charge.go        # Charge view
│   └── health.go        # Health/history view
├── cli/         # Headless CLI
│   ├── status.go        # Snapshot command
│   ├── watch.go         # Streaming command
│   ├── export.go        # Export command
│   └── format.go        # Output formatters (JSON, YAML, CSV)
└── store/       # Local persistence
    ├── store.go         # Store interface
    └── sqlite.go        # SQLite implementation
```

### Key Architectural Decisions

#### 1. **State Reducer Pattern**

Both the TUI and headless CLI use the **same** domain model and state reducer. This ensures:
- No duplicated logic
- Consistent behavior between modes
- Easier testing (test the reducer once, both modes benefit)

The reducer in `internal/model/reducer.go` takes raw API events (GraphQL queries or WebSocket messages) and produces a stable `VehicleState` struct.

#### 2. **Interface-Based Design**

All external dependencies (API client, storage) are defined as interfaces:

```go
// internal/rivian/client.go
type Client interface {
    Authenticate(email, password string) error
    GetVehicles() ([]Vehicle, error)
    GetVehicleState(vehicleID string) (*VehicleState, error)
    SubscribeToUpdates(vehicleID string) (<-chan Update, error)
}

// internal/store/store.go
type Store interface {
    SaveSnapshot(snapshot *model.VehicleState) error
    GetSnapshots(since time.Time, limit int) ([]model.VehicleState, error)
}
```

This makes testing easy: mock the interfaces, test the logic.

#### 3. **WebSocket with Polling Fallback**

The app prefers WebSocket subscriptions for live updates but gracefully degrades to polling if:
- WebSocket connection fails
- User is behind a restrictive firewall
- The API endpoint changes

Polling interval defaults to 30s but is configurable via `--interval`.

#### 4. **Secure Credential Storage**

Credentials and tokens are **never** hardcoded or logged. Storage strategy:

1. **Preferred**: OS keychain (macOS Keychain, Linux Secret Service, Windows Credential Manager)
2. **Fallback**: Encrypted file with passphrase
3. **CLI flag**: `--no-store` to disable persistence entirely

Implementation in `internal/rivian/auth.go`.

## Unofficial Rivian API

### API Endpoints

Based on community reverse-engineering, the Rivian mobile app uses:

- **GraphQL HTTPS**: `https://api.rivian.com/graphql` (or similar)
- **WebSocket**: `wss://api.rivian.com/subscriptions` (or similar)

**WARNING**: These endpoints are **not officially documented** and may change at any time.

### Authentication Flow

1. **Establish session**: GET request to establish CSRF token
2. **Login mutation**: POST GraphQL mutation with email/password
3. **OTP flow** (if enabled): Prompt user for OTP, submit via mutation
4. **Token storage**: Store access token + refresh token securely
5. **Token refresh**: Automatically refresh when access token expires

See `internal/rivian/auth.go` for implementation.

### GraphQL Queries

Key queries needed:

```graphql
query GetVehicles {
  currentUser {
    vehicles {
      id
      vin
      name
      model
    }
  }
}

query GetVehicleState($vehicleId: ID!) {
  vehicleState(id: $vehicleId) {
    batteryLevel
    chargeState
    rangeEstimate
    # ... etc
  }
}
```

### WebSocket Subscriptions

The app subscribes to:

1. **Vehicle state updates**: Battery, charging, range
2. **Tire pressure updates**: Individual tire pressures
3. **Closures**: Doors, frunk, liftgate, windows

Example subscription:

```graphql
subscription VehicleStateUpdates($vehicleId: ID!) {
  vehicleState(id: $vehicleId) {
    # Same fields as query
  }
}
```

### Known Caveats

- **Rate limiting**: Unknown. Be conservative with polling intervals.
- **Breaking changes**: The API can change without notice.
- **Terms of Service**: Using this API may violate Rivian's ToS.
- **MFA/OTP**: Some accounts require OTP. The app handles this interactively.

## Testing Strategy

### Unit Tests

- **API client**: Mock HTTP responses, test auth flow and query parsing
- **State reducer**: Test that raw API events correctly update domain model
- **Formatters**: Test JSON/CSV/YAML output formatting
- **Insights**: Test derived metrics (ready score, etc.)

### Integration Tests

- Use `httptest` to simulate the Rivian API
- Test full flow: auth → vehicle list → state query → WebSocket updates

### Coverage Goals

- **Minimum**: 80% (enforced by CI)
- **Target**: 90%+
- **Focus areas**: Critical paths (auth, state reducer, API client)

### Running Tests

```bash
# Run all tests
make test

# Run with coverage
make coverage

# Run specific package
go test -v ./internal/rivian/...

# Run specific test
go test -v -run TestAuthentication ./internal/rivian/
```

## Adding New Features

### Checklist

When adding a new feature:

1. ✅ Create a feature branch (`feat/...`)
2. ✅ Write tests FIRST (TDD where possible)
3. ✅ Implement the feature
4. ✅ Run `make check` locally
5. ✅ Update documentation (README.md, CLAUDE.md, or docs/)
6. ✅ Commit with clear message
7. ✅ Open PR to `main`
8. ✅ Ensure CI passes

### Example: Adding a New Derived Metric

1. Add the field to `internal/model/vehicle.go`
2. Implement calculation in `internal/model/insights.go`
3. Write tests in `internal/model/insights_test.go`
4. Update TUI view to display it (`internal/tui/dashboard.go`)
5. Update headless CLI formatters (`internal/cli/format.go`)
6. Update README.md with new metric description

## Common Issues

### Linter Errors

If `golangci-lint` fails:

```bash
# See what's wrong
make lint

# Auto-fix formatting issues
make fmt

# Check specific linters
golangci-lint run --disable-all --enable=errcheck
```

### Coverage Below Threshold

If CI fails on coverage:

```bash
# Generate coverage report
make coverage

# Open coverage.html in browser to see untested lines
open coverage.html
```

Focus on testing critical paths first (auth, API client, state reducer).

### WebSocket Debugging

Enable verbose logging to debug WebSocket issues:

```bash
./rivian-ls --verbose run
```

Look for:
- Connection establishment
- Subscription messages
- Reconnection attempts
- Fallback to polling

## Future Enhancements

Ideas for future work:

- [ ] **Multi-vehicle support**: Display multiple vehicles in TUI
- [ ] **Historical charts**: Graph battery/range over time (using local snapshots)
- [ ] **Notifications**: Alert on charge complete, low battery, etc.
- [ ] **Remote commands**: Lock/unlock, honk, flash lights (if API supports)
- [ ] **Config wizard**: Interactive setup command
- [ ] **Prometheus exporter**: Expose metrics for monitoring systems
- [ ] **Docker image**: Containerized deployment

## Resources

- [Bubble Tea Documentation](https://github.com/charmbracelet/bubbletea)
- [Bubbles Component Library](https://github.com/charmbracelet/bubbles)
- [Lip Gloss Styling](https://github.com/charmbracelet/lipgloss)
- [golangci-lint](https://golangci-lint.run/)

## Contact

For questions or contributions, please open an issue on GitHub.
