package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPurgeOldTimelapsesKeepsPreviousTenDays(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 10, 19, 15, 4, 0, 0, time.Local)

	// Unrelated files must be left alone.
	mustTouch(t, filepath.Join(dir, "goes_conus_geocolor.png"))
	mustTouch(t, filepath.Join(dir, "timelapse-notes.mp4"))
	frames := filepath.Join(dir, "frames")
	if err := os.Mkdir(frames, 0755); err != nil {
		t.Fatal(err)
	}
	mustTouch(t, filepath.Join(frames, "20261001-120000.png"))

	// Today plus the previous 9 days stay. The day before that is removed.
	var kept, removed []string
	for daysAgo := 0; daysAgo <= 12; daysAgo++ {
		name := "timelapse-" + now.AddDate(0, 0, -daysAgo).Format(timelapseDateFormat) + ".mp4"
		mustTouch(t, filepath.Join(dir, name))
		if daysAgo < timelapseRetentionDays {
			kept = append(kept, name)
		} else {
			removed = append(removed, name)
		}
	}

	if err := purgeOldTimelapses(dir, now, false); err != nil {
		t.Fatal(err)
	}

	for _, name := range kept {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("expected to keep %s: %v", name, err)
		}
	}
	for _, name := range removed {
		if _, err := os.Stat(filepath.Join(dir, name)); !os.IsNotExist(err) {
			t.Errorf("expected to remove %s, stat err=%v", name, err)
		}
	}

	for _, name := range []string{"goes_conus_geocolor.png", "timelapse-notes.mp4", filepath.Join("frames", "20261001-120000.png")} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("expected to leave %s alone: %v", name, err)
		}
	}
}

func mustTouch(t *testing.T, path string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}
