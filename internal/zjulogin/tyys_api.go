package zjulogin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
)

const (
	tyysAPIBase = "http://tyys.zju.edu.cn/venue-server"

	tyysPathCodesCodeKeys         = "/api/codes/code_keys"
	tyysPathVenueInfo             = "/api/reservation/campus/venue/info"
	tyysPathDayInfo               = "/api/reservation/day/info"
	tyysPathBuddies               = "/api/buddies"
	tyysPathSysSet                = "/api/sys_sets/key/"
	tyysPathProtocol              = "/api/protocols/code/"
	tyysPathOrderInfo             = "/api/reservation/order/info"
	tyysPathOrderSubmit           = "/api/reservation/order/submit"
	tyysPathCaptchaGet            = "/api/captcha/get"
	tyysPathCaptchaCheck          = "/api/captcha/check"
	tyysPathOrdersMine            = "/api/orders/mine"
	tyysPathFinanceOrderDetail    = "/api/venue/finances/order/detail"
	tyysPathFinanceOrderCancel    = "/api/venue/finances/order/cancel"
	tyysPathFinanceOrderPay       = "/api/venue/finances/order/pay"
	tyysContentTypeJSON           = "application/json"
	tyysContentTypeFormURLEncoded = "application/x-www-form-urlencoded"
)

type TYYSAPIResponse struct {
	Code    int             `json:"code"`
	Data    json.RawMessage `json:"data"`
	Message string          `json:"message"`
}

type tyysCaptchaGetData struct {
	RepData json.RawMessage `json:"repData"`
}

type tyysCaptchaRepData struct {
	Token     string `json:"token"`
	SecretKey string `json:"secretKey"`
}

type TYYSCaptchaSolver interface {
	SolveTYYS(ctx context.Context, challenge json.RawMessage) (url.Values, error)
}

func (s *TYYS) GetAPI(ctx context.Context, path string, params url.Values) (*TYYSAPIResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, tyysAPIURL(path, params), nil)
	if err != nil {
		return nil, err
	}
	return s.doAPI(req, nil)
}

func (s *TYYS) PostFormAPI(ctx context.Context, path string, data url.Values) (*TYYSAPIResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tyysAPIURL(path, nil), strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", tyysContentTypeFormURLEncoded)
	return s.doAPI(req, data)
}

func (s *TYYS) PostJSONAPI(ctx context.Context, path string, data any) (*TYYSAPIResponse, error) {
	body, signParams, err := marshalTYYSJSONBody(data)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tyysAPIURL(path, nil), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", tyysContentTypeJSON)
	return s.doAPI(req, signParams)
}

func (s *TYYS) Codes(ctx context.Context, codeKeys string) (*TYYSAPIResponse, error) {
	params := url.Values{}
	params.Set("codeKeys", codeKeys)
	return s.GetAPI(ctx, tyysPathCodesCodeKeys, params)
}

func (s *TYYS) VenueInfo(ctx context.Context, isArt int) (*TYYSAPIResponse, error) {
	params := url.Values{}
	params.Set("isArt", fmt.Sprintf("%d", isArt))
	return s.GetAPI(ctx, tyysPathVenueInfo, params)
}

func (s *TYYS) ReservationDayInfo(ctx context.Context, params url.Values) (*TYYSAPIResponse, error) {
	return s.GetAPI(ctx, tyysPathDayInfo, params)
}

func (s *TYYS) Buddies(ctx context.Context, params url.Values) (*TYYSAPIResponse, error) {
	return s.GetAPI(ctx, tyysPathBuddies, params)
}

func (s *TYYS) SysSet(ctx context.Context, key string) (*TYYSAPIResponse, error) {
	return s.GetAPI(ctx, tyysPathSysSet+url.PathEscape(key), nil)
}

func (s *TYYS) Protocol(ctx context.Context, code string) (*TYYSAPIResponse, error) {
	return s.GetAPI(ctx, tyysPathProtocol+url.PathEscape(code), nil)
}

func (s *TYYS) ReservationOrderInfo(ctx context.Context, data url.Values) (*TYYSAPIResponse, error) {
	return s.PostFormAPI(ctx, tyysPathOrderInfo, data)
}

func (s *TYYS) ReservationOrderSubmit(ctx context.Context, data url.Values) (*TYYSAPIResponse, error) {
	return s.PostFormAPI(ctx, tyysPathOrderSubmit, data)
}

func (s *TYYS) CaptchaGet(ctx context.Context, params url.Values) (*TYYSAPIResponse, error) {
	return s.GetAPI(ctx, tyysPathCaptchaGet, params)
}

func (s *TYYS) CaptchaCheck(ctx context.Context, data url.Values) (*TYYSAPIResponse, error) {
	return s.PostFormAPI(ctx, tyysPathCaptchaCheck, data)
}

func (s *TYYS) CaptchaGetAndCheck(ctx context.Context, params url.Values, solver TYYSCaptchaSolver) (*TYYSAPIResponse, error) {
	check, _, err := s.CaptchaGetSolveAndCheck(ctx, params, solver)
	return check, err
}

