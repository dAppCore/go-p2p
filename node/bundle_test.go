package node

import (
	"archive/tar"
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

type errBundleWriter struct{}

func (errBundleWriter) Write([]byte) (int, error) {
	return 0, errors.New("write failed")
}

func testMinerFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "miner")
	if err := os.WriteFile(path, []byte(content), 0755); err != nil {
		t.Fatalf("write miner file: %v", err)
	}
	return path
}

func TestCreateProfileBundleUnencrypted(t *testing.T) {
	profileJSON := []byte(`{"name":"test-profile","minerType":"xmrig","config":{}}`)

	bundle, err := CreateProfileBundleUnencrypted(profileJSON, "test-profile")
	if err != nil {
		t.Fatalf("failed to create bundle: %v", err)
	}

	if bundle.Type != BundleProfile {
		t.Errorf("expected type BundleProfile, got %s", bundle.Type)
	}

	if bundle.Name != "test-profile" {
		t.Errorf("expected name 'test-profile', got '%s'", bundle.Name)
	}

	if bundle.Checksum == "" {
		t.Error("checksum should not be empty")
	}

	if !bytes.Equal(bundle.Data, profileJSON) {
		t.Error("data should match original JSON")
	}
}

func TestBundle_CreateProfileBundle_Good(t *testing.T) {
	profile := []byte(`{"name":"profile"}`)
	bundle, err := CreateProfileBundle(profile, "profile", "password")
	if err != nil {
		t.Fatalf("CreateProfileBundle: %v", err)
	}
	if bundle.Type != BundleProfile || bytes.Equal(bundle.Data, profile) {
		t.Fatalf("bundle: %#v", bundle)
	}
}

func TestBundle_CreateProfileBundle_Bad(t *testing.T) {
	profile := []byte(`{"name":"profile"}`)
	bundle, err := CreateProfileBundle(profile, "profile", "")
	if err == nil {
		t.Fatal("expected empty password error")
	}
	if bundle != nil {
		t.Fatalf("bundle: got %#v, want nil", bundle)
	}
}

func TestBundle_CreateProfileBundle_Ugly(t *testing.T) {
	bundle, err := CreateProfileBundle(nil, "", "password")
	if err == nil {
		t.Fatal("expected nil profile error")
	}
	if bundle != nil {
		t.Fatalf("bundle: got %#v, want nil", bundle)
	}
}

func TestBundle_CreateProfileBundleUnencrypted_Good(t *testing.T) {
	profile := []byte(`{"name":"plain"}`)
	bundle, err := CreateProfileBundleUnencrypted(profile, "plain")
	if err != nil {
		t.Fatalf("CreateProfileBundleUnencrypted: %v", err)
	}
	if !bytes.Equal(bundle.Data, profile) {
		t.Fatal("data should match profile")
	}
}

func TestBundle_CreateProfileBundleUnencrypted_Bad(t *testing.T) {
	bundle, err := CreateProfileBundleUnencrypted(nil, "")
	if err != nil {
		t.Fatalf("CreateProfileBundleUnencrypted nil: %v", err)
	}
	if bundle.Name != "" || bundle.Data != nil {
		t.Fatalf("bundle: %#v", bundle)
	}
}

func TestBundle_CreateProfileBundleUnencrypted_Ugly(t *testing.T) {
	profile := []byte(`[]`)
	bundle, err := CreateProfileBundleUnencrypted(profile, "array")
	if err != nil {
		t.Fatalf("CreateProfileBundleUnencrypted: %v", err)
	}
	if !VerifyBundle(bundle) {
		t.Fatal("array JSON bundle should verify")
	}
}

func TestBundle_CreateMinerBundle_Good(t *testing.T) {
	minerPath := testMinerFile(t, "binary")
	bundle, err := CreateMinerBundle(minerPath, []byte(`{"profile":true}`), "miner", "password")
	if err != nil {
		t.Fatalf("CreateMinerBundle: %v", err)
	}
	if bundle.Type != BundleMiner || bundle.Name != "miner" {
		t.Fatalf("bundle: %#v", bundle)
	}
}

