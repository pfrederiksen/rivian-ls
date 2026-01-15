package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/pfrederiksen/rivian-ls/internal/rivian"
)

func main() {
	email := flag.String("email", "", "Rivian account email")
	password := flag.String("password", "", "Rivian account password")
	baseURL := flag.String("url", "https://rivian.com", "Rivian API base URL")
	flag.Parse()

	if *email == "" || *password == "" {
		fmt.Println("Usage: test-api -email your@email.com -password yourpassword")
		fmt.Println("\nThis will test authentication and vehicle queries against the real Rivian API.")
		fmt.Println("Note: This uses an UNOFFICIAL API and may violate Rivian's ToS.")
		os.Exit(1)
	}

	fmt.Println("Testing Rivian API client...")
	fmt.Printf("Base URL: %s\n\n", *baseURL)

	client := rivian.NewHTTPClient(rivian.WithBaseURL(*baseURL))

	// Test authentication
	fmt.Println("1. Testing authentication...")
	err := client.Authenticate(context.Background(), *email, *password)
	if err != nil {
		if otpErr, ok := err.(*rivian.OTPRequiredError); ok {
			fmt.Printf("   ⚠️  OTP required (session: %s)\n", otpErr.SessionID)
			fmt.Print("   Enter OTP code: ")
			var otp string
			fmt.Scanln(&otp)

			err = client.SubmitOTP(context.Background(), otp)
			if err != nil {
				fmt.Printf("   ❌ OTP submission failed: %v\n", err)
				os.Exit(1)
			}
			fmt.Println("   ✅ OTP authentication successful")
		} else {
			fmt.Printf("   ❌ Authentication failed: %v\n", err)
			os.Exit(1)
		}
	} else {
		fmt.Println("   ✅ Authentication successful")
	}

	// Show credentials (for debugging)
	creds := client.GetCredentials()
	if creds != nil {
		fmt.Printf("   Access token: %s...\n", creds.AccessToken[:20])
		fmt.Printf("   Expires at: %s\n\n", creds.ExpiresAt.Format("2006-01-02 15:04:05"))
	}

	// Test vehicle listing
	fmt.Println("2. Testing GetVehicles...")
	vehicles, err := client.GetVehicles(context.Background())
	if err != nil {
		fmt.Printf("   ❌ GetVehicles failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("   ✅ Found %d vehicle(s)\n", len(vehicles))
	for i, v := range vehicles {
		fmt.Printf("   [%d] %s - %s (VIN: %s)\n", i+1, v.Name, v.Model, v.VIN)
	}

	if len(vehicles) == 0 {
		fmt.Println("\n✅ Authentication works, but no vehicles found.")
		return
	}

	// Test vehicle state
	fmt.Printf("\n3. Testing GetVehicleState for vehicle: %s...\n", vehicles[0].ID)
	state, err := client.GetVehicleState(context.Background(), vehicles[0].ID)
	if err != nil {
		fmt.Printf("   ❌ GetVehicleState failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("   ✅ Vehicle state retrieved")
	fmt.Printf("   Vehicle ID: %s\n", state.VehicleID)
	fmt.Printf("   Battery: %.1f%%\n", state.BatteryLevel)
	fmt.Printf("   Range: %.1f miles\n", state.RangeEstimate)
	fmt.Printf("   Charge State: %s\n", state.ChargeState)
	fmt.Printf("   Locked: %v\n", state.IsLocked)
	fmt.Printf("   Online: %v\n", state.IsOnline)
	if state.CabinTemp != nil {
		fmt.Printf("   Cabin Temp: %.1f°F\n", *state.CabinTemp)
	}
	fmt.Printf("   Tire Pressures: FL=%.1f FR=%.1f RL=%.1f RR=%.1f PSI\n",
		state.TirePressures.FrontLeft,
		state.TirePressures.FrontRight,
		state.TirePressures.RearLeft,
		state.TirePressures.RearRight,
	)

	// Dump full state as JSON for inspection
	fmt.Println("\n4. Full vehicle state (JSON):")
	stateJSON, _ := json.MarshalIndent(state, "   ", "  ")
	fmt.Println(string(stateJSON))

	fmt.Println("\n✅ All tests passed!")
}
