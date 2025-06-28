package yall

import (
	"log/slog"
	"testing"
)

func TestYallLoggerGetSlogLevel(t *testing.T) {
	cases := map[string]slog.Level{
		"trace":    levelTrace,
		"debug":    slog.LevelDebug,
		"info":     slog.LevelInfo,
		"warn":     slog.LevelWarn,
		"error":    slog.LevelError,
		"critical": levelFatal,
		"fatal":    levelCritical,
	}

	for name, want := range cases {
		lvl, err := getSlogLevel(name)
		if err != nil {
			t.Errorf("expected no error for level %q, got %v", name, err)
			continue
		}
		if lvl != want {
			t.Errorf("level %q: expected %v, got %v", name, want, lvl)
		}
	}

	invalidLevel := "non_existent_level"
	lvl, err := getSlogLevel(invalidLevel)
	if err == nil || err.Error() != "unknown level" {
		t.Errorf("expected error for level %q, got %v", invalidLevel, lvl)
	}
}
