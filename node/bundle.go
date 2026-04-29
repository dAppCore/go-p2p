package node

import (
	"archive/tar"
	"crypto/sha256"
	"encoding/hex"
	"io"

	core "dappco.re/go"
	coreio "dappco.re/go/io"
	coreerr "dappco.re/go/log"

	"forge.lthn.ai/Snider/Borg/pkg/datanode"
	"forge.lthn.ai/Snider/Borg/pkg/tim"
)

// BundleType defines the type of deployment bundle.
type BundleType string

const (
	BundleProfile BundleType = "profile" // Just config/profile JSON
	BundleMiner   BundleType = "miner"   // Miner binary + config
	BundleFull    BundleType = "full"    // Everything (miner + profiles + config)
)

// Bundle represents a deployment bundle for P2P transfer.
type Bundle struct {
	Type     BundleType `json:"type"`
	Name     string     `json:"name"`
	Data     []byte     `json:"data"`     // Encrypted STIM data or raw JSON
	Checksum string     `json:"checksum"` // SHA-256 of Data
}

// BundleManifest describes the contents of a bundle.
type BundleManifest struct {
	Type       BundleType `json:"type"`
	Name       string     `json:"name"`
	Version    string     `json:"version,omitempty"`
	MinerType  string     `json:"minerType,omitempty"`
	ProfileIDs []string   `json:"profileIds,omitempty"`
	CreatedAt  string     `json:"createdAt"`
}

// CreateProfileBundle creates an encrypted bundle containing a mining profile.
func CreateProfileBundle(profileJSON []byte, name string, password string) (*Bundle, error) {
	// Create a TIM with just the profile config
	t, err := tim.New()
	if err != nil {
		return nil, coreerr.E("CreateProfileBundle", "failed to create TIM", err)
	}
	t.Config = profileJSON

	// Encrypt to STIM format
	stimData, err := t.ToSigil(password)
	if err != nil {
		return nil, coreerr.E("CreateProfileBundle", "failed to encrypt bundle", err)
	}

	// Calculate checksum
	checksum := calculateChecksum(stimData)

	return &Bundle{
		Type:     BundleProfile,
		Name:     name,
		Data:     stimData,
		Checksum: checksum,
	}, nil
}

// CreateProfileBundleUnencrypted creates a plain JSON bundle (for testing or trusted networks).
func CreateProfileBundleUnencrypted(profileJSON []byte, name string) (*Bundle, error) {
	checksum := calculateChecksum(profileJSON)

	return &Bundle{
		Type:     BundleProfile,
		Name:     name,
		Data:     profileJSON,
		Checksum: checksum,
	}, nil
}

// CreateMinerBundle creates an encrypted bundle containing a miner binary and optional profile.
func CreateMinerBundle(minerPath string, profileJSON []byte, name string, password string) (*Bundle, error) {
	// Read miner binary
	minerContent, err := coreio.Local.Read(minerPath)
	if err != nil {
		return nil, coreerr.E("CreateMinerBundle", "failed to read miner binary", err)
	}
	minerData := []byte(minerContent)

	// Create a tarball with the miner binary
	tarData, err := createTarball(map[string][]byte{
		core.PathBase(minerPath): minerData,
	})
	if err != nil {
		return nil, coreerr.E("CreateMinerBundle", "failed to create tarball", err)
	}

	// Create DataNode from tarball
	dn, err := datanode.FromTar(tarData)
	if err != nil {
		return nil, coreerr.E("CreateMinerBundle", "failed to create datanode", err)
	}

	// Create TIM from DataNode
	t, err := tim.FromDataNode(dn)
	if err != nil {
		return nil, coreerr.E("CreateMinerBundle", "failed to create TIM", err)
	}

	// Set profile as config if provided
	if profileJSON != nil {
		t.Config = profileJSON
	}

	// Encrypt to STIM format
	stimData, err := t.ToSigil(password)
	if err != nil {
		return nil, coreerr.E("CreateMinerBundle", "failed to encrypt bundle", err)
	}

	checksum := calculateChecksum(stimData)

	return &Bundle{
		Type:     BundleMiner,
		Name:     name,
		Data:     stimData,
		Checksum: checksum,
	}, nil
}

