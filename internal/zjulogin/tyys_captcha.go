package zjulogin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const tyysDefaultCaptchaScript = "scripts/tyys_captcha_solver.py"

type TYYSPythonCaptchaSolver struct {
	PythonPath string
	ScriptPath string
	ExtraArgs  []string
}

type tyysPythonCaptchaResult struct {
	CaptchaType         string             `json:"captchaType"`
	Token               string             `json:"token"`
	PointJSON           string             `json:"pointJson"`
	CaptchaVerification string             `json:"captchaVerification"`
	Values              map[string]string  `json:"values"`
	ClickPoints         []map[string]int32 `json:"click_points"`
}

func (s TYYSPythonCaptchaSolver) SolveTYYS(ctx context.Context, challenge json.RawMessage) (url.Values, error) {
	pythonPath := strings.TrimSpace(s.PythonPath)
	if pythonPath == "" {
		pythonPath = "python"
	}
	scriptPath := strings.TrimSpace(s.ScriptPath)
	if scriptPath == "" {
		scriptPath = findEnvFile(tyysDefaultCaptchaScript)
	} else if !filepath.IsAbs(scriptPath) {
		scriptPath = findEnvFile(scriptPath)
	}

	args := append([]string{scriptPath}, s.ExtraArgs...)
	cmd := exec.CommandContext(ctx, pythonPath, args...)
	cmd.Stdin = bytes.NewReader(challenge)
	cmd.Env = append(os.Environ(), "PYTHONIOENCODING=utf-8")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("run tyys captcha solver: %w: %s", err, strings.TrimSpace(stderr.String()))
	}

	var result tyysPythonCaptchaResult
	if err := json.Unmarshal(bytes.TrimSpace(stdout.Bytes()), &result); err != nil {
		return nil, fmt.Errorf("decode tyys captcha solver output: %w: %s", err, stdout.String())
	}
	if os.Getenv("TYYS_CAPTCHA_DEBUG") == "1" {
		fmt.Printf("tyys captcha solver: click_points=%v stderr=%s\n", result.ClickPoints, strings.TrimSpace(stderr.String()))
	}

	values := result.urlValues()
	if len(values) == 0 {
		return nil, fmt.Errorf("tyys captcha solver returned no check values")
	}
	return values, nil
}

func (r tyysPythonCaptchaResult) urlValues() url.Values {
	values := url.Values{}
	if r.CaptchaType != "" {
		values.Set("captchaType", r.CaptchaType)
	}
	if r.Token != "" {
		values.Set("token", r.Token)
	}
	if r.PointJSON != "" {
		values.Set("pointJson", r.PointJSON)
	}
	if r.CaptchaVerification != "" {
		values.Set("captchaVerification", r.CaptchaVerification)
	}
	for key, value := range r.Values {
		if value != "" {
			values.Set(key, value)
		}
	}
	return values
}
