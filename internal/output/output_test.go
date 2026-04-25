package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/wmango/tesla-cli/internal/errs"
)

func TestParseFormat_table(t *testing.T) {
	cases := []struct {
		in      string
		want    Format
		wantErr bool
	}{
		{"json", FormatJSON, false},
		{"yaml", FormatYAML, false},
		{"table", FormatTable, false},
		{"text", FormatText, false},
		{"xml", "", true},
		{"", "", true},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.in, func(t *testing.T) {
			got, err := ParseFormat(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q", tc.in)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if got != tc.want {
				t.Fatalf("want %q, got %q", tc.want, got)
			}
		})
	}
}

func TestAllFormats_containsAllFour(t *testing.T) {
	got := AllFormats()
	if len(got) != 4 {
		t.Fatalf("expected 4 formats, got %d", len(got))
	}
}

func TestSuccess_envelopeFields(t *testing.T) {
	started := time.Now().Add(-100 * time.Millisecond)
	env := Success(map[string]any{"x": 1}, "req-123", started)

	if !env.Ok {
		t.Errorf("Ok should be true")
	}
	if env.Data == nil {
		t.Errorf("Data should be set")
	}
	if env.RequestID != "req-123" {
		t.Errorf("RequestID mismatch: %q", env.RequestID)
	}
	if env.DurationMS < 0 {
		t.Errorf("DurationMS should be >= 0, got %d", env.DurationMS)
	}
	if env.Code != "" {
		t.Errorf("success Code should be empty, got %q", env.Code)
	}
}

func TestFailure_envelopeFields(t *testing.T) {
	e := errs.New(errs.ExitAuth, "no token").WithHint("login").WithRetryable(true)
	env := Failure(e, "req-9", time.Now())

	if env.Ok {
		t.Errorf("Ok should be false")
	}
	if env.Code != "AUTH" {
		t.Errorf("expected code AUTH, got %q", env.Code)
	}
	if env.Hint != "login" {
		t.Errorf("hint mismatch: %q", env.Hint)
	}
	if !env.Retryable {
		t.Errorf("retryable should be true")
	}
	if !strings.Contains(env.Message, "no token") {
		t.Errorf("message missing core text: %q", env.Message)
	}
}

func TestFailure_nilGracefullyDegrades(t *testing.T) {
	env := Failure(nil, "", time.Now())
	if env.Ok {
		t.Fatalf("Ok should be false")
	}
	if env.Code == "" {
		t.Fatalf("Code should default to GENERIC, got empty")
	}
}

func TestCodeName_allBranches(t *testing.T) {
	cases := []struct {
		code errs.ExitCode
		want string
	}{
		{errs.ExitOK, "OK"},
		{errs.ExitUsage, "USAGE"},
		{errs.ExitConfig, "CONFIG"},
		{errs.ExitAuth, "AUTH"},
		{errs.ExitScope, "SCOPE"},
		{errs.ExitVirtualKey, "VIRTUAL_KEY"},
		{errs.ExitVehicleState, "VEHICLE_STATE"},
		{errs.ExitUpstream5xx, "UPSTREAM_5XX"},
		{errs.ExitTimeout, "TIMEOUT"},
		{errs.ExitRateLimit, "RATE_LIMIT"},
		{errs.ExitGeneric, "GENERIC"},
	}
	for _, tc := range cases {
		if got := codeName(tc.code); got != tc.want {
			t.Errorf("code %d: want %q, got %q", tc.code, tc.want, got)
		}
	}
}

func TestJSONRenderer_validJSON(t *testing.T) {
	var buf bytes.Buffer
	r := NewRenderer(FormatJSON)
	if err := r.Render(&buf, Success("hello", "", time.Now())); err != nil {
		t.Fatalf("render: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, buf.String())
	}
	if got["ok"] != true {
		t.Errorf("ok should be true, got %v", got["ok"])
	}
	if got["data"] != "hello" {
		t.Errorf("data should be hello, got %v", got["data"])
	}
}

func TestYAMLRenderer_omitsEmpty(t *testing.T) {
	var buf bytes.Buffer
	r := NewRenderer(FormatYAML)
	if err := r.Render(&buf, Success("hi", "", time.Now())); err != nil {
		t.Fatalf("render: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "ok: true") {
		t.Errorf("missing ok line: %q", out)
	}
	if strings.Contains(out, "code:") || strings.Contains(out, "hint:") {
		t.Errorf("empty fields should be omitted: %q", out)
	}
}

func TestTextRenderer_okPath(t *testing.T) {
	var buf bytes.Buffer
	r := NewRenderer(FormatText)
	if err := r.Render(&buf, Success("d", "", time.Now())); err != nil {
		t.Fatalf("render: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "ok") {
		t.Errorf("text output should contain 'ok', got %q", out)
	}
}

func TestTextRenderer_failurePath(t *testing.T) {
	var buf bytes.Buffer
	r := NewRenderer(FormatText)
	e := errs.New(errs.ExitConfig, "bad").WithHint("fix it")
	if err := r.Render(&buf, Failure(e, "", time.Now())); err != nil {
		t.Fatalf("render: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "CONFIG") || !strings.Contains(out, "bad") || !strings.Contains(out, "fix it") {
		t.Errorf("text failure output incomplete: %q", out)
	}
}

func TestNewRenderer_unknownFormatFallsBackToJSON(t *testing.T) {
	var buf bytes.Buffer
	r := NewRenderer(Format("nonsense"))
	if err := r.Render(&buf, Success("x", "", time.Now())); err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.HasPrefix(strings.TrimSpace(buf.String()), "{") {
		t.Errorf("unknown format should fall back to JSON; got %q", buf.String())
	}
}