func (s *TYYS) CaptchaGetSolveAndCheck(ctx context.Context, params url.Values, solver TYYSCaptchaSolver) (*TYYSAPIResponse, url.Values, error) {
	if solver == nil {
		return nil, nil, fmt.Errorf("captcha solver is required")
	}
	challenge, err := s.CaptchaGet(ctx, params)
	if err != nil {
		return nil, nil, err
	}
	if rep, ok := parseTYYSCaptchaRepData(challenge.Data); ok {
		fmt.Printf("tyys captcha get: token_len=%d secret_key_len=%d\n", len(rep.Token), len(rep.SecretKey))
	}
	answer, err := solver.SolveTYYS(ctx, challenge.Data)
	if err != nil {
		return nil, nil, err
	}
	checkValues := tyysCaptchaCheckValues(answer)
	fmt.Printf("tyys captcha check: captchaType=%s chaType=%s token_len=%d pointJson_len=%d\n", checkValues.Get("captchaType"), checkValues.Get("chaType"), len(checkValues.Get("token")), len(checkValues.Get("pointJson")))
	check, err := s.CaptchaCheck(ctx, checkValues)
	return check, answer, err
}

func tyysCaptchaCheckValues(values url.Values) url.Values {
	checkValues := url.Values{}
	captchaType := firstNonEmptyReservation(values.Get("captchaType"), values.Get("chaType"), "clickWord")
	setTYYSCaptchaType(checkValues, captchaType)
	for _, key := range []string{"token", "pointJson"} {
		if value := values.Get(key); value != "" {
			checkValues.Set(key, value)
		}
	}
	return checkValues
}

func setTYYSCaptchaType(values url.Values, captchaType string) {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("TYYS_CAPTCHA_CHECK_TYPE_FIELD"))) {
	case "chatype":
		values.Set("chaType", captchaType)
	case "both":
		values.Set("captchaType", captchaType)
		values.Set("chaType", captchaType)
	default:
		values.Set("captchaType", captchaType)
	}
}

func parseTYYSCaptchaRepData(data json.RawMessage) (tyysCaptchaRepData, bool) {
	var wrapper tyysCaptchaGetData
	if err := json.Unmarshal(data, &wrapper); err == nil && len(wrapper.RepData) > 0 {
		var rep tyysCaptchaRepData
		if err := json.Unmarshal(wrapper.RepData, &rep); err == nil {
			return rep, true
		}
	}
	var rep tyysCaptchaRepData
	if err := json.Unmarshal(data, &rep); err == nil && (rep.Token != "" || rep.SecretKey != "") {
		return rep, true
	}
	return tyysCaptchaRepData{}, false
}

func (s *TYYS) OrdersMine(ctx context.Context, params url.Values) (*TYYSAPIResponse, error) {
	return s.GetAPI(ctx, tyysPathOrdersMine, params)
}

func (s *TYYS) FinanceOrderDetail(ctx context.Context, params url.Values) (*TYYSAPIResponse, error) {
	return s.GetAPI(ctx, tyysPathFinanceOrderDetail, params)
}

func (s *TYYS) FinanceOrderCancel(ctx context.Context, data url.Values) (*TYYSAPIResponse, error) {
	return s.PostFormAPI(ctx, tyysPathFinanceOrderCancel, data)
}

func (s *TYYS) FinanceOrderPay(ctx context.Context, data url.Values) (*TYYSAPIResponse, error) {
	return s.PostFormAPI(ctx, tyysPathFinanceOrderPay, data)
}

func (s *TYYS) doAPI(req *http.Request, signParams url.Values) (*TYYSAPIResponse, error) {
	if err := s.Login(req.Context()); err != nil {
		return nil, err
	}
	s.signRequestWithParams(req, signParams)

	res, err := s.core.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer closeResponseBody(res)

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	if res.StatusCode < http.StatusOK || res.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("tyys api %s failed with http status %d: %s", req.URL.Path, res.StatusCode, string(body))
	}

	var payload TYYSAPIResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("decode tyys api %s response: %w", req.URL.Path, err)
	}
	if payload.Code != http.StatusOK {
		return &payload, fmt.Errorf("tyys api %s failed with code %d: %s", req.URL.Path, payload.Code, payload.Message)
	}
	return &payload, nil
}

func tyysAPIURL(path string, params url.Values) string {
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	rawURL := tyysAPIBase + path
	if len(params) > 0 {
		rawURL += "?" + params.Encode()
	}
	return rawURL
}

func marshalTYYSJSONBody(data any) ([]byte, url.Values, error) {
	body, err := json.Marshal(data)
	if err != nil {
		return nil, nil, err
	}
	params := url.Values{}
	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		return body, params, nil
	}
	for key, value := range raw {
		switch item := value.(type) {
		case string:
			params.Set(key, item)
		case float64:
			params.Set(key, fmt.Sprintf("%v", item))
		case bool:
			params.Set(key, fmt.Sprintf("%t", item))
		case nil:
		default:
		}
	}
	return body, params, nil
}
