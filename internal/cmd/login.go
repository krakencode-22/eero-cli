package cmd

import (
	"fmt"
	"strings"

	"github.com/dorin/eero-cli/internal/api"
	"github.com/dorin/eero-cli/internal/config"
)

// Login handles the login command
func (a *App) Login(args ...string) error {
	if len(args) > 0 {
		switch args[0] {
		case "import-token":
			return a.ImportToken(args[1:])
		case "request-code":
			return a.RequestLoginCode(args[1:])
		case "verify":
			return a.LoginWithCode(args[1:])
		default:
			return fmt.Errorf("unknown login subcommand: %s", args[0])
		}
	}

	identity := Prompt("Enter your email or phone number: ")
	if identity == "" {
		return fmt.Errorf("email or phone number is required")
	}

	fmt.Println("Requesting verification code...")

	loginResp, err := a.Client.Login(identity)
	if err != nil {
		if api.IsAPIError(err, "error.login.unknown") {
			return fmt.Errorf("login failed: %w\n\nThis account was not recognized by the legacy email/phone code flow. If this eero account was switched to Amazon Login, eero disables the previous eero credentials for that account. This CLI cannot exchange Amazon web login cookies for a mobile API session token. Use a non-Amazon admin login if available, or run 'eero-cli login import-token' with a valid mobile API token obtained outside this CLI", err)
		}
		return fmt.Errorf("login failed: %w", err)
	}

	fmt.Println("A verification code has been sent to your email/phone.")
	code := PromptSecret("Enter verification code: ")
	if code == "" {
		return fmt.Errorf("verification code is required")
	}

	fmt.Println("Verifying...")

	if err := a.Client.LoginVerify(loginResp.UserToken, code); err != nil {
		return fmt.Errorf("verification failed: %w", err)
	}

	return a.saveVerifiedLogin(loginResp.UserToken)
}

// RequestLoginCode starts the email/phone flow without consuming the code.
func (a *App) RequestLoginCode(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: eero-cli login request-code <email-or-phone>")
	}

	identity := strings.TrimSpace(args[0])
	if identity == "" {
		return fmt.Errorf("email or phone number is required")
	}

	fmt.Println("Requesting verification code...")
	if _, err := a.Client.Login(identity); err != nil {
		if api.IsAPIError(err, "error.login.unknown") {
			return fmt.Errorf("login failed: %w\n\nThis account was not recognized by the legacy email/phone code flow. If this eero account was switched to Amazon Login, eero disables the previous eero credentials for that account. This CLI cannot exchange Amazon web login cookies for a mobile API session token. Use a non-Amazon admin login if available, or run 'eero-cli login import-token' with a valid mobile API token obtained outside this CLI", err)
		}
		return fmt.Errorf("login failed: %w", err)
	}

	fmt.Println("A verification code has been sent to your email/phone.")
	fmt.Println("When it arrives, run: eero-cli login verify <email-or-phone> <code>")
	return nil
}

// LoginWithCode completes the email/phone flow non-interactively.
func (a *App) LoginWithCode(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: eero-cli login verify <email-or-phone> <code>")
	}

	identity := strings.TrimSpace(args[0])
	code := strings.TrimSpace(args[1])
	if identity == "" {
		return fmt.Errorf("email or phone number is required")
	}
	if code == "" {
		return fmt.Errorf("verification code is required")
	}

	fmt.Println("Requesting verification code...")
	loginResp, err := a.Client.Login(identity)
	if err != nil {
		return fmt.Errorf("login failed: %w", err)
	}

	fmt.Println("Verifying...")
	if err := a.Client.LoginVerify(loginResp.UserToken, code); err != nil {
		return fmt.Errorf("verification failed: %w", err)
	}

	return a.saveVerifiedLogin(loginResp.UserToken)
}

// ImportToken validates and saves an existing mobile API session token.
func (a *App) ImportToken(args []string) error {
	var token string
	if len(args) > 0 {
		token = strings.TrimSpace(args[0])
	} else {
		token = PromptSecret("Enter mobile API session token: ")
	}
	if token == "" {
		return fmt.Errorf("token is required")
	}

	a.Client.SetToken(token)
	if !a.Client.ValidateToken() {
		return fmt.Errorf("token is invalid or expired")
	}

	a.Config.Token = token
	account, err := a.Client.GetAccount()
	if err != nil {
		if err := a.Config.Save(); err != nil {
			return fmt.Errorf("saving config: %w", err)
		}
		fmt.Println("Token imported! (Warning: couldn't fetch network info)")
		return nil
	}

	if len(account.Networks.Data) > 0 {
		a.Config.NetworkID = api.ExtractNetworkID(account.Networks.Data[0].URL)
		fmt.Printf("Imported token for network: %s\n", account.Networks.Data[0].Name)
	}

	if err := a.Config.Save(); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	fmt.Println("Token imported and validated.")
	return nil
}

func (a *App) saveVerifiedLogin(token string) error {
	a.Config.Token = token
	a.Client.SetToken(token)

	account, err := a.Client.GetAccount()
	if err != nil {
		if err := a.Config.Save(); err != nil {
			return fmt.Errorf("saving config: %w", err)
		}
		fmt.Println("Login successful! (Warning: couldn't fetch network info)")
		return nil
	}

	if len(account.Networks.Data) > 0 {
		a.Config.NetworkID = api.ExtractNetworkID(account.Networks.Data[0].URL)
		fmt.Printf("Logged in to network: %s\n", account.Networks.Data[0].Name)
	}

	if err := a.Config.Save(); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	fmt.Println("Login successful! Token saved.")
	return nil
}

// Logout handles the logout command
func (a *App) Logout() error {
	if err := a.Config.Clear(); err != nil {
		return fmt.Errorf("clearing config: %w", err)
	}
	fmt.Println("Logged out. Token cleared.")
	return nil
}

// Status shows the current authentication status
func (a *App) Status() error {
	path, _ := config.ConfigPath()

	if !a.Config.HasToken() {
		fmt.Println("Status: Not logged in")
		fmt.Printf("Config: %s\n", path)
		return nil
	}

	fmt.Println("Status: Checking token...")

	if !a.Client.ValidateToken() {
		fmt.Println("Status: Token is invalid or expired")
		fmt.Printf("Config: %s\n", path)
		return nil
	}

	account, err := a.Client.GetAccount()
	if err != nil {
		fmt.Println("Status: Authenticated (couldn't fetch account details)")
		fmt.Printf("Config: %s\n", path)
		return nil
	}

	fmt.Println("Status: Authenticated")
	if account.Email.Value != "" {
		fmt.Printf("Email: %s\n", account.Email.Value)
	}
	if account.Phone.Value != "" {
		fmt.Printf("Phone: %s\n", account.Phone.Value)
	}
	if account.Name != "" {
		fmt.Printf("Name: %s\n", account.Name)
	}
	if len(account.Networks.Data) > 0 {
		fmt.Println("Networks:")
		for _, n := range account.Networks.Data {
			networkID := api.ExtractNetworkID(n.URL)
			fmt.Printf("  - %s (ID: %s)\n", n.Name, networkID)
		}
	}
	fmt.Printf("Config: %s\n", path)

	return nil
}
