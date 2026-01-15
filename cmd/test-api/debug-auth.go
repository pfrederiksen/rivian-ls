package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
)

func main() {
	email := flag.String("email", "", "Rivian account email")
	password := flag.String("password", "", "Rivian account password")
	flag.Parse()

	if *email == "" || *password == "" {
		fmt.Println("Usage: debug-auth -email your@email.com -password yourpassword")
		os.Exit(1)
	}

	fmt.Println("=== STEP 1: CreateCSRFToken ===")
	csrfReq := map[string]interface{}{
		"query": `mutation CreateCSRFToken {
			createCsrfToken {
				__typename
				csrfToken
				appSessionToken
			}
		}`,
	}

	csrfBody, _ := json.MarshalIndent(csrfReq, "", "  ")
	fmt.Printf("Request body:\n%s\n\n", string(csrfBody))

	req, _ := http.NewRequestWithContext(context.Background(), "POST", "https://rivian.com/api/gql/gateway/graphql", bytes.NewReader(csrfBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "rivian-ls/0.1.0")
	req.Header.Set("apollographql-client-name", "com.rivian.android.consumer")

	fmt.Println("Request headers:")
	for k, v := range req.Header {
		fmt.Printf("  %s: %s\n", k, v)
	}
	fmt.Println()

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("❌ Error: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	fmt.Printf("Response status: %d\n", resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	fmt.Printf("Response body:\n%s\n\n", string(body))

	if resp.StatusCode != 200 {
		os.Exit(1)
	}

	var csrfResp map[string]interface{}
	json.Unmarshal(body, &csrfResp)

	data, ok := csrfResp["data"].(map[string]interface{})
	if !ok {
		fmt.Println("❌ No data in response")
		os.Exit(1)
	}

	csrfData, ok := data["createCsrfToken"].(map[string]interface{})
	if !ok {
		fmt.Println("❌ No createCsrfToken in response")
		os.Exit(1)
	}

	csrfToken := csrfData["csrfToken"].(string)
	appSessionToken := csrfData["appSessionToken"].(string)

	fmt.Printf("✅ Got tokens:\n")
	fmt.Printf("  CSRF Token: %s\n", csrfToken)
	fmt.Printf("  App Session Token: %s\n\n", appSessionToken)

	fmt.Println("=== STEP 2: Login ===")
	loginReq := map[string]interface{}{
		"query": `mutation Login($email: String!, $password: String!) {
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
		}`,
		"variables": map[string]interface{}{
			"email":    *email,
			"password": *password,
		},
	}

	loginBody, _ := json.MarshalIndent(loginReq, "", "  ")
	fmt.Printf("Request body:\n%s\n\n", string(loginBody))

	req2, _ := http.NewRequestWithContext(context.Background(), "POST", "https://rivian.com/api/gql/gateway/graphql", bytes.NewReader(loginBody))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("User-Agent", "rivian-ls/0.1.0")
	req2.Header.Set("apollographql-client-name", "com.rivian.android.consumer")
	req2.Header.Set("a-sess", appSessionToken)
	req2.Header.Set("csrf-token", csrfToken)

	fmt.Println("Request headers:")
	for k, v := range req2.Header {
		fmt.Printf("  %s: %s\n", k, v)
	}
	fmt.Println()

	resp2, err := client.Do(req2)
	if err != nil {
		fmt.Printf("❌ Error: %v\n", err)
		os.Exit(1)
	}
	defer resp2.Body.Close()

	fmt.Printf("Response status: %d\n", resp2.StatusCode)
	body2, _ := io.ReadAll(resp2.Body)

	var prettyJSON bytes.Buffer
	json.Indent(&prettyJSON, body2, "", "  ")
	fmt.Printf("Response body:\n%s\n\n", prettyJSON.String())

	var loginResp map[string]interface{}
	json.Unmarshal(body2, &loginResp)

	loginData, ok := loginResp["data"].(map[string]interface{})
	if !ok {
		fmt.Println("❌ No data in login response")
		os.Exit(1)
	}

	login, ok := loginData["login"].(map[string]interface{})
	if !ok {
		fmt.Println("❌ No login in response")
		os.Exit(1)
	}

	typename := login["__typename"].(string)
	fmt.Printf("Response type: %s\n\n", typename)

	if typename == "MobileMFALoginResponse" {
		otpToken := login["otpToken"].(string)
		fmt.Printf("⚠️  MFA Required\n")
		fmt.Printf("OTP Token: %s...\n\n", otpToken[:50])

		fmt.Print("Enter the OTP code sent to your phone: ")
		var otpCode string
		fmt.Scanln(&otpCode)

		fmt.Println("\n=== STEP 3: LoginWithOTP ===")
		otpReq := map[string]interface{}{
			"query": `mutation LoginWithOTP($email: String!, $otpCode: String!, $otpToken: String!) {
				loginWithOTP(email: $email, otpCode: $otpCode, otpToken: $otpToken) {
					__typename
					accessToken
					refreshToken
					userSessionToken
				}
			}`,
			"variables": map[string]interface{}{
				"email":    *email,
				"otpCode":  otpCode,
				"otpToken": otpToken,
			},
		}

		otpBody, _ := json.MarshalIndent(otpReq, "", "  ")
		fmt.Printf("Request body:\n%s\n\n", string(otpBody))

		req3, _ := http.NewRequestWithContext(context.Background(), "POST", "https://rivian.com/api/gql/gateway/graphql", bytes.NewReader(otpBody))
		req3.Header.Set("Content-Type", "application/json")
		req3.Header.Set("User-Agent", "rivian-ls/0.1.0")
		req3.Header.Set("apollographql-client-name", "com.rivian.android.consumer")
		req3.Header.Set("a-sess", appSessionToken)
		req3.Header.Set("csrf-token", csrfToken)

		fmt.Println("Request headers:")
		for k, v := range req3.Header {
			fmt.Printf("  %s: %s\n", k, v)
		}
		fmt.Println()

		resp3, err := client.Do(req3)
		if err != nil {
			fmt.Printf("❌ Error: %v\n", err)
			os.Exit(1)
		}
		defer resp3.Body.Close()

		fmt.Printf("Response status: %d\n", resp3.StatusCode)
		body3, _ := io.ReadAll(resp3.Body)

		var prettyJSON3 bytes.Buffer
		json.Indent(&prettyJSON3, body3, "", "  ")
		fmt.Printf("Response body:\n%s\n\n", prettyJSON3.String())

		var otpResp map[string]interface{}
		json.Unmarshal(body3, &otpResp)

		if otpData, ok := otpResp["data"].(map[string]interface{}); ok {
			if loginOTP, ok := otpData["loginWithOTP"].(map[string]interface{}); ok {
				fmt.Println("✅ OTP Authentication Successful!")
				fmt.Printf("Access Token: %s...\n", loginOTP["accessToken"].(string)[:50])
				fmt.Printf("Refresh Token: %s...\n", loginOTP["refreshToken"].(string)[:50])
				fmt.Printf("User Session Token: %s\n", loginOTP["userSessionToken"].(string))
			}
		}
	} else if typename == "MobileLoginResponse" {
		fmt.Println("✅ Login Successful (No MFA)")
		fmt.Printf("Access Token: %s...\n", login["accessToken"].(string)[:50])
		fmt.Printf("Refresh Token: %s...\n", login["refreshToken"].(string)[:50])
		fmt.Printf("User Session Token: %s\n", login["userSessionToken"].(string))
	}
}
