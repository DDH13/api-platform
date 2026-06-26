package analytics

import (
	"encoding/json"
	"testing"

	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

func TestGetHeaderFlags(t *testing.T) {
	cases := []struct {
		name     string
		params   map[string]interface{}
		wantReq  bool
		wantResp bool
	}{
		{"nil params", nil, false, false},
		{"absent", map[string]interface{}{}, false, false},
		{"bool true", map[string]interface{}{"send_request_headers": true, "send_response_headers": true}, true, true},
		{"string true", map[string]interface{}{"send_request_headers": "true"}, true, false},
		{"mixed", map[string]interface{}{"send_request_headers": false, "send_response_headers": "yes"}, false, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			gotReq, gotResp := getHeaderFlags(c.params)
			if gotReq != c.wantReq || gotResp != c.wantResp {
				t.Fatalf("getHeaderFlags(%v) = (%v, %v), want (%v, %v)", c.params, gotReq, gotResp, c.wantReq, c.wantResp)
			}
		})
	}
}

func TestSerializeHeaders(t *testing.T) {
	// Empty headers -> empty string.
	if got := serializeHeaders(policy.NewHeaders(nil)); got != "" {
		t.Fatalf("serializeHeaders(empty) = %q, want \"\"", got)
	}

	h := policy.NewHeaders(map[string][]string{
		"Authorization": {"Bearer secret"},
		"X-Foo":         {"a", "b"},
	})
	got := serializeHeaders(h)
	if got == "" {
		t.Fatal("serializeHeaders returned empty for non-empty headers")
	}

	var decoded map[string]string
	if err := json.Unmarshal([]byte(got), &decoded); err != nil {
		t.Fatalf("output is not valid JSON: %v (%q)", err, got)
	}
	// NewHeaders lower-cases keys; multi-value headers are joined with ", ".
	if decoded["authorization"] != "Bearer secret" {
		t.Errorf("authorization = %q, want %q", decoded["authorization"], "Bearer secret")
	}
	if decoded["x-foo"] != "a, b" {
		t.Errorf("x-foo = %q, want %q", decoded["x-foo"], "a, b")
	}
}

func TestGetMaxPayloadSize(t *testing.T) {
	cases := []struct {
		name   string
		params map[string]interface{}
		want   int
	}{
		{"nil", nil, 0},
		{"absent", map[string]interface{}{}, 0},
		{"int", map[string]interface{}{"max_payload_size": 2048}, 2048},
		{"float64", map[string]interface{}{"max_payload_size": float64(1024)}, 1024},
		{"string", map[string]interface{}{"max_payload_size": "512"}, 512},
		{"bad string", map[string]interface{}{"max_payload_size": "abc"}, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := getMaxPayloadSize(c.params); got != c.want {
				t.Fatalf("getMaxPayloadSize(%v) = %d, want %d", c.params, got, c.want)
			}
		})
	}
}

func TestTruncatePayload(t *testing.T) {
	body := []byte("hello world")
	if got := truncatePayload(body, 0); got != "hello world" {
		t.Errorf("max=0 (no limit) = %q, want full body", got)
	}
	if got := truncatePayload(body, 100); got != "hello world" {
		t.Errorf("max>len = %q, want full body", got)
	}
	if got := truncatePayload(body, 5); got != "hello" {
		t.Errorf("max=5 = %q, want %q", got, "hello")
	}
	if got := truncatePayload([]byte{}, 5); got != "" {
		t.Errorf("empty body = %q, want \"\"", got)
	}
}
