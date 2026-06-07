package health

import (
	"context"
	"testing"
)

func TestLiveAlwaysTrue(t *testing.T) {
	c := New(nil)
	if !c.Live() {
		t.Fatalf("Live should be true")
	}
}

func TestReadyDefaultTrue(t *testing.T) {
	c := New(nil)
	if !c.Ready(context.Background()) {
		t.Fatalf("default Ready should be true")
	}
}

func TestReadyProbe(t *testing.T) {
	tests := []struct {
		name  string
		probe ReadyFunc
		want  bool
	}{
		{"ready", func(context.Context) bool { return true }, true},
		{"not ready", func(context.Context) bool { return false }, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := New(tc.probe)
			if got := c.Ready(context.Background()); got != tc.want {
				t.Fatalf("Ready want %v, got %v", tc.want, got)
			}
		})
	}
}