// ExtractProfileBundle decrypts and extracts a profile bundle.
func ExtractProfileBundle(bundle *Bundle, password string) ([]byte, error) {
	// Verify checksum first
	if calculateChecksum(bundle.Data) != bundle.Checksum {
		return nil, coreerr.E("ExtractProfileBundle", "checksum mismatch - bundle may be corrupted", nil)
	}

	// If it's unencrypted JSON, just return it
	if isJSON(bundle.Data) {
		return bundle.Data, nil
	}

	// Decrypt STIM format
	t, err := tim.FromSigil(bundle.Data, password)
	if err != nil {
		return nil, coreerr.E("ExtractProfileBundle", "failed to decrypt bundle", err)
	}

	return t.Config, nil
}

// ExtractMinerBundle decrypts and extracts a miner bundle, returning the miner path and profile.
func ExtractMinerBundle(bundle *Bundle, password string, destDir string) (string, []byte, error) {
	// Verify checksum
	if calculateChecksum(bundle.Data) != bundle.Checksum {
		return "", nil, coreerr.E("ExtractMinerBundle", "checksum mismatch - bundle may be corrupted", nil)
	}

	// Decrypt STIM format
	t, err := tim.FromSigil(bundle.Data, password)
	if err != nil {
		return "", nil, coreerr.E("ExtractMinerBundle", "failed to decrypt bundle", err)
	}

	// Convert rootfs to tarball and extract
	tarData, err := t.RootFS.ToTar()
	if err != nil {
		return "", nil, coreerr.E("ExtractMinerBundle", "failed to convert rootfs to tar", err)
	}

	// Extract tarball to destination
	minerPath, err := extractTarball(tarData, destDir)
	if err != nil {
		return "", nil, coreerr.E("ExtractMinerBundle", "failed to extract tarball", err)
	}

	return minerPath, t.Config, nil
}

// VerifyBundle checks if a bundle's checksum is valid.
func VerifyBundle(bundle *Bundle) bool {
	return calculateChecksum(bundle.Data) == bundle.Checksum
}

