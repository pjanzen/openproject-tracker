package client

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strings"

	"github.com/pjanzen/openproject-tracker/internal/auth"
	"github.com/pjanzen/openproject-tracker/internal/config"
)

// ErrUnauthorized is returned when the server rejects credentials.
var ErrUnauthorized = errors.New("unauthorized: please log in again")

// WorkPackage represents an OpenProject work package.
type WorkPackage struct {
	ID      int
	Subject string
	Project string
}

// Activity represents a time entry activity type.
type Activity struct {
	ID   int
	Name string
}

// TimeEntry holds the data needed to create a time entry.
type TimeEntry struct {
	WorkPackageID int
	ActivityID    int
	Hours         float64
	Comment       string
	SpentOn       string // YYYY-MM-DD
}

// Client is an OpenProject API v3 client.
type Client struct {
	cfg  *config.Config
	jar  *auth.PersistentJar
	http *http.Client
}

// NewClient constructs a Client using the given config and cookie jar.
func NewClient(cfg *config.Config, jar *auth.PersistentJar) *Client {
	transport := auth.BuildTransport(cfg)
	httpClient := &http.Client{
		Jar:       jar,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	return &Client{cfg: cfg, jar: jar, http: httpClient}
}

// CheckAuth verifies the session is valid by hitting /api/v3/my_preferences.
func (c *Client) CheckAuth() error {
	resp, err := c.get("/api/v3/my_preferences")
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

// GetMyWorkPackages returns work packages assigned to the current user.
func (c *Client) GetMyWorkPackages() ([]WorkPackage, error) {
	filters := `[{"assignee":{"operator":"=","values":["me"]}}]`
	params := url.Values{}
	params.Set("filters", filters)
	params.Set("pageSize", "500")

	resp, err := c.get("/api/v3/work_packages?" + params.Encode())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result struct {
		Embedded struct {
			Elements []struct {
				ID      int    `json:"id"`
				Subject string `json:"subject"`
				Links   struct {
					Project struct {
						Title string `json:"title"`
					} `json:"project"`
				} `json:"_links"`
			} `json:"elements"`
		} `json:"_embedded"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse work packages: %w", err)
	}

	var wps []WorkPackage
	for _, el := range result.Embedded.Elements {
		wps = append(wps, WorkPackage{
			ID:      el.ID,
			Subject: el.Subject,
			Project: el.Links.Project.Title,
		})
	}
	return wps, nil
}

// GetActivities returns available time entry activity types.
func (c *Client) GetActivities() ([]Activity, error) {
	resp, err := c.get("/api/v3/time_entries/activities")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result struct {
		Embedded struct {
			Elements []struct {
				ID   int    `json:"id"`
				Name string `json:"name"`
			} `json:"elements"`
		} `json:"_embedded"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse activities: %w", err)
	}

	var activities []Activity
	for _, el := range result.Embedded.Elements {
		activities = append(activities, Activity{ID: el.ID, Name: el.Name})
	}
	return activities, nil
}

// CreateTimeEntry posts a new time entry to OpenProject.
func (c *Client) CreateTimeEntry(te TimeEntry) error {
	duration := hoursToISO8601(te.Hours)

	payload := map[string]any{
		"_links": map[string]any{
			"workPackage": map[string]string{
				"href": fmt.Sprintf("/api/v3/work_packages/%d", te.WorkPackageID),
			},
			"activity": map[string]string{
				"href": fmt.Sprintf("/api/v3/time_entries/activities/%d", te.ActivityID),
			},
		},
		"hours":   duration,
		"comment": map[string]string{"raw": te.Comment},
		"spentOn": te.SpentOn,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, c.cfg.BaseURL+"/api/v3/time_entries", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if err := c.checkResponse(resp); err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("create time entry: HTTP %d: %s", resp.StatusCode, string(b))
	}
	return nil
}

// get performs a GET request to path (relative to BaseURL) and checks the response.
func (c *Client) get(path string) (*http.Response, error) {
	reqURL := c.cfg.BaseURL + path
	req, err := http.NewRequest(http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	if err := c.checkResponse(resp); err != nil {
		resp.Body.Close()
		return nil, err
	}
	return resp, nil
}

// ssoRedirectPatterns are substrings used to detect SSO redirect responses.
var ssoRedirectPatterns = []string{"outpost.goauthentik.io", "authentik"}

// checkResponse returns ErrUnauthorized for 401 or SSO redirects, otherwise nil.
func (c *Client) checkResponse(resp *http.Response) error {
	if resp.StatusCode == http.StatusUnauthorized {
		return ErrUnauthorized
	}
	if resp.StatusCode == http.StatusFound || resp.StatusCode == http.StatusMovedPermanently {
		loc := resp.Header.Get("Location")
		for _, pattern := range ssoRedirectPatterns {
			if strings.Contains(loc, pattern) {
				return ErrUnauthorized
			}
		}
	}
	return nil
}

// hoursToISO8601 converts a float64 number of hours to an ISO 8601 duration string.
// e.g. 1.5 -> "PT1H30M", 0.75 -> "PT45M"
func hoursToISO8601(hours float64) string {
	totalMinutes := int(math.Round(hours * 60))
	h := totalMinutes / 60
	m := totalMinutes % 60
	if h > 0 && m > 0 {
		return fmt.Sprintf("PT%dH%dM", h, m)
	}
	if h > 0 {
		return fmt.Sprintf("PT%dH", h)
	}
	return fmt.Sprintf("PT%dM", m)
}
