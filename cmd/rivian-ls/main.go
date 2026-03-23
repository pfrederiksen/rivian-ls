package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/pfrederiksen/rivian-ls/internal/auth"
	"github.com/pfrederiksen/rivian-ls/internal/cli"
	"github.com/pfrederiksen/rivian-ls/internal/config"
	"github.com/pfrederiksen/rivian-ls/internal/rivian"
	"github.com/pfrederiksen/rivian-ls/internal/store"
	"github.com/pfrederiksen/rivian-ls/internal/tui"
	"golang.org/x/term"
)

// Version information - set by GoReleaser via ldflags
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

// Exit codes
const (
	ExitSuccess       = 0
	ExitAuthFailure   = 1
	ExitVehicleNotFound = 2
	ExitAPIError      = 3
	ExitInvalidArgs   = 4
)

func printVersion(w io.Writer) error {
	if _, err := fmt.Fprintf(w, "rivian-ls version %s\n", version); err != nil {
		return err
	}
	if commit != "none" {
		if _, err := fmt.Fprintf(w, "  commit: %s\n", commit); err != nil {
			return err
		}
	}
	if date != "unknown" {
		if _, err := fmt.Fprintf(w, "  built:  %s\n", date); err != nil {
			return err
		}
	}
	return nil
}

func run(args []string) int {
	// Handle version subcommand first (before flag parsing)
	if len(args) > 1 && args[1] == "version" {
		if err := printVersion(os.Stdout); err != nil {
			return ExitInvalidArgs
		}
		return ExitSuccess
	}

	// Load configuration from file and environment variables
	cfg, err := config.Load()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error loading configuration: %v\n", err)
		return ExitInvalidArgs
	}

	// Parse command line flags (using config values as defaults)
	fs := flag.NewFlagSet("rivian-ls", flag.ExitOnError)
	email := fs.String("email", cfg.Email, "Email address for authentication")
	password := fs.String("password", cfg.Password, "Password (will prompt if not provided)")
	otp := fs.String("otp", "", "OTP/MFA code (for non-interactive login)")
	vehicleIndex := fs.Int("vehicle", cfg.Vehicle, "Vehicle index (0-based)")
	dbPath := fs.String("db", cfg.DBPath, "Database path (default: ~/.local/share/rivian-ls/state.db)")
	versionFlag := fs.Bool("version", false, "Print version and exit")
	quiet := fs.Bool("quiet", cfg.Quiet, "Suppress informational output")
	verbose := fs.Bool("verbose", cfg.Verbose, "Enable verbose logging")
	noStore := fs.Bool("no-store", cfg.DisableStore, "Don't persist snapshots locally")

	if err := fs.Parse(args[1:]); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error parsing flags: %v\n", err)
		return ExitInvalidArgs
	}

	// Check for subcommands in remaining args after flag parsing
	var subcommand string
	var subcommandArgs []string
	remainingArgs := fs.Args()
	if len(remainingArgs) > 0 {
		subcommand = remainingArgs[0]
		subcommandArgs = remainingArgs[1:]
	}

	// Handle version flag
	if *versionFlag {
		if err := printVersion(os.Stdout); err != nil {
			return ExitInvalidArgs
		}
		return ExitSuccess
	}

	// Set verbosity based on flags
	if *quiet && *verbose {
		_, _ = fmt.Fprintf(os.Stderr, "Error: --quiet and --verbose cannot be used together\n")
		return ExitInvalidArgs
	}

	// Apply verbosity settings to logger (we'll add proper logging later)
	// For now, just store the flags
	_ = quiet
	_ = verbose

	// Ensure database directory exists (unless --no-store is set)
	if !*noStore {
		dbDir := filepath.Dir(*dbPath)
		if err := os.MkdirAll(dbDir, 0750); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Error creating database directory: %v\n", err)
			return ExitInvalidArgs
		}
	}

	ctx := context.Background()

	// Create HTTP client
	client := rivian.NewHTTPClient()

	// Create credentials cache
	credCache, err := auth.NewCredentialsCache()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Warning: Could not create credentials cache: %v\n", err)
		credCache = nil
	}

	// Handle login subcommand (auth-only, no vehicle query needed)
	if subcommand == "login" {
		return runLoginCommand(ctx, client, credCache, email, password, otp)
	}

	// Try to authenticate
	if err := authenticate(ctx, client, credCache, email, password, otp); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Authentication failed: %v\n", err)
		return ExitAuthFailure
	}

	// Get vehicles
	vehicles, err := client.GetVehicles(ctx)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Failed to get vehicles: %v\n", err)
		return ExitAPIError
	}

	if len(vehicles) == 0 {
		_, _ = fmt.Fprintf(os.Stderr, "No vehicles found\n")
		return ExitVehicleNotFound
	}

	if *vehicleIndex < 0 || *vehicleIndex >= len(vehicles) {
		_, _ = fmt.Fprintf(os.Stderr, "Vehicle index %d out of range (have %d vehicles)\n", *vehicleIndex, len(vehicles))
		return ExitVehicleNotFound
	}

	vehicle := vehicles[*vehicleIndex]

	// Open database (unless --no-store is set)
	var db *store.Store
	if !*noStore {
		var err error
		db, err = store.NewStore(*dbPath)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Failed to open database: %v\n", err)
			return ExitInvalidArgs
		}
		defer func() { _ = db.Close() }()
	}

	// Route to subcommand or launch TUI
	switch subcommand {
	case "status":
		return runStatusCommand(ctx, client, db, vehicle.ID, subcommandArgs)
	case "watch":
		return runWatchCommand(ctx, client, db, vehicle.ID, subcommandArgs)
	case "export":
		return runExportCommand(ctx, db, vehicle.ID, subcommandArgs)
	case "":
		// No subcommand - launch TUI
		model := tui.NewModel(client, db, vehicles, *vehicleIndex)
		p := tea.NewProgram(model, tea.WithAltScreen())

		if _, err := p.Run(); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Error running TUI: %v\n", err)
			return ExitAPIError
		}
		return ExitSuccess
	default:
		_, _ = fmt.Fprintf(os.Stderr, "Unknown command: %s\n", subcommand)
		_, _ = fmt.Fprintf(os.Stderr, "Available commands: login, status, watch, export\n")
		return ExitInvalidArgs
	}
}

