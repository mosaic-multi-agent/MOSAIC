package harness

import (
	"errors"
	"testing"
	"time"
)

func TestNoOutputErrorMessageAndSentinels(t *testing.T) {
	w := &outputWatch{after: 10 * time.Minute}
	err := w.err()
	want := "harness: no output within the no-output timeout: no stdout output for 10m0s"
	if err.Error() != want {
		t.Fatalf("message = %q, want %q", err.Error(), want)
	}
	if !errors.Is(err, ErrNoOutput) || !errors.Is(err, ErrTimeout) {
		t.Fatalf("error must wrap ErrNoOutput and ErrTimeout")
	}
}
