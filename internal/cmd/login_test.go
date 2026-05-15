package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dorin/eero-cli/internal/api"
	"github.com/dorin/eero-cli/internal/config"
)

func TestLoginUnknownMentionsAmazonLogin(t *testing.T) {
	mock := &mockClient{
		LoginFn: func(identity string) (*api.LoginResponse, error) {
			return nil, &api.RequestError{StatusCode: 404, APIError: "error.login.unknown"}
		},
	}
	app := newTestApp(mock)

	var err error
	captureStdoutWithInput(t, "amazon@example.com\n", func() {
		err = app.Login()
	})

	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	for _, want := range []string{"Amazon Login", "import-token", "mobile API session token"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error missing %q:\n%s", want, msg)
		}
	}
}

func TestRequestLoginCode(t *testing.T) {
	var gotIdentity string
	mock := &mockClient{
		LoginFn: func(identity string) (*api.LoginResponse, error) {
			gotIdentity = identity
			return &api.LoginResponse{UserToken: "temporary-user-token"}, nil
		},
	}
	app := newTestApp(mock)
	app.Config.Token = ""

	out := captureStdout(t, func() {
		if err := app.RequestLoginCode([]string{"admin@example.com"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if gotIdentity != "admin@example.com" {
		t.Errorf("identity = %q", gotIdentity)
	}
	if app.Config.Token != "" {
		t.Errorf("RequestLoginCode should not save a token, got %q", app.Config.Token)
	}
	for _, want := range []string{"verification code has been sent", "login verify"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestImportTokenValidatesAndSavesNetwork(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	var gotToken string
	mock := &mockClient{
		SetTokenFn: func(token string) {
			gotToken = token
		},
		ValidateTokenFn: func() bool {
			return gotToken == "mobile-token-123"
		},
		GetAccountFn: func() (*api.Account, error) {
			account := &api.Account{}
			account.Networks.Data = []api.Network{{
				URL:  "/2.2/networks/abc123",
				Name: "Home",
			}}
			return account, nil
		},
	}
	app := &App{Config: &config.Config{}, Client: mock}

	out := captureStdout(t, func() {
		if err := app.ImportToken([]string{"mobile-token-123"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if app.Config.Token != "mobile-token-123" {
		t.Errorf("Token = %q", app.Config.Token)
	}
	if app.Config.NetworkID != "abc123" {
		t.Errorf("NetworkID = %q", app.Config.NetworkID)
	}
	if !strings.Contains(out, "Token imported and validated") {
		t.Errorf("output = %q", out)
	}

	path, err := config.ConfigPath()
	if err != nil {
		t.Fatalf("ConfigPath: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected saved config at %s: %v", path, err)
	}
}

func TestImportTokenRejectsInvalidToken(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	mock := &mockClient{
		SetTokenFn:      func(token string) {},
		ValidateTokenFn: func() bool { return false },
	}
	app := &App{Config: &config.Config{}, Client: mock}

	err := app.ImportToken([]string{"bad-token"})
	if err == nil || !strings.Contains(err.Error(), "invalid or expired") {
		t.Fatalf("expected invalid token error, got %v", err)
	}

	path, _ := config.ConfigPath()
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Fatalf("config should not be saved, stat err: %v", statErr)
	}
}

func TestImportTokenSavesEvenWhenAccountFetchFails(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	mock := &mockClient{
		SetTokenFn:      func(token string) {},
		ValidateTokenFn: func() bool { return true },
		GetAccountFn: func() (*api.Account, error) {
			return nil, fmt.Errorf("temporary account failure")
		},
	}
	app := &App{Config: &config.Config{}, Client: mock}

	out := captureStdout(t, func() {
		if err := app.ImportToken([]string{"mobile-token-123"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(out, "Warning") {
		t.Errorf("output = %q", out)
	}
	path, _ := config.ConfigPath()
	if filepath.Base(path) != "config.json" {
		t.Fatalf("unexpected config path: %s", path)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected saved config: %v", err)
	}
}
