package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/pfrederiksen/rivian-ls/internal/auth"
	"github.com/pfrederiksen/rivian-ls/internal/model"
	"github.com/pfrederiksen/rivian-ls/internal/rivian"
)

func main() {
	ctx := context.Background()

	// Load cached credentials
	credCache, err := auth.NewCredentialsCache()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	cached, err := credCache.Load()
	if err != nil || cached == nil {
		fmt.Println("No cached credentials found")
		os.Exit(1)
	}

	// Create client
	client := rivian.NewHTTPClient()
	client.SetCredentials(cached.ToRivianCredentials())

	// Get vehicles
	vehicles, err := client.GetVehicles(ctx)
	if err != nil || len(vehicles) == 0 {
		fmt.Printf("Error getting vehicles: %v\n", err)
		os.Exit(1)
	}

	vehicle := vehicles[0]
	fmt.Printf("Vehicle: %s (%s)\n\n", vehicle.Name, vehicle.Model)

	// Get raw API response
	rivState, err := client.GetVehicleState(ctx, vehicle.ID)
	if err != nil {
		fmt.Printf("Error getting state: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("=== Raw API Response ===")
	data, _ := json.MarshalIndent(rivState, "", "  ")
	fmt.Println(string(data))

	// Convert to domain model
	domainState := model.FromRivianVehicleState(rivState)

	fmt.Println("\n=== Domain Model ===")
	fmt.Printf("ChargeState: %q\n", domainState.ChargeState)
	fmt.Printf("BatteryLevel: %.1f%%\n", domainState.BatteryLevel)
	fmt.Printf("RangeEstimate: %.0f mi\n", domainState.RangeEstimate)
	fmt.Printf("IsLocked: %v\n", domainState.IsLocked)
	fmt.Printf("IsOnline: %v\n", domainState.IsOnline)
}
