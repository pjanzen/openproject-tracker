package auth

import (
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"sync"
	"time"

	"github.com/playwright-community/playwright-go"

	"github.com/pjanzen/openproject-tracker/internal/storage"
)

// SerializableCookie holds cookie data that can be JSON serialized.
type SerializableCookie struct {
	Name     string    `json:"name"`
	Value    string    `json:"value"`
	Domain   string    `json:"domain"`
	Path     string    `json:"path"`
	Expires  time.Time `json:"expires,omitempty"`
	HttpOnly bool      `json:"http_only"`
	Secure   bool      `json:"secure"`
	SameSite string    `json:"same_site,omitempty"`
}

// PersistentJar implements http.CookieJar with JSON persistence.
type PersistentJar struct {
	mu      sync.Mutex
	inner   http.CookieJar
	cookies []SerializableCookie
}

// NewPersistentJar creates a new PersistentJar.
func NewPersistentJar() *PersistentJar {
	inner, _ := cookiejar.New(nil)
	return &PersistentJar{inner: inner}
}

// Load reads cookies from a JSON file and populates the jar.
func (j *PersistentJar) Load(path string) error {
	j.mu.Lock()
	defer j.mu.Unlock()

	var cookies []SerializableCookie
	if err := storage.ReadJSON(path, &cookies); err != nil {
		return err
	}
	j.cookies = cookies

	// Rebuild inner jar from stored cookies.
	byURL := make(map[string][]*http.Cookie)
	for _, sc := range cookies {
		scheme := "https"
		if !sc.Secure {
			scheme = "http"
		}
		// Normalize the URL to just host.
		hostURL := &url.URL{Scheme: scheme, Host: sc.Domain}
		c := serializableToHTTP(sc)
		byURL[hostURL.String()] = append(byURL[hostURL.String()], c)
	}
	for rawURL, cookies := range byURL {
		u, err := url.Parse(rawURL)
		if err != nil {
			continue
		}
		j.inner.SetCookies(u, cookies)
	}
	return nil
}

// Save writes all cookies to a JSON file.
func (j *PersistentJar) Save(path string) error {
	j.mu.Lock()
	defer j.mu.Unlock()
	return storage.WriteJSON(path, j.cookies)
}

// SetCookies implements http.CookieJar.
func (j *PersistentJar) SetCookies(u *url.URL, cookies []*http.Cookie) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.inner.SetCookies(u, cookies)
	for _, c := range cookies {
		j.upsertCookie(u, c)
	}
}

// Cookies implements http.CookieJar.
func (j *PersistentJar) Cookies(u *url.URL) []*http.Cookie {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.inner.Cookies(u)
}

// SetFromPlaywright imports cookies exported from a Playwright browser context.
func (j *PersistentJar) SetFromPlaywright(pwCookies []playwright.Cookie) {
	j.mu.Lock()
	defer j.mu.Unlock()

	j.cookies = nil
	inner, _ := cookiejar.New(nil)
	j.inner = inner

	for _, pc := range pwCookies {
		sc := playwrightToSerializable(pc)
		j.cookies = append(j.cookies, sc)

		scheme := "https"
		if !sc.Secure {
			scheme = "http"
		}
		hostURL := &url.URL{Scheme: scheme, Host: sc.Domain}
		j.inner.SetCookies(hostURL, []*http.Cookie{serializableToHTTP(sc)})
	}
}

// upsertCookie updates or appends a cookie in the serializable slice.
// Must be called with j.mu held.
func (j *PersistentJar) upsertCookie(u *url.URL, c *http.Cookie) {
	domain := c.Domain
	if domain == "" {
		domain = u.Hostname()
	}
	path := c.Path
	if path == "" {
		path = "/"
	}
	for i, sc := range j.cookies {
		if sc.Name == c.Name && sc.Domain == domain && sc.Path == path {
			j.cookies[i] = httpToSerializable(c, domain)
			return
		}
	}
	j.cookies = append(j.cookies, httpToSerializable(c, domain))
}

func httpToSerializable(c *http.Cookie, domain string) SerializableCookie {
	sc := SerializableCookie{
		Name:     c.Name,
		Value:    c.Value,
		Domain:   domain,
		Path:     c.Path,
		HttpOnly: c.HttpOnly,
		Secure:   c.Secure,
	}
	if !c.Expires.IsZero() {
		sc.Expires = c.Expires
	}
	switch c.SameSite {
	case http.SameSiteStrictMode:
		sc.SameSite = "Strict"
	case http.SameSiteLaxMode:
		sc.SameSite = "Lax"
	case http.SameSiteNoneMode:
		sc.SameSite = "None"
	}
	return sc
}

func serializableToHTTP(sc SerializableCookie) *http.Cookie {
	c := &http.Cookie{
		Name:     sc.Name,
		Value:    sc.Value,
		Domain:   sc.Domain,
		Path:     sc.Path,
		HttpOnly: sc.HttpOnly,
		Secure:   sc.Secure,
	}
	if !sc.Expires.IsZero() {
		c.Expires = sc.Expires
	}
	switch sc.SameSite {
	case "Strict":
		c.SameSite = http.SameSiteStrictMode
	case "Lax":
		c.SameSite = http.SameSiteLaxMode
	case "None":
		c.SameSite = http.SameSiteNoneMode
	}
	return c
}

func playwrightToSerializable(pc playwright.Cookie) SerializableCookie {
	sc := SerializableCookie{
		Name:     pc.Name,
		Value:    pc.Value,
		Domain:   pc.Domain,
		Path:     pc.Path,
		HttpOnly: pc.HttpOnly,
		Secure:   pc.Secure,
	}
	if pc.SameSite != nil {
		sc.SameSite = string(*pc.SameSite)
	}
	if pc.Expires > 0 {
		sc.Expires = time.Unix(int64(pc.Expires), 0)
	}
	return sc
}
