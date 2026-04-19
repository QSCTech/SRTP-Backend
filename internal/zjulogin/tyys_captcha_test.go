package zjulogin

import (
	"encoding/json"
	"net/url"
	"reflect"
	"testing"
)

func TestTYYSPythonCaptchaResultValues(t *testing.T) {
	raw := []byte(`{"captchaType":"clickWord","token":"abc","pointJson":"encrypted-point","captchaVerification":"encrypted-verification","values":{"captchaVerification":"ok"}}`)

	var result tyysPythonCaptchaResult
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	want := url.Values{
		"captchaType":         {"clickWord"},
		"token":               {"abc"},
		"pointJson":           {"encrypted-point"},
		"captchaVerification": {"ok"},
	}
	if values := result.urlValues(); !reflect.DeepEqual(values, want) {
		t.Fatalf("values=%v want=%v", values, want)
	}

	checkWant := url.Values{
		"captchaType": {"clickWord"},
		"token":       {"abc"},
		"pointJson":   {"encrypted-point"},
	}
	if values := tyysCaptchaCheckValues(result.urlValues()); !reflect.DeepEqual(values, checkWant) {
		t.Fatalf("check values=%v want=%v", values, checkWant)
	}
}