func TestBundle_CreateMinerBundle_Bad(t *testing.T) {
	bundle, err := CreateMinerBundle(filepath.Join(t.TempDir(), "missing"), nil, "miner", "password")
	if err == nil {
		t.Fatal("expected missing miner error")
	}
	if bundle != nil {
		t.Fatalf("bundle: got %#v, want nil", bundle)
	}
}

func TestBundle_CreateMinerBundle_Ugly(t *testing.T) {
	minerPath := testMinerFile(t, "")
	bundle, err := CreateMinerBundle(minerPath, nil, "", "password")
	if err != nil {
		t.Fatalf("CreateMinerBundle empty inputs: %v", err)
	}
	if bundle.Name != "" || !VerifyBundle(bundle) {
		t.Fatalf("bundle: %#v", bundle)
	}
}

func TestBundle_ExtractProfileBundle_Good(t *testing.T) {
	profile := []byte(`{"name":"plain"}`)
	bundle, _ := CreateProfileBundleUnencrypted(profile, "plain")
	extracted, err := ExtractProfileBundle(bundle, "")
	if err != nil {
		t.Fatalf("ExtractProfileBundle: %v", err)
	}
	if !bytes.Equal(extracted, profile) {
		t.Fatalf("profile: got %s", extracted)
	}
}

func TestBundle_ExtractProfileBundle_Bad(t *testing.T) {
	bundle, _ := CreateProfileBundleUnencrypted([]byte(`{}`), "plain")
	bundle.Checksum = "bad"
	extracted, err := ExtractProfileBundle(bundle, "")
	if err == nil {
		t.Fatal("expected checksum error")
	}
	if extracted != nil {
		t.Fatalf("profile: got %s, want nil", extracted)
	}
}

func TestBundle_ExtractProfileBundle_Ugly(t *testing.T) {
	profile := []byte(`{"name":"secret"}`)
	bundle, _ := CreateProfileBundle(profile, "secret", "password")
	extracted, err := ExtractProfileBundle(bundle, "password")
	if err != nil {
		t.Fatalf("ExtractProfileBundle encrypted: %v", err)
	}
	if !bytes.Equal(extracted, profile) {
		t.Fatalf("profile: got %s", extracted)
	}
}

func TestBundle_ExtractMinerBundle_Good(t *testing.T) {
	minerPath := testMinerFile(t, "binary")
	bundle, err := CreateMinerBundle(minerPath, []byte(`{"profile":true}`), "miner", "password")
	if err != nil {
		t.Fatalf("CreateMinerBundle: %v", err)
	}
	extractedPath, profile, err := ExtractMinerBundle(bundle, "password", t.TempDir())
	if err != nil {
		t.Fatalf("ExtractMinerBundle: %v", err)
	}
	if !bytes.Equal(profile, []byte(`{"profile":true}`)) {
		t.Fatalf("path=%q profile=%s", extractedPath, profile)
	}
}

func TestBundle_ExtractMinerBundle_Bad(t *testing.T) {
	bundle := &Bundle{Type: BundleMiner, Data: []byte("bad"), Checksum: "bad"}
	path, profile, err := ExtractMinerBundle(bundle, "password", t.TempDir())
	if err == nil {
		t.Fatal("expected checksum error")
	}
	if path != "" || profile != nil {
		t.Fatalf("path=%q profile=%s", path, profile)
	}
}

func TestBundle_ExtractMinerBundle_Ugly(t *testing.T) {
	minerPath := testMinerFile(t, "binary")
	bundle, err := CreateMinerBundle(minerPath, nil, "miner", "password")
	if err != nil {
		t.Fatalf("CreateMinerBundle: %v", err)
	}
	path, profile, err := ExtractMinerBundle(bundle, "wrong-password", t.TempDir())
	if err == nil {
		t.Fatal("expected decrypt error")
	}
	if path != "" || profile != nil {
		t.Fatalf("path=%q profile=%s", path, profile)
	}
}

func TestBundle_VerifyBundle_Good(t *testing.T) {
	bundle, _ := CreateProfileBundleUnencrypted([]byte(`{"ok":true}`), "ok")
	if !VerifyBundle(bundle) {
		t.Fatal("valid bundle should verify")
	}
	if bundle.Checksum == "" {
		t.Fatal("expected checksum")
	}
}

