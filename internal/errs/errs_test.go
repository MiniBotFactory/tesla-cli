package errs

import (
	"errors"
	"strings"
	"testing"
)

func TestNew_setsCodeAndMessage(t *testing.T) {
	e := New(ExitAuth, "no token")

	if e.Code != ExitAuth {
		t.Fatalf("code: want %d, got %d", ExitAuth, e.Code)
	}
	if e.Message != "no token" {
		t.Fatalf("message: want %q, got %q", "no token", e.Message)
	}
	if e.Cause != nil {
		t.Fatalf("cause should be nil, got %v", e.Cause)
	}
}

func TestWrap_chainsCause(t *testing.T) {
	cause := errors.New("io: closed")
	e := Wrap(ExitTimeout, "load", cause)

	if e.Code != ExitTimeout {
		t.Fatalf("code mismatch")
	}
	if !errors.Is(e, cause) {
		t.Fatalf("errors.Is should match wrapped cause")
	}
	if !strings.Contains(e.Error(), "load") || !strings.Contains(e.Error(), "io: closed") {
		t.Fatalf("error message should include both prefix and cause: %q", e.Error())
	}
}

func TestError_nilSafe(t *testing.T) {
	var e *Error
	if got := e.Error(); got != "" {
		t.Fatalf("nil Error() should be empty, got %q", got)
	}
	if got := e.Unwrap(); got != nil {
		t.Fatalf("nil Unwrap() should be nil")
	}
	if got := e.WithHint("x"); got != nil {
		t.Fatalf("nil WithHint() should be nil")
	}
	if got := e.WithRetryable(true); got != nil {
		t.Fatalf("nil WithRetryable() should be nil")
	}
}

func TestWithHint_immutable(t *testing.T) {
	orig := New(ExitConfig, "boom")

	derived := orig.WithHint("try X")

	if orig.Hint != "" {
		t.Fatalf("original Hint should remain empty (immutability), got %q", orig.Hint)
	}
	if derived.Hint != "try X" {
		t.Fatalf("derived Hint mismatch: %q", derived.Hint)
	}
	if orig == derived {
		t.Fatalf("WithHint must return a new pointer")
	}
}

func TestWithRetryable_immutable(t *testing.T) {
	orig := New(ExitRateLimit, "x")
	derived := orig.WithRetryable(true)
	if orig.Retryable {
		t.Fatalf("original Retryable should remain false")
	}
	if !derived.Retryable {
		t.Fatalf("derived Retryable should be true")
	}
}

func TestCodeOf_table(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want ExitCode
	}{
		{"nil → OK", nil, ExitOK},
		{"plain error → Generic", errors.New("plain"), ExitGeneric},
		{"*Error → its code", New(ExitScope, "x"), ExitScope},
		{"wrapped", Wrap(ExitVehicleState, "wrap", errors.New("x")), ExitVehicleState},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if got := CodeOf(tc.err); got != tc.want {
				t.Fatalf("want %d, got %d", tc.want, got)
			}
		})
	}
}

func TestExitCodes_distinct(t *testing.T) {
	codes := map[ExitCode]string{
		ExitOK: "OK", ExitGeneric: "Generic", ExitUsage: "Usage",
		ExitConfig: "Config", ExitAuth: "Auth", ExitScope: "Scope",
		ExitVirtualKey: "VirtualKey", ExitVehicleState: "VehicleState",
		ExitUpstream5xx: "Upstream5xx", ExitTimeout: "Timeout",
		ExitRateLimit: "RateLimit",
	}
	if len(codes) != 11 {
		t.Fatalf("expected 11 distinct exit codes, got %d (collision?)", len(codes))
	}
}