// calculateChecksum computes SHA-256 checksum of data.
func calculateChecksum(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// isJSON checks if data starts with JSON characters.
func isJSON(data []byte) bool {
	data = []byte(core.Trim(string(data)))
	if len(data) == 0 {
		return false
	}
	// JSON typically starts with { or [
	return data[0] == '{' || data[0] == '['
}

// createTarball creates a tar archive from a map of filename -> content.
func createTarball(files map[string][]byte) ([]byte, error) {
	buf := core.NewBuffer()
	tw := tar.NewWriter(buf)

	// Track directories we've created
	dirs := make(map[string]bool)

	for name, content := range files {
		// Create parent directories if needed
		dir := core.PathDir(name)
		if dir != "." && !dirs[dir] {
			hdr := &tar.Header{
				Name:     dir + "/",
				Mode:     0755,
				Typeflag: tar.TypeDir,
			}
			if err := tw.WriteHeader(hdr); err != nil {
				return nil, err
			}
			dirs[dir] = true
		}

		// Determine file mode (executable for binaries in miners/)
		mode := int64(0644)
		if core.PathDir(name) == "miners" || !isJSON(content) {
			mode = 0755
		}

		hdr := &tar.Header{
			Name: name,
			Mode: mode,
			Size: int64(len(content)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return nil, err
		}
		if _, err := tw.Write(content); err != nil {
			return nil, err
		}
	}

	if err := tw.Close(); err != nil {
		return nil, err
	}

	out := make([]byte, len(buf.Bytes()))
	copy(out, buf.Bytes())
	return out, nil
}

// extractTarball extracts a tar archive to a directory, returns first executable found.
func extractTarball(tarData []byte, destDir string) (string, error) {
	// Ensure destDir is an absolute, clean path for security checks
	absResult := core.PathAbs(destDir)
	if !absResult.OK {
		if err, ok := absResult.Value.(error); ok {
			return "", coreerr.E("extractTarball", "failed to resolve destination directory", err)
		}
		return "", coreerr.E("extractTarball", "failed to resolve destination directory", nil)
	}
	absDestDir := core.CleanPath(absResult.Value.(string), string(core.PathSeparator))

	if err := coreio.Local.EnsureDir(absDestDir); err != nil {
		return "", err
	}

	tr := tar.NewReader(core.NewBuffer(tarData))
	var firstExecutable string

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}

		// Security: Sanitize the tar entry name to prevent path traversal (Zip Slip)
		cleanName := core.CleanPath(hdr.Name, string(core.PathSeparator))

		// Reject absolute paths
		if core.PathIsAbs(cleanName) {
			return "", coreerr.E("extractTarball", "invalid tar entry: absolute path not allowed: "+hdr.Name, nil)
		}

		// Reject paths that escape the destination directory
		if core.HasPrefix(cleanName, ".."+string(core.PathSeparator)) || cleanName == ".." {
			return "", coreerr.E("extractTarball", "invalid tar entry: path traversal attempt: "+hdr.Name, nil)
		}

		// Build the full path and verify it's within destDir
		fullPath := core.PathJoin(absDestDir, cleanName)
		fullPath = core.CleanPath(fullPath, string(core.PathSeparator))

		// Final security check: ensure the path is still within destDir
		if !core.HasPrefix(fullPath, absDestDir+string(core.PathSeparator)) && fullPath != absDestDir {
			return "", coreerr.E("extractTarball", "invalid tar entry: path escape attempt: "+hdr.Name, nil)
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := coreio.Local.EnsureDir(fullPath); err != nil {
				return "", err
			}
		case tar.TypeReg:
			// Ensure parent directory exists
			if err := coreio.Local.EnsureDir(core.PathDir(fullPath)); err != nil {
				return "", err
			}

			// core.OpenFile is used deliberately here instead of coreio.Local.Create/Write
			// because coreio hardcodes file permissions (0644) and we need to preserve
			// the tar header's mode bits — executable binaries require 0755.
			openResult := core.OpenFile(fullPath, core.O_CREATE|core.O_WRONLY|core.O_TRUNC, core.FileMode(hdr.Mode))
			if !openResult.OK {
				err, _ := openResult.Value.(error)
				return "", coreerr.E("extractTarball", "failed to create file "+hdr.Name, err)
			}
			f := openResult.Value.(*core.OSFile)

			// Limit file size to prevent decompression bombs (100MB max per file)
			const maxFileSize int64 = 100 * 1024 * 1024
			limitedReader := io.LimitReader(tr, maxFileSize+1)
			written, err := io.Copy(f, limitedReader)
			f.Close()
			if err != nil {
				return "", coreerr.E("extractTarball", "failed to write file "+hdr.Name, err)
			}
			if written > maxFileSize {
				coreio.Local.Delete(fullPath)
				return "", coreerr.E("extractTarball", "file "+hdr.Name+" exceeds maximum size", nil)
			}

			// Track first executable
			if hdr.Mode&0111 != 0 && firstExecutable == "" {
				firstExecutable = fullPath
			}
		// Explicitly ignore symlinks and hard links to prevent symlink attacks
		case tar.TypeSymlink, tar.TypeLink:
			// Skip symlinks and hard links for security
			continue
		}
	}

	return firstExecutable, nil
}

// StreamBundle writes a bundle to a writer (for large transfers).
func StreamBundle(bundle *Bundle, w io.Writer) error {
	data, err := MarshalJSON(bundle)
	if err != nil {
		return err
	}
	if _, err := w.Write(data); err != nil {
		return err
	}
	_, err = w.Write([]byte("\n"))
	return err
}

// ReadBundle reads a bundle from a reader.
func ReadBundle(r io.Reader) (*Bundle, error) {
	raw, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	var bundle Bundle
	result := core.JSONUnmarshal(raw, &bundle)
	if !result.OK {
		if err, ok := result.Value.(error); ok {
			return nil, err
		}
		return nil, core.NewError("bundle unmarshal failed")
	}
	return &bundle, nil
}