// isInteractive returns true if stdin is a terminal (TTY).
func isInteractive() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
}

func authenticate(ctx context.Context, client *rivian.HTTPClient, credCache *auth.CredentialsCache, email, password, otp *string) error {
	// Phase 2: If --otp is provided, check for a pending OTP session first.
	// This completes a two-phase login:
	//   Phase 1: rivian-ls login --email x --password y   → triggers SMS
	//   Phase 2: rivian-ls login --otp 123456              → completes auth
	if *otp != "" && credCache != nil {
		pending, err := credCache.LoadPendingOTP()
		if err == nil && pending != nil && pending.IsValid() {
			client.SetSessionTokens(pending.CSRFToken, pending.AppSessionID)
			client.SetOTPState(pending.OTPToken, pending.Email)

			if err := client.SubmitOTP(ctx, *otp); err != nil {
				_ = credCache.DeletePendingOTP()
				return fmt.Errorf("OTP submission failed: %w", err)
			}

			// Clean up pending OTP and save credentials
			_ = credCache.DeletePendingOTP()

			if creds := client.GetCredentials(); creds != nil {
				if err := credCache.Save(pending.Email, creds); err != nil {
					_, _ = fmt.Fprintf(os.Stderr, "Warning: Could not save credentials: %v\n", err)
				}
			}
			return nil
		}
		// No valid pending session — fall through to normal auth
	}

	// If no email provided, try to load from cache
	if *email == "" {
		if credCache != nil {
			cached, err := credCache.Load()
			if err == nil && cached != nil && cached.IsValid() {
				client.SetCredentials(cached.ToRivianCredentials())
				// Create a fresh session so CSRF token and app session ID are set
				if err := client.CreateSession(ctx); err == nil {
					return nil
				}
				// Session creation failed — cached token may be invalid server-side.
				// Fall through to prompt for credentials.
				_, _ = fmt.Fprintf(os.Stderr, "Warning: Cached session expired, re-authentication required\n")
				_ = credCache.Delete()
			}
		}

		if !isInteractive() {
			return fmt.Errorf("email required: use --email flag, RIVIAN_EMAIL env var, or config file")
		}
		fmt.Print("Email: ")
		scanner := bufio.NewScanner(os.Stdin)
		scanner.Scan()
		emailInput := strings.TrimSpace(scanner.Text())
		email = &emailInput
	}

	// Try to load cached credentials for this email
	var needsAuth = true
	if credCache != nil {
		cached, err := credCache.Load()
		if err == nil && cached != nil {
			if cached.Email == *email {
				if cached.IsValid() {
					client.SetCredentials(cached.ToRivianCredentials())
					// Create a fresh session so CSRF token and app session ID are set
					if err := client.CreateSession(ctx); err == nil {
						needsAuth = false
					} else {
						// Token looks valid locally but rejected server-side
						_, _ = fmt.Fprintf(os.Stderr, "Warning: Cached session expired, re-authentication required\n")
						_ = credCache.Delete()
					}
				} else {
					// Try to refresh - need a session first for the API call
					client.SetCredentials(cached.ToRivianCredentials())
					if err := client.CreateSession(ctx); err == nil {
						if err := client.RefreshToken(ctx); err == nil {
							needsAuth = false
							// Save refreshed credentials
							if creds := client.GetCredentials(); creds != nil {
								_ = credCache.Save(*email, creds)
							}
						} else {
							// Refresh failed — clear stale cache
							_ = credCache.Delete()
						}
					} else {
						// Session creation failed — clear stale cache
						_ = credCache.Delete()
					}
				}
			}
		}
	}

	// Perform full authentication if needed
	if needsAuth {
		// Prompt for password if not provided
		if *password == "" {
			if !isInteractive() {
				return fmt.Errorf("password required: use --password flag, RIVIAN_PASSWORD env var, or config file")
			}
			fmt.Print("Password: ")
			passBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
			fmt.Println()
			if err != nil {
				return fmt.Errorf("failed to read password: %w", err)
			}
			pwd := string(passBytes)
			password = &pwd
		}

		err := client.Authenticate(ctx, *email, *password)
		if err != nil {
			// Check if it's OTP required
			if _, ok := err.(*rivian.OTPRequiredError); ok {
				otpCode := *otp
				if otpCode == "" {
					if !isInteractive() {
						// Non-interactive: save the OTP session for phase 2
						if credCache != nil {
							pending := &auth.PendingOTP{
								Email:        *email,
								OTPToken:     client.GetOTPToken(),
								CSRFToken:    client.GetCSRFToken(),
								AppSessionID: client.GetAppSessionID(),
							}
							if saveErr := credCache.SavePendingOTP(pending); saveErr != nil {
								return fmt.Errorf("failed to save OTP session: %w", saveErr)
							}
						}
						return fmt.Errorf("OTP sent. Complete login with: rivian-ls login --otp <code>")
					}
					_, _ = fmt.Fprintf(os.Stderr, "OTP code sent. Enter OTP code: ")
					scanner := bufio.NewScanner(os.Stdin)
					scanner.Scan()
					otpCode = strings.TrimSpace(scanner.Text())
					if otpCode == "" {
						return fmt.Errorf("OTP required but no code provided")
					}
				}

				if err := client.SubmitOTP(ctx, otpCode); err != nil {
					return fmt.Errorf("OTP submission failed: %w", err)
				}
			} else {
				return err
			}
		}

		// Verify authentication
		if !client.IsAuthenticated() {
			return fmt.Errorf("authentication failed: not authenticated after login")
		}

		// Save credentials to cache
		if credCache != nil {
			if creds := client.GetCredentials(); creds != nil {
				if err := credCache.Save(*email, creds); err != nil {
					_, _ = fmt.Fprintf(os.Stderr, "Warning: Could not save credentials: %v\n", err)
				}
			}
			// Clean up any leftover pending OTP
			_ = credCache.DeletePendingOTP()
		}
	}

	return nil
}

