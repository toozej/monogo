package cmd

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

func TestIsCancellationOnly(t *testing.T) {
	otherErr := errors.New("database close failed")
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "canceled", err: context.Canceled, want: true},
		{name: "wrapped cancellation", err: fmt.Errorf("stop run: %w", context.Canceled), want: true},
		{name: "joined cancellations", err: errors.Join(context.Canceled, fmt.Errorf("fetch: %w", context.Canceled)), want: true},
		{name: "other error", err: otherErr, want: false},
		{name: "cancellation joined with failure", err: errors.Join(context.Canceled, otherErr), want: false},
		{name: "wrapped cancellation joined with failure", err: fmt.Errorf("run: %w", errors.Join(context.Canceled, otherErr)), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isCancellationOnly(tt.err); got != tt.want {
				t.Fatalf("isCancellationOnly(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestRootCommandSilencesCobraErrorPrinting(t *testing.T) {
	if !rootCmd.SilenceErrors {
		t.Fatal("root command must let Execute print returned errors exactly once")
	}
}
