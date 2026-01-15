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
├── rivian/      # Rivian API client (Coverage: 67.3%)
│   ├── client.go        # Client interface and types
│   ├── http_client.go   # HTTP/GraphQL implementation
│   ├── auth.go          # 3-step authentication (CSRF → Login → OTP)
│   ├── vehicles.go      # Vehicle queries and parsing
│   └── websocket.go     # WebSocket subscription client
├── model/       # Domain models (Coverage: 84.5%)
│   ├── vehicle.go       # VehicleState domain model
│   ├── reducer.go       # Redux-style event reducer
│   └── insights.go      # Derived metrics (ReadyScore, issues)
├── store/       # Local persistence (Coverage: 71.3%)
│   └── store.go         # SQLite storage with dual column+JSON strategy
├── cli/         # Headless CLI (Coverage: 57.9%)
│   ├── format.go        # Output formatters (JSON, YAML, CSV, text, table)
│   ├── status.go        # Current state snapshot command
│   ├── watch.go         # Real-time streaming command
│   └── export.go        # Historical data export command
└── tui/         # Bubble Tea TUI (Coverage: TBD)
    ├── model.go         # Bubble Tea model (Elm architecture)
    ├── dashboard.go     # Dashboard view (battery, charging, security, tires, stats)
    ├── charge.go        # Detailed charging view
    └── health.go        # Health/history view with timeline
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

**VERIFIED** endpoints (tested January 2026):

- **GraphQL HTTPS**: `https://rivian.com/api/gql/gateway/graphql`
- **WebSocket**: TBD (not yet implemented)

**WARNING**: These endpoints are **not officially documented** and may change at any time.

### Critical Implementation Details

#### Required Headers

All GraphQL requests MUST include:

```
Content-Type: application/json
User-Agent: rivian-ls/0.1.0
apollographql-client-name: com.rivian.android.consumer
```

After authentication, also include:
```
a-sess: <appSessionToken>
csrf-token: <csrfToken>
u-sess: <userSessionToken>
```

**IMPORTANT**: Use `com.rivian.android.consumer` NOT `com.rivian.ios.consumer`. The iOS client identifier returns "Entity not found" errors on certain queries.

#### Authentication Flow

The actual authentication flow (3 steps):

1. **CreateCSRFToken mutation**: Obtain CSRF token and app session token
   ```graphql
   mutation CreateCSRFToken {
     createCsrfToken {
       __typename
       csrfToken
       appSessionToken
     }
   }
   ```
   - Returns: `csrfToken` and `appSessionToken`
   - Set headers: `csrf-token` and `a-sess` for subsequent requests

2. **Login mutation**: Authenticate with email/password
   ```graphql
   mutation Login($email: String!, $password: String!) {
     login(email: $email, password: $password) {
       __typename
       ... on MobileLoginResponse {
         accessToken
         refreshToken
         userSessionToken
       }
       ... on MobileMFALoginResponse {
         otpToken
       }
     }
   }
   ```
   - Uses **union types**: Returns either `MobileLoginResponse` (success) or `MobileMFALoginResponse` (MFA required)
   - Check `__typename` to determine response type
   - If MFA required, store `otpToken` for step 3

3. **LoginWithOTP mutation** (if MFA enabled):
   ```graphql
   mutation LoginWithOTP($email: String!, $otpCode: String!, $otpToken: String!) {
     loginWithOTP(email: $email, otpCode: $otpCode, otpToken: $otpToken) {
       __typename
       accessToken
       refreshToken
       userSessionToken
     }
   }
   ```
   - Requires the original `email` from step 2 (store it!)
   - Returns same tokens as `MobileLoginResponse`

4. **Token usage**:
   - `userSessionToken` → Used in `u-sess` header (NOT accessToken!)
   - `refreshToken` → Used for token refresh
   - `accessToken` → Purpose unclear, not used in subsequent requests
   - Tokens expire in ~2 hours (access) and ~180 days (refresh)

See `internal/rivian/auth.go` for implementation.

### GraphQL Queries

#### GetVehicles Query

**VERIFIED** schema (note the nested `vehicle` object):

```graphql
query GetVehicles {
  currentUser {
    __typename
    vehicles {
      __typename
      id
      vin
      name
      vehicle {
        __typename
        model
      }
    }
  }
}
```