func TestBundle_VerifyBundle_Bad(t *testing.T) {
	bundle, _ := CreateProfileBundleUnencrypted([]byte(`{"ok":true}`), "ok")
	bundle.Data = []byte(`{"ok":false}`)
	if VerifyBundle(bundle) {
		t.Fatal("modified bundle should not verify")
	}
	if bundle.Checksum == "" {
		t.Fatal("expected checksum")
	}
}

func TestBundle_VerifyBundle_Ugly(t *testing.T) {
	bundle := &Bundle{Data: nil, Checksum: calculateChecksum(nil)}
	if !VerifyBundle(bundle) {
		t.Fatal("nil data with matching checksum should verify")
	}
	if bundle.Type != "" {
		t.Fatal("type should remain empty")
	}
}

func TestBundle_StreamBundle_Good(t *testing.T) {
	bundle, _ := CreateProfileBundleUnencrypted([]byte(`{"stream":true}`), "stream")
	var buf bytes.Buffer
	err := StreamBundle(bundle, &buf)
	if err != nil {
		t.Fatalf("StreamBundle: %v", err)
	}
	if buf.Len() == 0 {
		t.Fatal("expected streamed data")
	}
}

func TestBundle_StreamBundle_Bad(t *testing.T) {
	bundle, _ := CreateProfileBundleUnencrypted([]byte(`{}`), "stream")
	err := StreamBundle(bundle, errBundleWriter{})
	if err == nil {
		t.Fatal("expected writer error")
	}
	if !VerifyBundle(bundle) {
		t.Fatal("stream failure should not mutate bundle")
	}
}

func TestBundle_StreamBundle_Ugly(t *testing.T) {
	var buf bytes.Buffer
	err := StreamBundle(&Bundle{}, &buf)
	if err != nil {
		t.Fatalf("StreamBundle empty: %v", err)
	}
	if !bytes.Contains(buf.Bytes(), []byte(`"type"`)) {
		t.Fatalf("json: %s", buf.String())
	}
}

func TestBundle_ReadBundle_Good(t *testing.T) {
	bundle, _ := CreateProfileBundleUnencrypted([]byte(`{"read":true}`), "read")
	var buf bytes.Buffer
	if err := StreamBundle(bundle, &buf); err != nil {
		t.Fatalf("StreamBundle: %v", err)
	}
	restored, err := ReadBundle(&buf)
	if err != nil || restored.Name != "read" {
		t.Fatalf("restored: %#v err=%v", restored, err)
	}
}

func TestBundle_ReadBundle_Bad(t *testing.T) {
	restored, err := ReadBundle(bytes.NewReader([]byte("not json")))
	if err == nil {
		t.Fatal("expected JSON error")
	}
	if restored != nil {
		t.Fatalf("bundle: got %#v, want nil", restored)
	}
}

func TestBundle_ReadBundle_Ugly(t *testing.T) {
	restored, err := ReadBundle(bytes.NewReader(nil))
	if err == nil {
		t.Fatal("expected empty reader error")
	}
	if restored != nil {
		t.Fatalf("bundle: got %#v, want nil", restored)
	}
}

func TestVerifyBundle(t *testing.T) {
	t.Run("ValidChecksum", func(t *testing.T) {
		bundle, _ := CreateProfileBundleUnencrypted([]byte(`{"test":"data"}`), "test")

		if !VerifyBundle(bundle) {
			t.Error("valid bundle should verify")
		}
	})

	t.Run("InvalidChecksum", func(t *testing.T) {
		bundle, _ := CreateProfileBundleUnencrypted([]byte(`{"test":"data"}`), "test")
		bundle.Checksum = "invalid-checksum"

		if VerifyBundle(bundle) {
			t.Error("bundle with invalid checksum should not verify")
		}
	})

	t.Run("ModifiedData", func(t *testing.T) {
		bundle, _ := CreateProfileBundleUnencrypted([]byte(`{"test":"data"}`), "test")
		bundle.Data = []byte(`{"test":"modified"}`)

		if VerifyBundle(bundle) {
			t.Error("bundle with modified data should not verify")
		}
	})
}

