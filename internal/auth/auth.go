package auth

import (
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"time"

	"github.com/playwright-community/playwright-go"

	"github.com/pjanzen/openproject-tracker/internal/config"
	"github.com/pjanzen/openproject-tracker/internal/storage"
)

// ErrLoginTimeout is returned when SSO login does not complete in time.
var ErrLoginTimeout = errors.New("SSO login timed out")

const (
	loginPollInterval = 2 * time.Second
	loginTimeout      = 5 * time.Minute
)

// Login opens a headful Chromium window and waits for the user to complete SSO.
// It polls /api/v3/my_preferences until it returns 200, then exports cookies.
func Login(cfg *config.Config, jar *PersistentJar) error {
	pw, err := playwright.Run()
	if err != nil {
		return fmt.Errorf("start playwright: %w", err)
	}
	defer pw.Stop() //nolint:errcheck

	headless := false
	browser, err := pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
		Headless: &headless,
	})
	if err != nil {
		return fmt.Errorf("launch browser: %w", err)
	}
	defer browser.Close()

	ctx, err := browser.NewContext(playwright.BrowserNewContextOptions{})
	if err != nil {
		return fmt.Errorf("new context: %w", err)
	}
	defer ctx.Close()

	page, err := ctx.NewPage()
	if err != nil {
		return fmt.Errorf("new page: %w", err)
	}

	if _, err := page.Goto(cfg.BaseURL); err != nil {
		return fmt.Errorf("navigate to base URL: %w", err)
	}

	ticker := time.NewTicker(loginPollInterval)
	defer ticker.Stop()
	timeout := time.After(loginTimeout)

	prefsURL := cfg.BaseURL + "/api/v3/my_preferences"

	for {
		select {
		case <-timeout:
			return ErrLoginTimeout
		case <-ticker.C:
			// Export cookies from browser and do a quick HTTP check.
			pwCookies, err := ctx.Cookies()
			if err != nil {
				continue
			}
			if authed := checkAuthWithCookies(prefsURL, pwCookies, cfg); authed {
				jar.SetFromPlaywright(pwCookies)
				dir, err := storage.ConfigDir()
				if err != nil {
					return fmt.Errorf("config dir: %w", err)
				}
				if err := jar.Save(filepath.Join(dir, "cookies.json")); err != nil {
					return fmt.Errorf("save cookies: %w", err)
				}
				return nil
			}
		}
	}
}

// checkAuthWithCookies does a GET to prefsURL using the provided playwright cookies
// and returns true if the response status is 200.
func checkAuthWithCookies(prefsURL string, pwCookies []playwright.Cookie, cfg *config.Config) bool {
	tmpJar := NewPersistentJar()
	tmpJar.SetFromPlaywright(pwCookies)

	transport := BuildTransport(cfg)
	client := &http.Client{
		Jar:       tmpJar,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	req, err := http.NewRequest(http.MethodGet, prefsURL, nil)
	if err != nil {
		return false
	}
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}