func runLoginCommand(ctx context.Context, client *rivian.HTTPClient, credCache *auth.CredentialsCache, email, password, otp *string) int {
	if err := authenticate(ctx, client, credCache, email, password, otp); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Authentication failed: %v\n", err)
		return ExitAuthFailure
	}

	// Verify by fetching vehicles (validates the session works end-to-end)
	vehicles, err := client.GetVehicles(ctx)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Login succeeded but API validation failed: %v\n", err)
		return ExitAPIError
	}

	fmt.Printf("Authenticated successfully. Found %d vehicle(s).\n", len(vehicles))
	for i, v := range vehicles {
		name := v.Name
		if name == "" {
			name = v.Model
		}
		fmt.Printf("  [%d] %s (VIN: ...%s)\n", i, name, v.VIN[len(v.VIN)-6:])
	}

	return ExitSuccess
}

func runStatusCommand(ctx context.Context, client rivian.Client, db *store.Store, vehicleID string, args []string) int {
	fs := flag.NewFlagSet("status", flag.ExitOnError)
	format := fs.String("format", "text", "Output format (text|json|yaml|csv|table)")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON/YAML output")
	offline := fs.Bool("offline", false, "Use cached data (offline mode)")

	if err := fs.Parse(args); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error parsing status flags: %v\n", err)
		return ExitInvalidArgs
	}

	cmd := cli.NewStatusCommand(client, db, vehicleID, os.Stdout)
	opts := cli.StatusOptions{
		Format:  cli.OutputFormat(*format),
		Pretty:  *pretty,
		Offline: *offline,
	}

	if err := cmd.Run(ctx, opts); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Status command failed: %v\n", err)
		return ExitAPIError
	}

	return ExitSuccess
}