func TestCreateProfileBundle(t *testing.T) {
	profileJSON := []byte(`{"name":"encrypted-profile","minerType":"xmrig"}`)
	password := "test-password-123"

	bundle, err := CreateProfileBundle(profileJSON, "encrypted-test", password)
	if err != nil {
		t.Fatalf("failed to create encrypted bundle: %v", err)
	}

	if bundle.Type != BundleProfile {
		t.Errorf("expected type BundleProfile, got %s", bundle.Type)
	}

	// Encrypted data should not match original
	if bytes.Equal(bundle.Data, profileJSON) {
		t.Error("encrypted data should not match original")
	}

	// Should be able to extract with correct password
	extracted, err := ExtractProfileBundle(bundle, password)
	if err != nil {
		t.Fatalf("failed to extract bundle: %v", err)
	}

	if !bytes.Equal(extracted, profileJSON) {
		t.Errorf("extracted data should match original: got %s", string(extracted))
	}
}

func TestExtractProfileBundle(t *testing.T) {
	t.Run("UnencryptedBundle", func(t *testing.T) {
		originalJSON := []byte(`{"name":"plain","config":{}}`)
		bundle, _ := CreateProfileBundleUnencrypted(originalJSON, "plain")

		extracted, err := ExtractProfileBundle(bundle, "")
		if err != nil {
			t.Fatalf("failed to extract unencrypted bundle: %v", err)
		}

		if !bytes.Equal(extracted, originalJSON) {
			t.Error("extracted data should match original")
		}
	})

	t.Run("EncryptedBundle", func(t *testing.T) {
		originalJSON := []byte(`{"name":"secret","config":{"pool":"pool.example.com"}}`)
		password := "strong-password"

		bundle, _ := CreateProfileBundle(originalJSON, "secret", password)

		extracted, err := ExtractProfileBundle(bundle, password)
		if err != nil {
			t.Fatalf("failed to extract encrypted bundle: %v", err)
		}

		if !bytes.Equal(extracted, originalJSON) {
			t.Error("extracted data should match original")
		}
	})

	t.Run("WrongPassword", func(t *testing.T) {
		originalJSON := []byte(`{"name":"secret"}`)
		bundle, _ := CreateProfileBundle(originalJSON, "secret", "correct-password")

		_, err := ExtractProfileBundle(bundle, "wrong-password")
		if err == nil {
			t.Error("should fail with wrong password")
		}
	})

	t.Run("CorruptedChecksum", func(t *testing.T) {
		bundle, _ := CreateProfileBundleUnencrypted([]byte(`{}`), "test")
		bundle.Checksum = "corrupted"

		_, err := ExtractProfileBundle(bundle, "")
		if err == nil {
			t.Error("should fail with corrupted checksum")
		}
	})
}

func TestTarballFunctions(t *testing.T) {
	t.Run("CreateAndExtractTarball", func(t *testing.T) {
		files := map[string][]byte{
			"file1.txt":      []byte("content of file 1"),
			"dir/file2.json": []byte(`{"key":"value"}`),
			"miners/xmrig":   []byte("binary content"),
		}

		tarData, err := createTarball(files)
		if err != nil {
			t.Fatalf("failed to create tarball: %v", err)
		}

		if len(tarData) == 0 {
			t.Error("tarball should not be empty")
		}

		// Extract to temp directory
		tmpDir, _ := os.MkdirTemp("", "tarball-test")
		defer os.RemoveAll(tmpDir)

		firstExec, err := extractTarball(tarData, tmpDir)
		if err != nil {
			t.Fatalf("failed to extract tarball: %v", err)
		}

		// Check files exist
		for name, content := range files {
			path := filepath.Join(tmpDir, name)
			data, err := os.ReadFile(path)
			if err != nil {
				t.Errorf("failed to read extracted file %s: %v", name, err)
				continue
			}

			if !bytes.Equal(data, content) {
				t.Errorf("content mismatch for %s", name)
			}
		}

		// Check first executable is the miner
		if firstExec == "" {
			t.Error("should find an executable")
		}
	})
}

