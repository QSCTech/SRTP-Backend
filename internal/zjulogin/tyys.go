package zjulogin

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

const (
	tyysAppKey            = "8fceb735082b5a529312040b58ea780b"
	tyysDefaultSignSecret = "c640ca392cd45fb3a55b00a63a86c618"
	tyysLoginAPI          = "http://tyys.zju.edu.cn/venue-server/api/login"
	tyysSSOLoginURL       = "http://tyys.zju.edu.cn/venue-server/sso/manageLogin"
	tyysReservationUI     = "http://tyys.zju.edu.cn/venue/reservation"
)

type TYYS struct {
	am         *ZJUAM
	signSecret string
	core       *serviceCore
	token      string
}

type tyysLoginResponse struct {
	Code    int             `json:"code"`
	Data    json.RawMessage `json:"data"`
	Message string          `json:"message"`
}

func NewTYYS(am *ZJUAM, signSecret string) (*TYYS, error) {
	if strings.TrimSpace(signSecret) == "" {
		signSecret = tyysDefaultSignSecret
	}

	s := &TYYS{
		am:         am,
		signSecret: signSecret,
	}
	core, err := newServiceCore(s.login)
	if err != nil {
		return nil, err
	}
	s.core = core
	return s, nil
}

func (s *TYYS) Login(ctx context.Context) error {
	return s.core.ensureLogin(ctx)
}

func (s *TYYS) Token(ctx context.Context) (string, error) {
	if err := s.Login(ctx); err != nil {
		return "", err
	}
	return s.token, nil
}

func (s *TYYS) Do(req *http.Request) (*http.Response, error) {
	if err := s.Login(req.Context()); err != nil {
		return nil, err
	}
	s.signRequest(req)
	return s.core.client.Do(req)
}

func (s *TYYS) Get(ctx context.Context, rawURL string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	return s.Do(req)
}

func (s *TYYS) Post(ctx context.Context, rawURL string, contentType string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rawURL, body)
	if err != nil {
		return nil, err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	return s.Do(req)
}

func (s *TYYS) PostForm(ctx context.Context, rawURL string, data url.Values) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rawURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if err := s.Login(ctx); err != nil {
		return nil, err
	}
	s.signRequestWithParams(req, data)
	return s.core.client.Do(req)
}

func (s *TYYS) VenueInfoURL(isArt int) string {
	values := url.Values{}
	values.Set("isArt", fmt.Sprintf("%d", isArt))
	return "http://tyys.zju.edu.cn/venue-server/api/reservation/campus/venue/info?" + values.Encode()
}

func (s *TYYS) login(ctx context.Context) error {
	callbackURL, err := s.am.LoginService(ctx, tyysSSOLoginURL)
	if err != nil {
		return err
	}

	oauthToken, err := s.followLoginRedirects(ctx, callbackURL)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tyysLoginAPI, bytes.NewReader(nil))
	if err != nil {
		return err
	}
	req.Header.Set("oauth-token", oauthToken)
	s.signRequestWithParams(req, nil)

	res, err := s.core.client.Do(req)
	if err != nil {
		return err
	}
	defer closeResponseBody(res)

	var data tyysLoginResponse
	if err := json.NewDecoder(res.Body).Decode(&data); err != nil {
		return err
	}
	if data.Code != http.StatusOK {
		return fmt.Errorf("tyys login failed with code %d: %s", data.Code, data.Message)
	}

	token, err := parseTYYSToken(data.Data)
	if err != nil {
		return err
	}
	s.token = token
	return nil
}

func (s *TYYS) followLoginRedirects(ctx context.Context, startURL string) (string, error) {
	currentURL := startURL
	for {
		parsed, err := url.Parse(currentURL)
		if err != nil {
			return "", err
		}
		if token := parsed.Query().Get("oauth_token"); token != "" {
			return token, nil
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, currentURL, nil)
		if err != nil {
			return "", err
		}
		res, err := s.core.client.Do(req)
		if err != nil {
			return "", err
		}
		location := res.Header.Get("Location")
		statusCode := res.StatusCode
		closeResponseBody(res)
		if location == "" {
			return "", fmt.Errorf("tyys login redirect stopped at %s with status %d", currentURL, statusCode)
		}
		currentURL = resolveLocation(currentURL, location)
	}
}

func (s *TYYS) signRequest(req *http.Request) {
	s.signRequestWithParams(req, nil)
}

func (s *TYYS) signRequestWithParams(req *http.Request, signParams url.Values) {
	timestamp := fmt.Sprintf("%d", time.Now().UnixMilli())
	params := signParams
	if params == nil {
		params = req.URL.Query()
	} else {
		params = cloneValues(params)
	}
	if req.Method == http.MethodGet {
		params.Set("nocache", timestamp)
		req.URL.RawQuery = params.Encode()
	}
	sign := tyysSign(s.signSecret, tyysSignPath(req.URL.Path), params, timestamp)

	req.Header.Set("app-key", tyysAppKey)
	req.Header.Set("timestamp", timestamp)
	req.Header.Set("sign", sign)
	if s.token != "" {
		// TYYS is backed by Chingo's gateway; some write APIs appear to check
		// the frontend's original mixed-case header name more strictly.
		req.Header["cgAuthorization"] = []string{s.token}
	}
	req.Header.Set("accept", "application/json, text/plain, */*")
	req.Header.Set("accept-language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("referer", tyysReservationUI)
	if req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
}

func cloneValues(values url.Values) url.Values {
	cloned := make(url.Values, len(values))
	for key, items := range values {
		cloned[key] = append([]string(nil), items...)
	}
	return cloned
}

func tyysSign(secret string, path string, params url.Values, timestamp string) string {
	raw := secret + path + sortedParams(params) + timestamp + " " + secret
	sum := md5.Sum([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func tyysSignPath(path string) string {
	if strings.HasPrefix(path, "/venue-server/") {
		return strings.TrimPrefix(path, "/venue-server")
	}
	return path
}

func sortedParams(params url.Values) string {
	if len(params) == 0 {
		return ""
	}

	keys := make([]string, 0, len(params))
	for key := range params {
		if isTYYSSignIgnoredParam(key) {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var builder strings.Builder
	for _, key := range keys {
		values := append([]string(nil), params[key]...)
		sort.Strings(values)
		for _, value := range values {
			if value == "" {
				continue
			}
			builder.WriteString(key)
			builder.WriteString(value)
		}
	}
	return builder.String()
}

func isTYYSSignIgnoredParam(key string) bool {
	switch key {
	case "gmtCreate", "gmtModified", "creator", "modifier", "id", "_index", "_rowKey":
		return true
	default:
		return false
	}
}

func parseTYYSToken(data json.RawMessage) (string, error) {
	var token string
	if err := json.Unmarshal(data, &token); err == nil && token != "" {
		return token, nil
	}

	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		return "", err
	}
	for _, key := range []string{"token", "accessToken", "authorization", "cgAuthorization", "cgauthorization"} {
		if value, ok := obj[key].(string); ok && value != "" {
			return value, nil
		}
	}
	if tokenObj, ok := obj["token"].(map[string]any); ok {
		for _, key := range []string{"access_token", "accessToken", "token"} {
			if value, ok := tokenObj[key].(string); ok && value != "" {
				return value, nil
			}
		}
	}

	return "", fmt.Errorf("tyys login response does not contain token: %s", string(data))
}
