package meta

import "testing"

func TestSnapshot_defaultValues(t *testing.T) {
	got := Snapshot()
	if got.Version == "" {
		t.Fatalf("Version should be non-empty (default %q)", defaultVersion)
	}
	if got.Commit == "" {
		t.Fatalf("Commit should be non-empty (default %q)", defaultCommit)
	}
	if got.BuildDate == "" {
		t.Fatalf("BuildDate should be non-empty (default %q)", defaultBuildDate)
	}
}

func TestSnapshot_reflectsLDFlagsInjection(t *testing.T) {
	// 模拟 ldflags 注入。
	origV, origC, origD := Version, Commit, BuildDate
	t.Cleanup(func() { Version, Commit, BuildDate = origV, origC, origD })

	Version = "1.2.3"
	Commit = "abc1234"
	BuildDate = "2026-04-26T00:00:00Z"

	got := Snapshot()
	if got.Version != "1.2.3" || got.Commit != "abc1234" || got.BuildDate != "2026-04-26T00:00:00Z" {
		t.Fatalf("snapshot did not reflect mutated package vars: %+v", got)
	}
}

func TestSnapshot_stableAcrossCalls(t *testing.T) {
	a := Snapshot()
	b := Snapshot()
	if a != b {
		t.Fatalf("expected stable snapshot across calls; got %+v vs %+v", a, b)
	}
}