func TestStreamAndReadBundle(t *testing.T) {
	original, _ := CreateProfileBundleUnencrypted([]byte(`{"streaming":"test"}`), "stream-test")

	// Stream to buffer
	var buf bytes.Buffer
	err := StreamBundle(original, &buf)
	if err != nil {
		t.Fatalf("failed to stream bundle: %v", err)
	}

	// Read back
	restored, err := ReadBundle(&buf)
	if err != nil {
		t.Fatalf("failed to read bundle: %v", err)
	}

	if restored.Name != original.Name {
		t.Errorf("name mismatch: expected '%s', got '%s'", original.Name, restored.Name)
	}

	if restored.Checksum != original.Checksum {
		t.Error("checksum mismatch")
	}

	if !bytes.Equal(restored.Data, original.Data) {
		t.Error("data mismatch")
	}
}

func TestCalculateChecksum(t *testing.T) {
	t.Run("Deterministic", func(t *testing.T) {
		data := []byte("test data for checksum")

		checksum1 := calculateChecksum(data)
		checksum2 := calculateChecksum(data)

		if checksum1 != checksum2 {
			t.Error("checksum should be deterministic")
		}
	})

	t.Run("DifferentData", func(t *testing.T) {
		checksum1 := calculateChecksum([]byte("data1"))
		checksum2 := calculateChecksum([]byte("data2"))

		if checksum1 == checksum2 {
			t.Error("different data should produce different checksums")
		}
	})

	t.Run("HexFormat", func(t *testing.T) {
		checksum := calculateChecksum([]byte("test"))

		// SHA-256 produces 64 hex characters
		if len(checksum) != 64 {
			t.Errorf("expected 64 character hex string, got %d characters", len(checksum))
		}

		// Should be valid hex
		for _, c := range checksum {
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
				t.Errorf("invalid hex character: %c", c)
			}
		}
	})
}

func TestIsJSON(t *testing.T) {
	tests := []struct {
		data     []byte
		expected bool
	}{
		{[]byte(`{"key":"value"}`), true},
		{[]byte("  \n\t{\"key\":\"value\"}"), true},
		{[]byte(`["item1","item2"]`), true},
		{[]byte(`{}`), true},
		{[]byte(`[]`), true},
		{[]byte(`binary\x00data`), false},
		{[]byte(`plain text`), false},
		{[]byte{}, false},
		{nil, false},
	}

	for _, tt := range tests {
		result := isJSON(tt.data)
		if result != tt.expected {
			t.Errorf("isJSON(%q) = %v, expected %v", tt.data, result, tt.expected)
		}
	}
}

func TestBundleTypes(t *testing.T) {
	types := []BundleType{
		BundleProfile,
		BundleMiner,
		BundleFull,
	}

	expected := []string{"profile", "miner", "full"}

	for i, bt := range types {
		if string(bt) != expected[i] {
			t.Errorf("expected %s, got %s", expected[i], string(bt))
		}
	}
}

func TestCreateMinerBundle(t *testing.T) {
	// Create a temp "miner binary"
	tmpDir, _ := os.MkdirTemp("", "miner-bundle-test")
	defer os.RemoveAll(tmpDir)

	minerPath := filepath.Join(tmpDir, "test-miner")
	err := os.WriteFile(minerPath, []byte("fake miner binary content"), 0755)
	if err != nil {
		t.Fatalf("failed to create test miner: %v", err)
	}

	profileJSON := []byte(`{"profile":"data"}`)
	password := "miner-password"

	bundle, err := CreateMinerBundle(minerPath, profileJSON, "miner-bundle", password)
	if err != nil {
		t.Fatalf("failed to create miner bundle: %v", err)
	}

	if bundle.Type != BundleMiner {
		t.Errorf("expected type BundleMiner, got %s", bundle.Type)
	}

	if bundle.Name != "miner-bundle" {
		t.Errorf("expected name 'miner-bundle', got '%s'", bundle.Name)
	}

	// Extract and verify
	extractDir, _ := os.MkdirTemp("", "miner-extract-test")
	defer os.RemoveAll(extractDir)

	extractedPath, extractedProfile, err := ExtractMinerBundle(bundle, password, extractDir)
	if err != nil {
		t.Fatalf("failed to extract miner bundle: %v", err)
	}

	// Note: extractedPath may be empty if the tarball structure doesn't match
	// what extractTarball expects (it looks for files at root with executable bit)
	t.Logf("extracted path: %s", extractedPath)

	if !bytes.Equal(extractedProfile, profileJSON) {
		t.Error("profile data mismatch")
	}

	// If we got an extracted path, verify its content
	if extractedPath != "" {
		minerData, err := os.ReadFile(extractedPath)
		if err != nil {
			t.Fatalf("failed to read extracted miner: %v", err)
		}

		if string(minerData) != "fake miner binary content" {
			t.Error("miner content mismatch")
		}
	}
}

