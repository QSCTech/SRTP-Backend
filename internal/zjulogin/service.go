package zjulogin

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
)

type Service interface {
	Login(ctx context.Context) error
	Do(req *http.Request) (*http.Response, error)
	Get(ctx context.Context, rawURL string) (*http.Response, error)
	Post(ctx context.Context, rawURL string, contentType string, body io.Reader) (*http.Response, error)
	PostForm(ctx context.Context, rawURL string, data url.Values) (*http.Response, error)
}

type TokenService interface {
	Service
	Token(ctx context.Context) (string, error)
}

type serviceCore struct {
	client *CookieClient
	login  func(context.Context) error

	mu       sync.Mutex
	loggedIn bool
}

func newServiceCore(login func(context.Context) error) (*serviceCore, error) {
	client, err := NewCookieClient()
	if err != nil {
		return nil, err
	}
	return &serviceCore{client: client, login: login}, nil
}

func (s *serviceCore) ensureLogin(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.loggedIn {
		return nil
	}
	if err := s.login(ctx); err != nil {
		return err
	}
	s.loggedIn = true
	return nil
}

func (s *serviceCore) resetLogin() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.loggedIn = false
}

func (s *serviceCore) Login(ctx context.Context) error {
	return s.ensureLogin(ctx)
}

func (s *serviceCore) Do(req *http.Request) (*http.Response, error) {
	if err := s.ensureLogin(req.Context()); err != nil {
		return nil, err
	}
	return s.client.Do(req)
}

func (s *serviceCore) Get(ctx context.Context, rawURL string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	return s.Do(req)
}

func (s *serviceCore) Post(ctx context.Context, rawURL string, contentType string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rawURL, body)
	if err != nil {
		return nil, err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	return s.Do(req)
}

func (s *serviceCore) PostForm(ctx context.Context, rawURL string, data url.Values) (*http.Response, error) {
	return s.Post(ctx, rawURL, "application/x-www-form-urlencoded", strings.NewReader(data.Encode()))
}

func closeResponseBody(res *http.Response) {
	if res != nil && res.Body != nil {
		_ = res.Body.Close()
	}
}