**Key insight**: `model` is nested inside `vehicle`, not a direct field on `vehicles`. The `vehicles` array contains user-specific data (name, VIN), while `vehicle` contains vehicle specifications.

#### GetVehicleState Query

**VERIFIED** schema - uses timestamped values pattern:

```graphql
query GetVehicleState($vehicleID: String!) {
  vehicleState(id: $vehicleID) {
    __typename
    gnssLocation {
      __typename
      latitude
      longitude
      timeStamp
    }
    batteryLevel {
      __typename
      timeStamp
      value
    }
    distanceToEmpty {
      __typename
      timeStamp
      value
    }
    chargerState {
      __typename
      timeStamp
      value
    }
    # ... many more timestamped fields
  }
}
```

**Critical schema details**:

1. **Variable name**: Use `$vehicleID` (uppercase ID), not `$vehicleId`
2. **Timestamped values**: Every sensor value is wrapped in `{__typename, timeStamp, value}`
3. **Timestamp format**: ISO 8601 strings (e.g., `"2024-01-15T10:30:00.000Z"`), NOT Unix integers
4. **Location**: `gnssLocation` has `latitude`/`longitude` directly, not nested in a `value` field
5. **Tire pressure**: API returns status strings ("OK", "low", "high"), NOT actual PSI values - Rivian doesn't expose raw sensor data
6. **Battery capacity**: Not provided by API - must be estimated from batteryLevel and distanceToEmpty using typical efficiency values (R1T: ~2.0 mi/kWh, R1S: ~2.1 mi/kWh)
7. **Odometer**: Value is in meters (multiply by 0.000621371 for miles)

**Available fields** (tested subset):
- `batteryLevel`, `batteryLimit`, `distanceToEmpty`
- `chargerState`, `timeToEndOfCharge`
- `vehicleMileage`, `cabinClimateInteriorTemperature`
- `doorFrontLeftLocked/Closed`, `doorFrontRightLocked/Closed`, etc.
- `windowFrontLeftClosed`, `windowFrontRightClosed`, etc.
- `closureFrunkLocked/Closed`, `closureLiftgateLocked/Closed`
- `closureTonneauLocked/Closed` (R1T only)
- `tirePressureStatusFrontLeft/Right/RearLeft/Right`
- `gnssLocation` (GPS coordinates)