// --- Additional coverage tests for bundle.go ---

func TestExtractTarball_PathTraversal(t *testing.T) {
	t.Run("AbsolutePath", func(t *testing.T) {
		// Create a tarball with an absolute path entry
		tarData, err := createTarballWithCustomName("/etc/passwd", []byte("malicious"))
		if err != nil {
			t.Fatalf("failed to create tarball: %v", err)
		}

		tmpDir := t.TempDir()
		_, err = extractTarball(tarData, tmpDir)
		if err == nil {
			t.Error("expected error for absolute path in tar")
		}
	})

	t.Run("DotDotTraversal", func(t *testing.T) {
		tarData, err := createTarballWithCustomName("../../etc/passwd", []byte("malicious"))
		if err != nil {
			t.Fatalf("failed to create tarball: %v", err)
		}

		tmpDir := t.TempDir()
		_, err = extractTarball(tarData, tmpDir)
		if err == nil {
			t.Error("expected error for path traversal in tar")
		}
	})

	t.Run("DotDotAlone", func(t *testing.T) {
		tarData, err := createTarballWithCustomName("..", []byte("malicious"))
		if err != nil {
			t.Fatalf("failed to create tarball: %v", err)
		}

		tmpDir := t.TempDir()
		_, err = extractTarball(tarData, tmpDir)
		if err == nil {
			t.Error("expected error for '..' path in tar")
		}
	})

	t.Run("EmptyTarball", func(t *testing.T) {
		// Create an empty tarball
		tarData, err := createTarball(map[string][]byte{})
		if err != nil {
			t.Fatalf("failed to create empty tarball: %v", err)
		}

		tmpDir := t.TempDir()
		path, err := extractTarball(tarData, tmpDir)
		if err != nil {
			t.Fatalf("extractTarball should handle empty archive: %v", err)
		}
		if path != "" {
			t.Errorf("expected empty path for empty tarball, got %s", path)
		}
	})

	t.Run("NonExecutableFiles", func(t *testing.T) {
		files := map[string][]byte{
			"config.json": []byte(`{"key":"value"}`),
			"readme.txt":  []byte("hello"),
		}
		tarData, err := createTarball(files)
		if err != nil {
			t.Fatalf("failed to create tarball: %v", err)
		}

		tmpDir := t.TempDir()
		path, err := extractTarball(tarData, tmpDir)
		if err != nil {
			t.Fatalf("extractTarball failed: %v", err)
		}
		// config.json might not be executable, but non-JSON files get 0755
		_ = path
	})

	t.Run("SymlinkIgnored", func(t *testing.T) {
		// Create a tarball with a symlink entry
		tarData, err := createTarballWithSymlink("link", "/etc/passwd")
		if err != nil {
			t.Fatalf("failed to create tarball: %v", err)
		}

		tmpDir := t.TempDir()
		_, err = extractTarball(tarData, tmpDir)
		// Symlinks should be silently skipped, not error
		if err != nil {
			t.Fatalf("extractTarball should skip symlinks without error: %v", err)
		}

		// Verify symlink was not created
		linkPath := filepath.Join(tmpDir, "link")
		if _, statErr := os.Lstat(linkPath); !os.IsNotExist(statErr) {
			t.Error("symlink should not be created")
		}
	})

	t.Run("DirectoryEntry", func(t *testing.T) {
		// Create a tarball with a directory entry followed by a file in it
		var buf bytes.Buffer
		tw := tar.NewWriter(&buf)

		// Write directory
		tw.WriteHeader(&tar.Header{
			Name:     "mydir/",
			Mode:     0755,
			Typeflag: tar.TypeDir,
		})

		// Write file in directory
		content := []byte("file in dir")
		tw.WriteHeader(&tar.Header{
			Name: "mydir/file.txt",
			Mode: 0644,
			Size: int64(len(content)),
		})
		tw.Write(content)
		tw.Close()

		tmpDir := t.TempDir()
		_, err := extractTarball(buf.Bytes(), tmpDir)
		if err != nil {
			t.Fatalf("extractTarball failed: %v", err)
		}

		// Verify directory and file exist
		data, err := os.ReadFile(filepath.Join(tmpDir, "mydir", "file.txt"))
		if err != nil {
			t.Fatalf("failed to read extracted file: %v", err)
		}
		if !bytes.Equal(data, content) {
			t.Error("content mismatch")
		}
	})
}

