package collector

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestCleanupBadFiles_DeletesStaleSiblings verifies the basic happy-path:
// siblings whose mtime differs from the base file get removed, and matching
// the base file's mtime is preserved.
func TestCleanupBadFiles_DeletesStaleSiblings(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, ".rtreport")
	stale := filepath.Join(dir, ".rtreport_stale")
	keep := filepath.Join(dir, ".rtreport_keep")

	mustWrite(t, base, "base")
	mustWrite(t, stale, "stale")
	mustWrite(t, keep, "keep")

	// Force differing mtimes.
	now := time.Now()
	if err := os.Chtimes(base, now, now); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(keep, now, now); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(stale, now.Add(-time.Hour), now.Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}

	cleanupBadFiles(base, base+"*")

	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Fatalf("expected stale file to be removed, got err=%v", err)
	}
	if _, err := os.Stat(keep); err != nil {
		t.Fatalf("expected keep file to survive: %v", err)
	}
	if _, err := os.Stat(base); err != nil {
		t.Fatalf("expected base file to survive: %v", err)
	}
}

// TestCleanupBadFiles_FollowsSymlinkBase verifies the v0.2.1 behaviour:
// LiteSpeed publishes .rtreport as a symlink, so the cleanup pass must
// follow it (compare mtimes against the symlink target) rather than
// abort. Stale siblings should still be deleted.
func TestCleanupBadFiles_FollowsSymlinkBase(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, ".rtreport_real")
	link := filepath.Join(dir, ".rtreport")
	mustWrite(t, target, "real")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks unsupported: %v", err)
	}
	stale := filepath.Join(dir, ".rtreport_stale")
	mustWrite(t, stale, "stale")

	now := time.Now()
	if err := os.Chtimes(target, now, now); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(stale, now.Add(-time.Hour), now.Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}

	cleanupBadFiles(link, link+"*")

	// The stale sibling should be gone (its mtime didn't match the
	// symlink target's).
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Fatalf("expected stale file to be removed when base is a symlink, got err=%v", err)
	}
	// The symlink itself and its target must survive — we never unlink
	// through symlinks, and the target's mtime matches itself.
	if _, err := os.Lstat(link); err != nil {
		t.Fatalf("symlink should still exist: %v", err)
	}
	if _, err := os.Stat(target); err != nil {
		t.Fatalf("symlink target should still exist: %v", err)
	}
}

// TestCleanupBadFiles_MissingBaseIsSilent verifies that a missing
// .rtreport (e.g. LSWS hasn't started yet) is handled silently and does
// not touch any sibling files.
func TestCleanupBadFiles_MissingBaseIsSilent(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, ".rtreport")
	sibling := filepath.Join(dir, ".rtreport_stale")
	mustWrite(t, sibling, "sibling")

	// Should not panic, should not delete anything.
	cleanupBadFiles(base, base+"*")

	if _, err := os.Stat(sibling); err != nil {
		t.Fatalf("sibling must survive when base is missing: %v", err)
	}
}

// TestCleanupBadFiles_SkipsSymlinkMatch ensures symlinks that match the glob
// are not unlinked through. Without the Lstat / IsRegular guard, an attacker
// could place a symlink in the rtreport dir pointing somewhere else.
func TestCleanupBadFiles_SkipsSymlinkMatch(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, ".rtreport")
	mustWrite(t, base, "base")
	now := time.Now()
	_ = os.Chtimes(base, now, now)

	// Real file outside the glob, plus a symlink that matches the glob.
	outside := filepath.Join(dir, "outside")
	mustWrite(t, outside, "outside")
	link := filepath.Join(dir, ".rtreport_link")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlinks unsupported: %v", err)
	}
	// Make the symlink target's mtime differ from base so the old code would
	// have unlinked it.
	_ = os.Chtimes(outside, now.Add(-time.Hour), now.Add(-time.Hour))

	cleanupBadFiles(base, base+"*")

	if _, err := os.Lstat(link); err != nil {
		t.Fatalf("symlink match should not have been removed: %v", err)
	}
	if _, err := os.Stat(outside); err != nil {
		t.Fatalf("symlink target must remain untouched: %v", err)
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile %s: %v", path, err)
	}
}
