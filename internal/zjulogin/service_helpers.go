package zjulogin

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
)

var metaRefreshRegexp = regexp.MustCompile(`meta http-equiv="refresh" content="0;URL=([^"]+)"`)

func followRedirectsAndMetaRefresh(ctx context.Context, client *CookieClient, startURL string) error {
	currentURL := startURL
	for {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, currentURL, nil)
		if err != nil {
			return err
		}

		res, err := client.Do(req)
		if err != nil {
			return err
		}
		body, readErr := io.ReadAll(res.Body)
		closeResponseBody(res)
		if readErr != nil {
			return readErr
		}

		if res.StatusCode == http.StatusOK {
			if next := findSubmatch(metaRefreshRegexp, string(body)); next != "" {
				currentURL = resolveLocation(currentURL, next)
				continue
			}
		}

		if res.StatusCode < http.StatusMultipleChoices || res.StatusCode >= http.StatusBadRequest {
			return nil
		}

		location := res.Header.Get("Location")
		if location == "" {
			return fmt.Errorf("redirect from %s has empty location", currentURL)
		}
		currentURL = resolveLocation(currentURL, location)
	}
}

func redirectToHost(ctx context.Context, client *CookieClient, startURL string, targetHost string) (string, error) {
	currentURL := startURL
	for {
		currentHost, err := hostName(currentURL)
		if err != nil {
			return "", err
		}
		if currentHost == targetHost {
			return currentURL, nil
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, currentURL, nil)
		if err != nil {
			return "", err
		}
		res, err := client.Do(req)
		if err != nil {
			return "", err
		}
		closeResponseBody(res)

		location := res.Header.Get("Location")
		if location == "" {
			return "", fmt.Errorf("redirect from %s has empty location", currentURL)
		}
		currentURL = resolveLocation(currentURL, location)
	}
}

func hostName(rawURL string) (string, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}
	return parsed.Hostname(), nil
}
