package zjulogin

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
)

const (
	zjuamLoginURL  = "https://zjuam.zju.edu.cn/cas/login"
	zjuamPubKeyURL = "https://zjuam.zju.edu.cn/cas/v2/getPubKey"
)

var executionRegexp = regexp.MustCompile(`name="execution" value="([^"]+)"`)
var loginMessageRegexp = regexp.MustCompile(`<span id="msg">([^<]+)</span>`)

type ZJUAM struct {
	username string
	password string
	client   *CookieClient

	mu       sync.Mutex
	loggedIn bool
}

type zjuamPubKey struct {
	Modulus  string `json:"modulus"`
	Exponent string `json:"exponent"`
}

func NewZJUAM(cfg Config) (*ZJUAM, error) {
	client, err := NewCookieClient()
	if err != nil {
		return nil, err
	}
	return &ZJUAM{username: cfg.Username, password: cfg.Password, client: client}, nil
}

func (a *ZJUAM) Login(ctx context.Context) error {
	_, err := a.login(ctx, zjuamLoginURL)
	return err
}

func (a *ZJUAM) Do(req *http.Request) (*http.Response, error) {
	if err := a.ensureLogin(req.Context()); err != nil {
		return nil, err
	}
	return a.client.Do(req)
}

func (a *ZJUAM) Get(ctx context.Context, rawURL string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	return a.Do(req)
}

func (a *ZJUAM) Post(ctx context.Context, rawURL string, contentType string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rawURL, body)
	if err != nil {
		return nil, err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	return a.Do(req)
}

func (a *ZJUAM) PostForm(ctx context.Context, rawURL string, data url.Values) (*http.Response, error) {
	return a.Post(ctx, rawURL, "application/x-www-form-urlencoded", strings.NewReader(data.Encode()))
}

func (a *ZJUAM) LoginService(ctx context.Context, serviceURL string) (string, error) {
	fullLoginURL := zjuamLoginURL + "?service=" + url.QueryEscape(serviceURL)
	a.mu.Lock()
	loggedIn := a.loggedIn
	a.mu.Unlock()

	if !loggedIn {
		return a.login(ctx, fullLoginURL)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullLoginURL, nil)
	if err != nil {
		return "", err
	}
	res, err := a.client.Do(req)
	if err != nil {
		return "", err
	}
	defer closeResponseBody(res)

	switch {
	case res.StatusCode == http.StatusFound:
		location := res.Header.Get("Location")
		if location == "" {
			return "", fmt.Errorf("zjuam service login returned empty location")
		}
		return location, nil
	case res.StatusCode == http.StatusOK:
		return a.login(ctx, fullLoginURL)
	default:
		return "", fmt.Errorf("zjuam service login failed with status %d", res.StatusCode)
	}
}

func (a *ZJUAM) LoginOAuth2(ctx context.Context, redirectURL string) (string, error) {
	currentURL := redirectURL
	for {
		parsed, err := url.Parse(currentURL)
		if err != nil {
			return "", err
		}
		if parsed.Hostname() != "zjuam.zju.edu.cn" {
			return currentURL, nil
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, currentURL, nil)
		if err != nil {
			return "", err
		}
		res, err := a.Do(req)
		if err != nil {
			return "", err
		}
		location := res.Header.Get("Location")
		closeResponseBody(res)
		if location == "" {
			return "", fmt.Errorf("zjuam oauth2 login stopped at %s with status %d", currentURL, res.StatusCode)
		}
		currentURL = resolveLocation(currentURL, location)
	}
}

func (a *ZJUAM) ensureLogin(ctx context.Context) error {
	a.mu.Lock()
	loggedIn := a.loggedIn
	a.mu.Unlock()
	if loggedIn {
		return nil
	}

	_, err := a.login(ctx, zjuamLoginURL)
	return err
}

func (a *ZJUAM) login(ctx context.Context, loginURL string) (string, error) {
	a.mu.Lock()
	if a.loggedIn && loginURL == zjuamLoginURL {
		a.mu.Unlock()
		return "", nil
	}
	a.mu.Unlock()

	loginHTML, err := a.getText(ctx, loginURL)
	if err != nil {
		return "", fmt.Errorf("fetch zjuam login page: %w", err)
	}
	execution := findSubmatch(executionRegexp, loginHTML)
	if execution == "" {
		return "", fmt.Errorf("zjuam login page does not contain execution")
	}

	pubKey, err := a.getPubKey(ctx)
	if err != nil {
		return "", err
	}
	encryptedPassword, err := rsaEncryptPassword(a.password, pubKey.Exponent, pubKey.Modulus)
	if err != nil {
		return "", err
	}

	form := url.Values{}
	form.Set("username", a.username)
	form.Set("password", encryptedPassword)
	form.Set("execution", execution)
	form.Set("_eventId", "submit")
	form.Set("authcode", "")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, loginURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	res, err := a.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("post zjuam login: %w", err)
	}
	defer closeResponseBody(res)

	if res.StatusCode == http.StatusFound {
		location := res.Header.Get("Location")
		if location == "" {
			return "", fmt.Errorf("zjuam login returned empty location")
		}
		a.mu.Lock()
		a.loggedIn = true
		a.mu.Unlock()
		return location, nil
	}
	if res.StatusCode == http.StatusOK {
		body, _ := io.ReadAll(res.Body)
		message := findSubmatch(loginMessageRegexp, string(body))
		if message == "" {
			message = "unknown login failure"
		}
		return "", fmt.Errorf("zjuam login failed: %s", message)
	}
	return "", fmt.Errorf("zjuam login failed with status %d", res.StatusCode)
}

func (a *ZJUAM) getText(ctx context.Context, rawURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", err
	}
	res, err := a.client.Do(req)
	if err != nil {
		return "", err
	}
	defer closeResponseBody(res)

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func (a *ZJUAM) getPubKey(ctx context.Context) (zjuamPubKey, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, zjuamPubKeyURL, nil)
	if err != nil {
		return zjuamPubKey{}, err
	}
	res, err := a.client.Do(req)
	if err != nil {
		return zjuamPubKey{}, fmt.Errorf("fetch zjuam pubkey: %w", err)
	}
	defer closeResponseBody(res)

	var pubKey zjuamPubKey
	if err := json.NewDecoder(res.Body).Decode(&pubKey); err != nil {
		return zjuamPubKey{}, fmt.Errorf("decode zjuam pubkey: %w", err)
	}
	if pubKey.Modulus == "" || pubKey.Exponent == "" {
		return zjuamPubKey{}, fmt.Errorf("zjuam pubkey response is incomplete")
	}
	return pubKey, nil
}

func findSubmatch(re *regexp.Regexp, text string) string {
	matches := re.FindStringSubmatch(text)
	if len(matches) < 2 {
		return ""
	}
	return matches[1]
}

func resolveLocation(baseURL string, location string) string {
	base, err := url.Parse(baseURL)
	if err != nil {
		return location
	}
	next, err := url.Parse(location)
	if err != nil {
		return location
	}
	return base.ResolveReference(next).String()
}