See [RivDocs](https://rivian-api.kaedenb.org/app/vehicle-info/vehicle-state/) for complete field list.

### WebSocket Subscriptions

**STATUS**: Implemented with graceful degradation.

**WebSocket Endpoint**: `wss://rivian.com/api/gql/gateway/graphql`

**Required Headers**:
```
apollographql-client-name: com.rivian.android.consumer
Sec-WebSocket-Protocol: graphql-ws
a-sess: <appSessionToken>
csrf-token: <csrfToken>
u-sess: <userSessionToken>
```

**Critical Implementation Details**:
1. Use `graphql-ws` protocol (NOT `graphql-transport-ws`)
2. Send `connection_init` message immediately after connection with `apollographql-client-name` in payload
3. CSRF token and app session ID must be FRESH - call `CreateCSRFToken` before each WebSocket connection
4. WebSocket connection often fails with "bad handshake" - implement graceful degradation (use manual refresh as fallback)

**Implemented Subscriptions**:
1. **vehicleState**: Real-time updates for battery, charging, range, locks, temperature, doors, windows, tire status
   - See `internal/rivian/websocket.go` for full query structure
   - Updates come as GraphQL `data` messages with partial state changes
   - Apply updates through state reducer for consistency

**Known Limitations**:
- WebSocket connection is unreliable and may fail to establish (Rivian server-side issues)
- TUI implements graceful fallback - continues functioning without real-time updates
- Users can manually refresh with 'r' key when WebSocket is unavailable

See [RivDocs Subscriptions](https://rivian-api.kaedenb.org/app/vehicle-info/subscriptions/) for complete subscription list.

### Common API Pitfalls

Based on real-world testing, these are the most common issues:

#### 1. GraphQL Validation Errors

**Symptom**: `{"errors":[{"extensions":{"code":"GRAPHQL_VALIDATION_FAILED"},"message":"Error in GraphQL validation"}]}`

**Common causes**:
- Using wrong client identifier (`com.rivian.ios.consumer` instead of `com.rivian.android.consumer`)
- Querying fields that don't exist in the schema
- Incorrect nesting (e.g., `model` is inside `vehicle`, not a direct field)
- Wrong variable names (e.g., `$vehicleId` instead of `$vehicleID`)

**Debugging**:
1. Compare your query against [RivDocs](https://rivian-api.kaedenb.org/)
2. Check `__typename` fields to see actual response structure
3. Use the debug tool: `./debug-auth` to see raw requests/responses

#### 2. "Entity not found" Error

**Symptom**: GraphQL returns success (200) but error: `{"errors":[{"message":"Entity not found"}]}`

**Root cause**: Wrong `apollographql-client-name` header.

**Fix**: Use `com.rivian.android.consumer`, not `com.rivian.ios.consumer`.

#### 3. Type Mismatch Errors

**Symptom**: `json: cannot unmarshal string into Go struct field ... of type int64`

**Root cause**: Rivian uses ISO 8601 string timestamps everywhere, not Unix integers.

**Fix**: All timestamp fields should be `string` type, parse with `time.Parse(time.RFC3339, ts)`.

#### 4. Missing Email in OTP Flow

**Symptom**: `LoginWithOTP` mutation fails with missing email parameter.

**Root cause**: The mutation requires the original email from the `Login` step, but it's not returned in `MobileMFALoginResponse`.

**Fix**: Store the email from the initial `Authenticate()` call and pass it to `SubmitOTP()`.

## TUI (Terminal User Interface)

### Overview

The TUI is built using [Bubble Tea](https://github.com/charmbracelet/bubbletea) (Elm architecture) and [Lip Gloss](https://github.com/charmbracelet/lipgloss) (styling).

**Key features**:
- Real-time updates via WebSocket (with graceful degradation to manual refresh)
- Three main views: Dashboard, Charge, Health
- Keyboard navigation ([1]/[2]/[3] for views, [r] for refresh, [q] to quit)
- Alt-screen mode (preserves terminal on exit)

### Dashboard View

Displays vehicle overview in three-column layout:

**Left Column**:
- Battery & Range section with visual battery bar
- Charging section with status (Complete, Charging, Disconnected, etc.)

**Middle Column**:
- Security section (lock status, doors, windows)
- Tire Status section (OK/Low/High for each tire)

**Right Column**:
- Climate & Travel section (temperature, odometer, location)
- Battery Stats section (calculated capacity, efficiency, mi/kWh, mi/%)

### Calculated Metrics

Since the Rivian API doesn't expose all desired metrics, we calculate them:

#### Battery Capacity

Formula: `capacity = (rangeEstimate / batteryLevel) * 100 * efficiency`

Where efficiency is:
- R1T: 2.0 mi/kWh (typical average)
- R1S: 2.1 mi/kWh (slightly better due to aerodynamics)

#### Current Energy

Formula: `currentEnergy = batteryLevel / 100.0 * batteryCapacity`

#### Efficiency Rating

Formula: `efficiency = rangeEstimate / currentEnergy` (in mi/kWh)

#### Miles per Percent

Formula: `miPerPercent = rangeEstimate / batteryLevel`

### Color Coding

The TUI uses color to provide quick visual feedback:

**Battery/Range**:
- Green: ≥50 miles (normal)
- Yellow: <50 miles (low)
- Red: <25 miles (critical)

**Tire Status**:
- Green: OK
- Yellow: Low
- Orange: High
- Gray: Unknown

**Temperature**:
- Green: 60-80°F (comfortable)
- Yellow: 40-60°F or 80-90°F (less comfortable)
- Red: <40°F or >90°F (extreme)

**Charge Status**:
- Green: Charging, Complete
- Yellow: Scheduled
- Gray: Disconnected, Not Charging, Unknown

### Graceful Degradation

The TUI is designed to handle API limitations:

1. **WebSocket failures**: If WebSocket connection fails, TUI continues with manual refresh only
2. **Missing tire PSI values**: Shows status (OK/Low) instead of actual pressure
3. **Missing battery capacity**: Calculates from available data
4. **Offline mode**: Shows last known state from local cache

### Implementation Notes

**Footer Black Box Issue**: The footer width calculation must account for padding:
```go
availableWidth := m.width - 2  // Subtract padding
footerStyle.Width(availableWidth).Padding(0, 1)
```

**Format Verb Errors**: Avoid nested formatting with styled strings:
```go
// WRONG: Creates format errors like %!d(string=70)%
valueStyle.Render(fmt.Sprintf("%d", value))

// RIGHT: Format the full string including units
valueStyle.Render(fmt.Sprintf("%d%%", value))
```

#### Message Flow

1. **Client → Server**: `connection_init` with apollo client name
2. **Server → Client**: `connection_ack` when ready
3. **Client → Server**: `start` with subscription query (ID, query, variables)
4. **Server → Client**: `data` messages with subscription updates (same ID)
5. **Server → Client**: `ka` (keep-alive) messages every ~30 seconds
6. **Client → Server**: `stop` to end subscription (with ID)
7. **Client → Server**: `connection_terminate` to close connection

#### Subscription Query Example

```graphql
subscription VehicleStateUpdates($vehicleId: String!) {
  vehicleState(id: $vehicleId) {
    __typename
    batteryLevel { value timeStamp }
    chargeState { value timeStamp }
    rangeEstimate { value timeStamp }
    isLocked { value timeStamp }
    cabinTemp { value timeStamp }
  }
}
```

**Note**: Subscriptions use the same timestamped value pattern as queries.

#### Implementation Details

- **Auto-reconnect**: Up to 10 retries with 5-second delay
- **Ping/pong**: Client sends pings every 30 seconds to keep connection alive
- **Error handling**: Subscription errors are delivered via `error` messages (with subscription ID)
- **Completion**: Server sends `complete` when subscription ends

#### Usage Example

```go
// Create WebSocket client
wsClient := rivian.NewWebSocketClient(credentials, csrfToken, appSessionID)

// Connect
if err := wsClient.Connect(ctx); err != nil {
    return err
}
defer wsClient.Close()

// Subscribe to vehicle state updates
subscription, err := rivian.SubscribeToVehicleState(ctx, wsClient, vehicleID)
if err != nil {
    return err
}
defer subscription.Close()

// Receive updates
for update := range subscription.Updates() {
    // Process update (convert to PartialStateUpdate event)
    fmt.Printf("Battery: %v\n", update["data"]["vehicleState"]["batteryLevel"]["value"])
}
```

See `internal/rivian/websocket_test.go` for comprehensive examples.

### Debugging Tools

Two test binaries are available in `cmd/test-api/`:

#### `test-api`

Full integration test that runs:
1. Authentication (with OTP)
2. GetVehicles query
3. GetVehicleState query
4. JSON output of vehicle state

```bash
go build -o test-api ./cmd/test-api/main.go
./test-api -email your@email.com -password yourpassword
```

### Known Caveats

- **Rate limiting**: Unknown. Be conservative with polling intervals (recommend 30s+).
- **Breaking changes**: The API can change without notice - this is an unofficial API.
- **Terms of Service**: Using this API may violate Rivian's ToS. Use at your own risk.
- **MFA/OTP**: Most accounts require OTP via SMS. The app handles this interactively.
- **Client identifier matters**: Using iOS client name (`com.rivian.ios.consumer`) causes "Entity not found" errors. Always use Android client name.
- **Timestamp inconsistency**: All timestamps are ISO 8601 strings, not Unix integers.
- **Tire pressure**: API returns status enums ("normal", "low"), not actual PSI values.
- **Units**: Odometer is in meters, temperatures vary by endpoint.
- **Missing data**: Not all vehicles support all fields (e.g., R1S doesn't have `closureTonneauClosed`).

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

## Headless CLI Commands

The CLI provides three main commands for non-interactive vehicle monitoring and data export, implemented in `internal/cli/`.

### Command Overview

| Command | Purpose | Output Formats |
|---------|---------|----------------|
| `status` | Current vehicle state snapshot | JSON, YAML, CSV, text, table |
| `watch` | Real-time streaming updates | JSON, YAML, CSV, text, table |
| `export` | Historical data export | JSON, YAML, CSV |

### status - Current State Snapshot

Displays current vehicle state from either live API query or cached data.

**Usage:**
```bash
# Live query (default)
rivian-ls status --format json

# Pretty-printed JSON
rivian-ls status --format json --pretty

# Human-readable text
rivian-ls status --format text

# Offline mode (use cached data)
rivian-ls status --offline --format text

# Table format for quick overview
rivian-ls status --format table
```

**Behavior:**
- Live mode: Queries API, updates ReadyScore, saves to storage
- Offline mode: Reads latest state from SQLite storage
- Auto-saves all live queries for offline access later

### watch - Real-Time Streaming

Streams vehicle state updates in real-time using WebSocket or polling.

**Usage:**
```bash
# WebSocket mode (real-time, default)
rivian-ls watch --format text

# Polling mode (30-second intervals)
rivian-ls watch --interval 30s --format json

# Table format for monitoring
rivian-ls watch --format table

# Pretty JSON for debugging
rivian-ls watch --format json --pretty
```

**Behavior:**
- WebSocket mode (default): Subscribes to live updates via GraphQL subscriptions
- Polling mode: Queries API at specified interval
- Graceful shutdown: Ctrl+C triggers cleanup and connection close
- Auto-saves all updates to storage
- Uses Redux reducer for incremental state updates

**WebSocket Update Flow:**
1. Initial HTTP query for full state
2. Subscribe to vehicle state changes
3. Receive partial updates (battery, range, charging, lock, temp)
4. Apply updates via PartialStateUpdate event
5. Output formatted state
6. Save to storage

### export - Historical Data Export

Exports historical vehicle state data from local storage.

**Usage:**
```bash
# Last 24 hours as CSV
rivian-ls export --since 24h --format csv > history.csv

# Specific date range as JSON
rivian-ls export --since 2024-01-01 --until 2024-01-15 --format json

# Last 100 states (default)
rivian-ls export --format csv

# Limited number of records
rivian-ls export --since 7d --limit 50 --format yaml

# Pretty JSON for analysis
rivian-ls export --since 30d --format json --pretty > month.json
```

**Query Modes:**
- Range query: Both `--since` and `--until` specified
- History query: Only `--since` specified (with optional `--limit`)
- Recent query: No time flags (last 100 states from past year)

**Time Formats:**
- Absolute: `2024-01-15T10:00:00Z`, `2024-01-15`
- Relative: `24h`, `7d`, `30d`, `1h30m`

### Output Formatters

All commands support multiple output formats via `--format` flag.

#### JSON Format
- Machine-readable structured data
- `--pretty` flag for indented output
- Preserves all data types (numbers, booleans, nulls)
- Perfect for: API integration, data processing, scripts

#### YAML Format
- Human-readable structured data
- 2-space indentation
- Great for: Configuration-style output, manual inspection
- Smaller than pretty JSON

#### CSV Format
- Excel-compatible tabular data
- Header row with field names
- Flattened structure (no nested objects)
- Fields: Timestamp, VehicleID, VIN, Name, Model, BatteryLevel, RangeEstimate, ChargeState, ChargeLimit, IsLocked, IsOnline, Latitude, Longitude, CabinTemp, ExteriorTemp, Odometer, ReadyScore
- Perfect for: Spreadsheet analysis, data visualization, reporting

#### Text Format
- Human-friendly terminal output
- Formatted sections:
  - Vehicle identity (name, model, VIN)
  - Battery & range with status
  - Charging state and rate
  - Security (lock, doors, windows, closures)
  - Climate (cabin/exterior temps)
  - Location coordinates
  - Odometer reading
  - Ready Score
  - Issue list (if any)
- Perfect for: Terminal monitoring, quick checks

#### Table Format
- Compact columnar layout
- Shows multiple states in rows
- Fixed-width columns
- Headers: TIMESTAMP, BATTERY, RANGE, LOCK, CHARGING, STATUS
- Perfect for: Comparing state history, trends over time

### Implementation Architecture

**Command Pattern:**
```go
type StatusCommand struct {
    client    rivian.Client
    store     *store.Store
    vehicleID string
    output    io.Writer
}

func (c *StatusCommand) Run(ctx context.Context, opts StatusOptions) error
```

**Strategy Pattern for Formatters:**
```go
type Formatter interface {
    FormatState(w io.Writer, state *model.VehicleState) error
    FormatStates(w io.Writer, states []*model.VehicleState) error
}
```

**Key Design Decisions:**
1. Commands operate on io.Writer for testability
2. Context-aware for cancellation and timeout
3. Non-fatal errors (storage) log warnings
4. Fatal errors return to caller
5. Dependency injection for testing

### Testing Strategy

CLI tests use:
- Mock `rivian.Client` implementation
- Temporary SQLite databases (`t.TempDir()`)
- Bytes buffer for output verification
- Table-driven tests for formatters

**Coverage:** 57.9% overall (83.8% for formatters)

Lower coverage on watch command due to WebSocket complexity and signal handling, which are integration-tested manually.

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
