package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/pfrederiksen/rivian-ls/internal/auth"
	"github.com/pfrederiksen/rivian-ls/internal/cli"
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
			return 1
		}
		return 0
	}

	// Check for subcommands (status, watch, export)
	var subcommand string
	var subcommandArgs []string
	if len(args) > 1 && !strings.HasPrefix(args[1], "-") {
		subcommand = args[1]
		subcommandArgs = args[2:]
	}

	// Parse command line flags
	fs := flag.NewFlagSet("rivian-ls", flag.ExitOnError)
	email := fs.String("email", "", "Email address for authentication")
	password := fs.String("password", "", "Password (will prompt if not provided)")
	vehicleIndex := fs.Int("vehicle", 0, "Vehicle index (0-based)")
	dbPath := fs.String("db", "", "Database path (default: ~/.local/share/rivian-ls/state.db)")
	versionFlag := fs.Bool("version", false, "Print version and exit")

	if err := fs.Parse(args[1:]); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error parsing flags: %v\n", err)
		return 1
	}

	// Handle version flag
	if *versionFlag {
		if err := printVersion(os.Stdout); err != nil {
			return 1
		}
		return 0
	}

	// Determine database path
	if *dbPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Error getting home directory: %v\n", err)
			return 1
		}
		*dbPath = home + "/.local/share/rivian-ls/state.db"

		// Ensure directory exists
		dbDir := home + "/.local/share/rivian-ls"
		if err := os.MkdirAll(dbDir, 0750); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Error creating database directory: %v\n", err)
			return 1
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

	// Try to authenticate
	if err := authenticate(ctx, client, credCache, email, password); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Authentication failed: %v\n", err)
		return 1
	}

	// Get vehicles
	vehicles, err := client.GetVehicles(ctx)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Failed to get vehicles: %v\n", err)
		return 1
	}

	if len(vehicles) == 0 {
		_, _ = fmt.Fprintf(os.Stderr, "No vehicles found\n")
		return 1
	}

	if *vehicleIndex >= len(vehicles) {
		_, _ = fmt.Fprintf(os.Stderr, "Vehicle index %d out of range (have %d vehicles)\n", *vehicleIndex, len(vehicles))
		return 1
	}

	vehicle := vehicles[*vehicleIndex]

	// Open database
	db, err := store.NewStore(*dbPath)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Failed to open database: %v\n", err)
		return 1
	}
	defer func() { _ = db.Close() }()

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
		model := tui.NewModel(client, db, vehicle.ID)
		p := tea.NewProgram(model, tea.WithAltScreen())

		if _, err := p.Run(); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Error running TUI: %v\n", err)
			return 1
		}
		return 0
	default:
		_, _ = fmt.Fprintf(os.Stderr, "Unknown command: %s\n", subcommand)
		_, _ = fmt.Fprintf(os.Stderr, "Available commands: status, watch, export\n")
		return 1
	}
}

func authenticate(ctx context.Context, client *rivian.HTTPClient, credCache *auth.CredentialsCache, email, password *string) error {
	// If no email provided, try to load from cache
	if *email == "" {
		if credCache != nil {
			cached, err := credCache.Load()
			if err == nil && cached != nil && cached.IsValid() {
				client.SetCredentials(cached.ToRivianCredentials())
				return nil
			}
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
					needsAuth = false
				} else {
					// Try to refresh
					client.SetCredentials(cached.ToRivianCredentials())
					if err := client.RefreshToken(ctx); err == nil {
						needsAuth = false
						// Save refreshed credentials
						if creds := client.GetCredentials(); creds != nil {
							_ = credCache.Save(*email, creds)
						}
					}
				}
			}
		}
	}

	// Perform full authentication if needed
	if needsAuth {
		// Prompt for password if not provided
		if *password == "" {
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
				fmt.Print("Enter OTP code: ")
				scanner := bufio.NewScanner(os.Stdin)
				scanner.Scan()
				otpCode := strings.TrimSpace(scanner.Text())

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
		}
	}

	return nil
}

func runStatusCommand(ctx context.Context, client rivian.Client, db *store.Store, vehicleID string, args []string) int {
	fs := flag.NewFlagSet("status", flag.ExitOnError)
	format := fs.String("format", "text", "Output format (text|json|yaml|csv|table)")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON/YAML output")
	offline := fs.Bool("offline", false, "Use cached data (offline mode)")

	if err := fs.Parse(args); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error parsing status flags: %v\n", err)
		return 1
	}

	cmd := cli.NewStatusCommand(client, db, vehicleID, os.Stdout)
	opts := cli.StatusOptions{
		Format:  cli.OutputFormat(*format),
		Pretty:  *pretty,
		Offline: *offline,
	}

	if err := cmd.Run(ctx, opts); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Status command failed: %v\n", err)
		return 1
	}

	return 0
}

func runWatchCommand(ctx context.Context, client rivian.Client, db *store.Store, vehicleID string, args []string) int {
	fs := flag.NewFlagSet("watch", flag.ExitOnError)
	format := fs.String("format", "text", "Output format (text|json|yaml|csv|table)")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON/YAML output")
	interval := fs.Duration("interval", 0, "Polling interval (0 = use WebSocket)")

	if err := fs.Parse(args); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error parsing watch flags: %v\n", err)
		return 1
	}

	// Get CSRF token and app session ID for WebSocket mode
	var csrfToken, appSessionID string
	if *interval == 0 {
		// WebSocket mode requires fresh session tokens
		httpClient, ok := client.(*rivian.HTTPClient)
		if !ok {
			_, _ = fmt.Fprintf(os.Stderr, "WebSocket mode requires HTTPClient\n")
			return 1
		}

		// Create fresh session for WebSocket
		if err := httpClient.CreateSession(ctx); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Failed to create session: %v\n", err)
			return 1
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
		return 1
	}

	return 0
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
		return 1
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
				return 1
			}
			sinceTime = t
		}
	}

	if *until != "" {
		t, err := time.Parse(time.RFC3339, *until)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Invalid until time: %v\n", err)
			return 1
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
		return 1
	}

	return 0
}

func main() {
	exitCode := run(os.Args)
	os.Exit(exitCode)
}
