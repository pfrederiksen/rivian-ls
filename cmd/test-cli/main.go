package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/pfrederiksen/rivian-ls/internal/auth"
	"github.com/pfrederiksen/rivian-ls/internal/cli"
	"github.com/pfrederiksen/rivian-ls/internal/rivian"
	"github.com/pfrederiksen/rivian-ls/internal/store"
	"golang.org/x/term"
)

func main() {
	email := flag.String("email", "", "Email address")
	password := flag.String("password", "", "Password (will prompt if not provided)")
	vehicleIndex := flag.Int("vehicle", 0, "Vehicle index (0-based)")
	format := flag.String("format", "text", "Output format (json, yaml, csv, text, table)")
	dbPath := flag.String("db", "test-cli.db", "Database path for storage")
	flag.Parse()

	if *email == "" {
		fmt.Println("Usage: test-cli -email your@email.com [-password yourpassword]")
		os.Exit(1)
	}

	// Prompt for password if not provided
	if *password == "" {
		fmt.Print("Password: ")
		passBytes, err := term.ReadPassword(int(syscall.Stdin))
		fmt.Println()
		if err != nil {
			fmt.Printf("Error reading password: %v\n", err)
			os.Exit(1)
		}
		pwd := string(passBytes)
		password = &pwd
	}

	ctx := context.Background()

	// Create HTTP client
	client := rivian.NewHTTPClient()

	// Create credentials cache
	credCache, err := auth.NewCredentialsCache()
	if err != nil {
		fmt.Printf("Warning: Could not create credentials cache: %v\n", err)
		credCache = nil // Continue without caching
	}

	// Try to load cached credentials
	var needsAuth = true
	if credCache != nil {
		cached, err := credCache.Load()
		if err != nil {
			fmt.Printf("Warning: Could not load cached credentials: %v\n", err)
		} else if cached != nil {
			if cached.Email == *email {
				if cached.IsValid() {
					fmt.Println("Using cached credentials...")
					client.SetCredentials(cached.ToRivianCredentials())
					needsAuth = false
					fmt.Println("✓ Authenticated (from cache)")
				} else {
					fmt.Println("Cached credentials expired, refreshing...")
					client.SetCredentials(cached.ToRivianCredentials())
					if err := client.RefreshToken(ctx); err != nil {
						fmt.Printf("Token refresh failed: %v, re-authenticating...\n", err)
						needsAuth = true
					} else {
						needsAuth = false
						// Save refreshed credentials
						if creds := client.GetCredentials(); creds != nil {
							_ = credCache.Save(*email, creds)
						}
						fmt.Println("✓ Authenticated (token refreshed)")
					}
				}
			} else {
				fmt.Printf("Cached credentials for different email (%s), re-authenticating...\n", cached.Email)
			}
		}
	}

	// Perform full authentication if needed
	if needsAuth {
		fmt.Println("Authenticating...")
		err := client.Authenticate(ctx, *email, *password)
		if err != nil {
			// Check if it's OTP required (not a fatal error)
			if _, ok := err.(*rivian.OTPRequiredError); ok {
				fmt.Print("Enter OTP code: ")
				scanner := bufio.NewScanner(os.Stdin)
				scanner.Scan()
				otpCode := strings.TrimSpace(scanner.Text())

				fmt.Println("Submitting OTP...")
				err = client.SubmitOTP(ctx, otpCode)
				if err != nil {
					fmt.Printf("OTP submission failed: %v\n", err)
					os.Exit(1)
				}
			} else {
				// Real authentication error
				fmt.Printf("Authentication failed: %v\n", err)
				os.Exit(1)
			}
		}

		// Verify we're authenticated
		if !client.IsAuthenticated() {
			fmt.Println("Authentication failed: not authenticated after login")
			os.Exit(1)
		}

		fmt.Println("✓ Authenticated")

		// Save credentials to cache
		if credCache != nil {
			if creds := client.GetCredentials(); creds != nil {
				if err := credCache.Save(*email, creds); err != nil {
					fmt.Printf("Warning: Could not save credentials: %v\n", err)
				} else {
					fmt.Printf("Credentials cached to: %s\n", credCache.Path())
				}
			}
		}
	}

	// Get vehicles
	fmt.Println("\nFetching vehicles...")
	vehicles, err := client.GetVehicles(ctx)
	if err != nil {
		fmt.Printf("Get vehicles failed: %v\n", err)
		os.Exit(1)
	}

	if len(vehicles) == 0 {
		fmt.Println("No vehicles found")
		os.Exit(1)
	}

	if *vehicleIndex >= len(vehicles) {
		fmt.Printf("Vehicle index %d out of range (have %d vehicles)\n", *vehicleIndex, len(vehicles))
		os.Exit(1)
	}

	vehicle := vehicles[*vehicleIndex]
	fmt.Printf("✓ Found vehicle: %s (%s %s)\n", vehicle.Name, vehicle.Model, vehicle.VIN)

	// Create store
	fmt.Printf("\nOpening database: %s\n", *dbPath)
	testStore, err := store.NewStore(*dbPath)
	if err != nil {
		fmt.Printf("Failed to create store: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = testStore.Close() }()

	// Parse format
	outputFormat := cli.OutputFormat(*format)

	// Test 1: Status command (live)
	fmt.Println("\n=== Testing Status Command (Live) ===")
	statusCmd := cli.NewStatusCommand(client, testStore, vehicle.ID, os.Stdout)
	statusCmd.SetVehicleInfo(vehicle.Name, vehicle.VIN, vehicle.Model)
	statusOpts := cli.StatusOptions{
		Format: outputFormat,
		Pretty: true,
	}

	err = statusCmd.Run(ctx, statusOpts)
	if err != nil {
		fmt.Printf("\nError: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\n✓ Status command succeeded (state saved to database)")

	// Test 2: Status command (offline)
	fmt.Println("\n=== Testing Status Command (Offline) ===")
	statusOpts.Offline = true
	err = statusCmd.Run(ctx, statusOpts)
	if err != nil {
		fmt.Printf("\nError: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\n✓ Offline status command succeeded")

	// Test 3: Export command
	fmt.Println("\n=== Testing Export Command ===")
	exportCmd := cli.NewExportCommand(testStore, vehicle.ID, os.Stdout)
	exportOpts := cli.ExportOptions{
		Format: cli.FormatTable,
		Since:  time.Now().Add(-24 * time.Hour),
		Limit:  10,
	}

	err = exportCmd.Run(ctx, exportOpts)
	if err != nil {
		fmt.Printf("\nError: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\n✓ Export command succeeded")

	// Test 4: Storage stats
	fmt.Println("\n=== Database Statistics ===")
	stats, err := testStore.GetStats(ctx)
	if err != nil {
		fmt.Printf("Error getting stats: %v\n", err)
	} else {
		fmt.Printf("Total states: %d\n", stats.TotalStates)
		fmt.Printf("Unique vehicles: %d\n", stats.UniqueVehicles)
		if stats.OldestState != nil {
			fmt.Printf("Oldest state: %s\n", stats.OldestState.Format(time.RFC3339))
		}
		if stats.NewestState != nil {
			fmt.Printf("Newest state: %s\n", stats.NewestState.Format(time.RFC3339))
		}
		fmt.Printf("Database size: %.2f KB\n", float64(stats.DatabaseSize)/1024)
	}

	fmt.Println("\n✓ All CLI commands tested successfully!")
}
