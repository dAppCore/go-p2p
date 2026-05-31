package node

import (
	"archive/tar"
	"testing"

	core "dappco.re/go"
)

// TestExtractTarball_CorruptArchive rejects bytes that are not a valid tar
// stream, exercising the tr.Next() error path.
func TestExtractTarball_CorruptArchive(t *testing.T) {
	// A non-empty, non-tar byte stream: the tar reader fails on the header.
	corrupt := []byte("this is definitely not a tar archive, just some bytes")
	_, err := resultValue[string](extractTarball(corrupt, t.TempDir()))
	if err == nil {
		t.Fatal("expected error for corrupt tar archive")
	}
}

// TestExtractTarball_ExecutableTracking returns the first executable entry's
// path, covering the executable-bit tracking branch.
func TestExtractTarball_ExecutableTracking(t *testing.T) {
	buf := core.NewBuffer()
	tw := tar.NewWriter(buf)

	// A non-executable file first.
	cfg := []byte(`{"k":"v"}`)
	if err := tw.WriteHeader(&tar.Header{Name: "config.json", Mode: 0o600, Size: int64(len(cfg)), Typeflag: tar.TypeReg}); err != nil {
		t.Fatalf("write config header: %v", err)
	}
	if _, err := tw.Write(cfg); err != nil {
		t.Fatalf("write config: %v", err)
	}

	// An executable binary second.
	bin := []byte("\x7fELF-stub")
	if err := tw.WriteHeader(&tar.Header{Name: "miner", Mode: 0o755, Size: int64(len(bin)), Typeflag: tar.TypeReg}); err != nil {
		t.Fatalf("write bin header: %v", err)
	}
	if _, err := tw.Write(bin); err != nil {
		t.Fatalf("write bin: %v", err)
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("close tar writer: %v", err)
	}

	tmpDir := t.TempDir()
	first, err := resultValue[string](extractTarball(buf.Bytes(), tmpDir))
	if err != nil {
		t.Fatalf("extractTarball: %v", err)
	}
	if first != core.PathJoin(tmpDir, "miner") {
		t.Fatalf("first executable: got %q, want %q", first, core.PathJoin(tmpDir, "miner"))
	}

	// Confirm the non-executable file landed too.
	if _, err := testReadFile(core.PathJoin(tmpDir, "config.json")); err != nil {
		t.Fatalf("read extracted config: %v", err)
	}
}

// TestExtractTarball_HardLinkIgnored silently skips hard-link entries the same
// way symlinks are skipped, for symlink-attack resistance.
func TestExtractTarball_HardLinkIgnored(t *testing.T) {
	buf := core.NewBuffer()
	tw := tar.NewWriter(buf)
	if err := tw.WriteHeader(&tar.Header{
		Name:     "hardlink",
		Linkname: "/etc/passwd",
		Typeflag: tar.TypeLink,
	}); err != nil {
		t.Fatalf("write hardlink header: %v", err)
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("close tar writer: %v", err)
	}

	tmpDir := t.TempDir()
	if _, err := resultValue[string](extractTarball(buf.Bytes(), tmpDir)); err != nil {
		t.Fatalf("extractTarball should skip hard links without error: %v", err)
	}
	if _, statErr := testLstat(core.PathJoin(tmpDir, "hardlink")); !core.IsNotExist(statErr) {
		t.Fatal("hard link should not be created")
	}
}