func runWatchCommand(ctx context.Context, client rivian.Client, db *store.Store, vehicleID string, args []string) int {
	fs := flag.NewFlagSet("watch", flag.ExitOnError)
	format := fs.String("format", "text", "Output format (text|json|yaml|csv|table)")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON/YAML output")
	interval := fs.Duration("interval", 0, "Polling interval (0 = use WebSocket)")

	if err := fs.Parse(args); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error parsing watch flags: %v\n", err)
		return ExitInvalidArgs
	}

	// Get CSRF token and app session ID for WebSocket mode
	var csrfToken, appSessionID string
	if *interval == 0 {
		// WebSocket mode requires fresh session tokens
		httpClient, ok := client.(*rivian.HTTPClient)
		if !ok {
			_, _ = fmt.Fprintf(os.Stderr, "WebSocket mode requires HTTPClient\n")
			return ExitInvalidArgs
		}

		// Create fresh session for WebSocket
		if err := httpClient.CreateSession(ctx); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Failed to create session: %v\n", err)
			return ExitAPIError
		}

		csrfToken = httpClient.GetCSRFToken()
		appSessionID = httpClient.GetAppSessionID()
	}

	cmd := cli.NewWatchCommand(client, db, vehicleID, csrfToken, appSessionID, os.Stdout)
	opts := cli.WatchOptions{
		Format:   cli.OutputFormat(*format),
		Pretty:   *pretty,
		Interval: *interval,
	}

	if err := cmd.Run(ctx, opts); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Watch command failed: %v\n", err)
		return ExitAPIError
	}

	return ExitSuccess
}

func runExportCommand(ctx context.Context, db *store.Store, vehicleID string, args []string) int {
	fs := flag.NewFlagSet("export", flag.ExitOnError)
	format := fs.String("format", "csv", "Output format (json|yaml|csv)")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON/YAML output")
	since := fs.String("since", "", "Start time (RFC3339 or duration like '24h')")
	until := fs.String("until", "", "End time (RFC3339)")
	limit := fs.Int("limit", 0, "Maximum number of states to export")

	if err := fs.Parse(args); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error parsing export flags: %v\n", err)
		return ExitInvalidArgs
	}

	// Parse time arguments
	var sinceTime, untilTime time.Time
	if *since != "" {
		// Try parsing as duration first
		if d, err := time.ParseDuration(*since); err == nil {
			sinceTime = time.Now().Add(-d)
		} else {
			// Try parsing as RFC3339
			t, err := time.Parse(time.RFC3339, *since)
			if err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "Invalid since time: %v\n", err)
				return ExitInvalidArgs
			}
			sinceTime = t
		}
	}

	if *until != "" {
		t, err := time.Parse(time.RFC3339, *until)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Invalid until time: %v\n", err)
			return ExitInvalidArgs
		}
		untilTime = t
	}

	cmd := cli.NewExportCommand(db, vehicleID, os.Stdout)
	opts := cli.ExportOptions{
		Format: cli.OutputFormat(*format),
		Pretty: *pretty,
		Since:  sinceTime,
		Until:  untilTime,
		Limit:  *limit,
	}

	if err := cmd.Run(ctx, opts); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Export command failed: %v\n", err)
		return ExitAPIError
	}

	return ExitSuccess
}

func main() {
	exitCode := run(os.Args)
	os.Exit(exitCode)
}
