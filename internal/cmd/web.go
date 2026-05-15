package cmd

import (
	"fmt"
	"html"
	"io"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"time"
)

var webAccountURL = "https://account.eero.com/"

// Web handles account.eero.com helper commands. These commands use the web
// account session only; they do not authenticate mobile API device management.
func (a *App) Web(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("web subcommand is required: open or account")
	}

	switch args[0] {
	case "open":
		return openURL(webAccountURL)
	case "account":
		return a.WebAccount(args[1:])
	default:
		return fmt.Errorf("unknown web subcommand: %s", args[0])
	}
}

// WebAccount validates a supplied account.eero.com browser session cookie and
// prints the account details exposed by the web account page.
func (a *App) WebAccount(args []string) error {
	cookie, err := webCookieFromArgs(args)
	if err != nil {
		return err
	}

	details, err := FetchWebAccount(cookie)
	if err != nil {
		return err
	}

	fmt.Println("Web account session: authenticated")
	if details.FullName != "" {
		fmt.Printf("Full name: %s\n", details.FullName)
	}
	if details.Phone != "" {
		fmt.Printf("Mobile number: %s\n", details.Phone)
	}
	if details.Email != "" {
		fmt.Printf("Email: %s\n", details.Email)
	}
	if details.Network != "" {
		fmt.Printf("Network: %s\n", details.Network)
	}
	if details.CreatedOn != "" {
		fmt.Printf("Created on: %s\n", details.CreatedOn)
	}
	fmt.Println("Note: this web session cannot be used as a mobile API token for devices/profiles/guest commands.")
	return nil
}

type WebAccountDetails struct {
	FullName  string
	Phone     string
	Email     string
	Network   string
	CreatedOn string
}

func webCookieFromArgs(args []string) (string, error) {
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--cookie":
			if i+1 >= len(args) {
				return "", fmt.Errorf("--cookie requires a value")
			}
			return strings.TrimSpace(args[i+1]), nil
		case "--cookie-env":
			if i+1 >= len(args) {
				return "", fmt.Errorf("--cookie-env requires a variable name")
			}
			return strings.TrimSpace(os.Getenv(args[i+1])), nil
		}
	}

	if cookie := strings.TrimSpace(os.Getenv("EERO_WEB_COOKIE")); cookie != "" {
		return cookie, nil
	}

	cookie := PromptSecret("Enter account.eero.com Cookie header: ")
	if cookie == "" {
		return "", fmt.Errorf("web cookie is required")
	}
	return cookie, nil
}

func FetchWebAccount(cookie string) (*WebAccountDetails, error) {
	if strings.TrimSpace(cookie) == "" {
		return nil, fmt.Errorf("web cookie is required")
	}

	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest(http.MethodGet, webAccountURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating web account request: %w", err)
	}
	req.Header.Set("Cookie", cookie)
	req.Header.Set("User-Agent", "Mozilla/5.0 eero-cli web account checker")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching web account: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading web account response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("web account request failed with status %d", resp.StatusCode)
	}

	details := ParseWebAccountHTML(string(body))
	if details.Email == "" && details.Network == "" && !strings.Contains(string(body), "Your account") {
		return nil, fmt.Errorf("web session did not return an authenticated account page")
	}
	return details, nil
}

func ParseWebAccountHTML(page string) *WebAccountDetails {
	return &WebAccountDetails{
		FullName:  extractDefinition(page, "Full name"),
		Phone:     extractDefinition(page, "Mobile number"),
		Email:     extractDefinition(page, "Email"),
		Network:   extractDefinition(page, "Network"),
		CreatedOn: extractCreatedOn(page),
	}
}

func extractDefinition(page, label string) string {
	pattern := regexp.MustCompile(`(?is)<d[dt][^>]*>\s*` + regexp.QuoteMeta(label) + `\s*</d[dt]>\s*<dd[^>]*>(.*?)</dd>`)
	matches := pattern.FindStringSubmatch(page)
	if len(matches) < 2 {
		return ""
	}
	return cleanHTMLText(matches[1])
}

func extractCreatedOn(page string) string {
	pattern := regexp.MustCompile(`(?is)<dd[^>]*>\s*Created on\s+([^<]+)\s*</dd>`)
	matches := pattern.FindStringSubmatch(page)
	if len(matches) < 2 {
		return ""
	}
	return cleanHTMLText(matches[1])
}

func cleanHTMLText(s string) string {
	tags := regexp.MustCompile(`(?is)<[^>]+>`)
	space := regexp.MustCompile(`\s+`)
	cleaned := tags.ReplaceAllString(s, " ")
	cleaned = html.UnescapeString(cleaned)
	return strings.TrimSpace(space.ReplaceAllString(cleaned, " "))
}

func openURL(url string) error {
	var command string
	var args []string
	switch runtime.GOOS {
	case "darwin":
		command = "open"
		args = []string{url}
	case "windows":
		command = "rundll32"
		args = []string{"url.dll,FileProtocolHandler", url}
	default:
		command = "xdg-open"
		args = []string{url}
	}

	if err := exec.Command(command, args...).Start(); err != nil {
		return fmt.Errorf("opening %s: %w", url, err)
	}
	return nil
}
