package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/pfrederiksen/rivian-ls/internal/rivian"
	"golang.org/x/term"
)

func main() {
	email := flag.String("email", "", "Email address")
	password := flag.String("password", "", "Password (will prompt if not provided)")
	flag.Parse()

	if *email == "" {
		fmt.Println("Usage: test-cli-debug -email your@email.com [-password yourpassword]")
		fmt.Println()
		fmt.Println("This debug tool tests ONLY authentication to help diagnose issues.")
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

	fmt.Println("=== Step 1: Creating CSRF Token ===")
	fmt.Printf("Email: %s\n", *email)
	fmt.Println("Calling CreateCSRFToken mutation...")

	// We can't directly access internal methods, so we'll just call Authenticate
	// and see what error we get
	fmt.Println("\n=== Step 2: Authenticating ===")
	err := client.Authenticate(ctx, *email, *password)

	if err != nil {
		// Check if it's OTP required
		if otpErr, ok := err.(*rivian.OTPRequiredError); ok {
			fmt.Println("✓ Login succeeded - OTP required")
			fmt.Printf("OTP Session ID: %s\n", otpErr.SessionID)

			fmt.Print("\nEnter OTP code: ")
			scanner := bufio.NewScanner(os.Stdin)
			scanner.Scan()
			otpCode := strings.TrimSpace(scanner.Text())

			fmt.Println("\n=== Step 3: Submitting OTP ===")
			err = client.SubmitOTP(ctx, otpCode)
			if err != nil {
				fmt.Printf("✗ OTP submission failed: %v\n", err)
				os.Exit(1)
			}

			fmt.Println("✓ OTP accepted")
		} else {
			fmt.Printf("✗ Authentication failed: %v\n", err)
			fmt.Println()
			fmt.Println("Common issues:")
			fmt.Println("1. Wrong email or password")
			fmt.Println("2. Make sure you're using your REAL Rivian account email (not 'your@email.com')")
			fmt.Println("3. Account might have restrictions")
			fmt.Println()
			fmt.Println("Error details:")
			fmt.Printf("  %v\n", err)
			os.Exit(1)
		}
	}

	// Check if authenticated
	if !client.IsAuthenticated() {
		fmt.Println("✗ Not authenticated after login")
		os.Exit(1)
	}

	fmt.Println("✓ Fully authenticated!")

	// Try to get credentials
	creds := client.GetCredentials()
	if creds != nil {
		fmt.Printf("\nTokens received:\n")
		fmt.Printf("  Access token: %s...\n", creds.AccessToken[:20])
		fmt.Printf("  Refresh token: %s...\n", creds.RefreshToken[:20])
		fmt.Printf("  Expires: %s\n", creds.ExpiresAt.Format("2006-01-02 15:04:05"))
	}

	// Test getting vehicles
	fmt.Println("\n=== Testing GetVehicles ===")
	vehicles, err := client.GetVehicles(ctx)
	if err != nil {
		fmt.Printf("✗ GetVehicles failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Found %d vehicle(s):\n", len(vehicles))
	for i, v := range vehicles {
		fmt.Printf("  [%d] %s (%s %s) - VIN: %s\n", i, v.Name, v.Model, v.VIN, v.ID)
	}

	fmt.Println("\n✓ All authentication steps successful!")
}