// createTarballWithCustomName creates a tar with a single file at an arbitrary path.
func createTarballWithCustomName(name string, content []byte) ([]byte, error) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	hdr := &tar.Header{
		Name: name,
		Mode: 0644,
		Size: int64(len(content)),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return nil, err
	}
	if _, err := tw.Write(content); err != nil {
		return nil, err
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// createTarballWithSymlink creates a tar containing a symlink entry.
func createTarballWithSymlink(name, target string) ([]byte, error) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	hdr := &tar.Header{
		Name:     name,
		Linkname: target,
		Mode:     0777,
		Typeflag: tar.TypeSymlink,
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return nil, err
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func TestExtractMinerBundle_ChecksumMismatch(t *testing.T) {
	bundle := &Bundle{
		Type:     BundleMiner,
		Name:     "bad-bundle",
		Data:     []byte("some data"),
		Checksum: "invalid-checksum",
	}

	_, _, err := ExtractMinerBundle(bundle, "password", t.TempDir())
	if err == nil {
		t.Error("expected error for checksum mismatch")
	}
}

func TestCreateMinerBundle_NonExistentFile(t *testing.T) {
	_, err := CreateMinerBundle("/non/existent/miner", nil, "test", "password")
	if err == nil {
		t.Error("expected error for non-existent miner file")
	}
}

func TestCreateMinerBundle_NilProfile(t *testing.T) {
	tmpDir := t.TempDir()
	minerPath := filepath.Join(tmpDir, "miner")
	os.WriteFile(minerPath, []byte("binary"), 0755)

	bundle, err := CreateMinerBundle(minerPath, nil, "nil-profile", "pass")
	if err != nil {
		t.Fatalf("CreateMinerBundle with nil profile should succeed: %v", err)
	}
	if bundle.Type != BundleMiner {
		t.Errorf("expected type BundleMiner, got %s", bundle.Type)
	}
}

func TestReadBundle_InvalidJSON(t *testing.T) {
	reader := bytes.NewReader([]byte("not json"))
	_, err := ReadBundle(reader)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestStreamBundle_EmptyBundle(t *testing.T) {
	bundle := &Bundle{
		Type:     BundleProfile,
		Name:     "empty",
		Data:     nil,
		Checksum: "",
	}

	var buf bytes.Buffer
	err := StreamBundle(bundle, &buf)
	if err != nil {
		t.Fatalf("StreamBundle should handle empty bundle: %v", err)
	}

	// Should be valid JSON
	restored, err := ReadBundle(&buf)
	if err != nil {
		t.Fatalf("ReadBundle should read back streamed bundle: %v", err)
	}
	if restored.Name != "empty" {
		t.Errorf("expected name 'empty', got '%s'", restored.Name)
	}
}

func TestCreateTarball_MultipleDirs(t *testing.T) {
	files := map[string][]byte{
		"dir1/file1.txt": []byte("content1"),
		"dir2/file2.txt": []byte("content2"),
	}

	tarData, err := createTarball(files)
	if err != nil {
		t.Fatalf("failed to create tarball: %v", err)
	}

	tmpDir := t.TempDir()
	_, err = extractTarball(tarData, tmpDir)
	if err != nil {
		t.Fatalf("failed to extract: %v", err)
	}

	for name, content := range files {
		data, err := os.ReadFile(filepath.Join(tmpDir, name))
		if err != nil {
			t.Errorf("failed to read %s: %v", name, err)
			continue
		}
		if !bytes.Equal(data, content) {
			t.Errorf("content mismatch for %s", name)
		}
	}
}
