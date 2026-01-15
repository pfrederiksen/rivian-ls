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
5. **Tire pressure**: API returns status strings ("normal", "low"), NOT actual PSI values
6. **Odometer**: Value is in meters (multiply by 0.000621371 for miles)

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

**STATUS**: Not yet implemented.

Planned subscriptions:

1. **Vehicle state updates**: Battery, charging, range
2. **Tire pressure updates**: Individual tire pressures
3. **Closures**: Doors, frunk, liftgate, windows

WebSocket endpoint and subscription format TBD based on further reverse engineering.

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

#### `debug-auth`

Minimal authentication test with full request/response logging:

```bash
go build -o debug-auth ./cmd/test-api/debug-auth.go
./debug-auth -email your@email.com -password yourpassword
```

Shows:
- Exact GraphQL mutations being sent
- All HTTP headers
- Full JSON responses (pretty-printed)
- Step-by-step authentication flow

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
