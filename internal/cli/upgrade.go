package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

const (
	releaseAPIURL = "https://api.github.com/repos/alexandreafj/gitm/releases/latest"
	httpTimeout   = 60 * time.Second
	userAgent     = "gitm-upgrade"
)

type ghRelease struct {
	TagName string    `json:"tag_name"`
	Assets  []ghAsset `json:"assets"`
}

type ghAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

func assetName(goos, goarch string) (string, error) {
	switch {
	case goos == "darwin" && goarch == "amd64":
		return "gitm-macos-x86_64", nil
	case goos == "darwin" && goarch == "arm64":
		return "gitm-macos-arm64", nil
	case goos == "linux" && goarch == "amd64":
		return "gitm-linux-amd64", nil
	case goos == "linux" && goarch == "arm64":
		return "gitm-linux-arm64", nil
	case goos == "windows" && goarch == "amd64":
		return "gitm-windows-amd64.exe", nil
	default:
		return "", fmt.Errorf("unsupported platform: %s/%s", goos, goarch)
	}
}

func findAssetURL(assets []ghAsset, name string) (string, bool) {
	for _, a := range assets {
		if a.Name == name {
			return a.BrowserDownloadURL, true
		}
	}
	return "", false
}

func parseChecksums(data string) map[string]string {
	m := make(map[string]string)
	for _, line := range strings.Split(strings.TrimSpace(data), "\n") {
		parts := strings.Fields(line)
		if len(parts) == 2 {
			m[parts[1]] = parts[0]
		}
	}
	return m
}

type upgradeClient interface {
	fetchLatestRelease() (*ghRelease, error)
	downloadToFile(url, path string) error
}

type httpUpgradeClient struct {
	client *http.Client
}

func newHTTPUpgradeClient() *httpUpgradeClient {
	return &httpUpgradeClient{
		client: &http.Client{Timeout: httpTimeout},
	}
}

func (h *httpUpgradeClient) fetchLatestRelease() (*ghRelease, error) {
	req, err := http.NewRequest("GET", releaseAPIURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", userAgent)

	resp, err := h.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch latest release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusForbidden {
		return nil, fmt.Errorf("GitHub API rate limit exceeded (HTTP 403); try again later")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	var rel ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, fmt.Errorf("decode release: %w", err)
	}
	return &rel, nil
}

func (h *httpUpgradeClient) downloadToFile(url, path string) error {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := h.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download returned %d", resp.StatusCode)
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}

	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// installBinary atomically replaces the binary at execPath with the one at srcPath.
func installBinary(srcPath, execPath string) error {
	dir := filepath.Dir(execPath)

	// Remove any leftover backup from a previous failed upgrade.
	backupPath := execPath + ".old"
	_ = os.Remove(backupPath)

	// Write the new binary to a temp file in the same directory so rename is atomic.
	tmpFile, err := os.CreateTemp(dir, ".gitm-new-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()

	if err := copyFile(srcPath, tmpPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("copy new binary: %w", err)
	}

	if runtime.GOOS != "windows" {
		if err := os.Chmod(tmpPath, 0755); err != nil {
			os.Remove(tmpPath)
			return fmt.Errorf("set executable permission: %w", err)
		}
	}

	// Back up the current binary so we can restore on failure.
	if err := os.Rename(execPath, backupPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("backup current binary: %w", err)
	}

	// Atomic rename of the new binary into place.
	if err := os.Rename(tmpPath, execPath); err != nil {
		if rbErr := os.Rename(backupPath, execPath); rbErr != nil {
			return fmt.Errorf("install new binary: %w (rollback also failed: %w)", err, rbErr)
		}
		return fmt.Errorf("install new binary: %w", err)
	}

	// Best-effort cleanup of the backup.
	_ = os.Remove(backupPath)
	return nil
}

func runUpgrade(currentVersion string, uc upgradeClient) error {
	bold := color.New(color.Bold)
	green := color.New(color.FgGreen, color.Bold)

	bold.Print("Checking for updates... ")

	rel, err := uc.fetchLatestRelease()
	if err != nil {
		return err
	}

	if rel.TagName == currentVersion {
		green.Println("already up to date (" + currentVersion + ")")
		return nil
	}

	fmt.Println("found " + rel.TagName)

	name, err := assetName(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return err
	}

	binaryURL, ok := findAssetURL(rel.Assets, name)
	if !ok {
		return fmt.Errorf("no binary %q in release %s", name, rel.TagName)
	}

	checksumURL, hasChecksum := findAssetURL(rel.Assets, "checksums.txt")

	tmpFile, err := os.CreateTemp("", "gitm-upgrade-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	fmt.Printf("Downloading %s... ", name)
	if err := uc.downloadToFile(binaryURL, tmpPath); err != nil {
		return fmt.Errorf("download binary: %w", err)
	}
	fmt.Println("done")

	if hasChecksum {
		fmt.Print("Verifying checksum... ")
		csFile, err := os.CreateTemp("", "gitm-checksums-*")
		if err != nil {
			return fmt.Errorf("create checksum temp file: %w", err)
		}
		csPath := csFile.Name()
		csFile.Close()
		defer os.Remove(csPath)

		if err := uc.downloadToFile(checksumURL, csPath); err != nil {
			return fmt.Errorf("download checksums: %w", err)
		}

		csData, err := os.ReadFile(csPath)
		if err != nil {
			return fmt.Errorf("read checksums: %w", err)
		}

		checksums := parseChecksums(string(csData))
		expected, ok := checksums[name]
		if !ok {
			return fmt.Errorf("no checksum entry for %s", name)
		}

		actual, err := fileSHA256(tmpPath)
		if err != nil {
			return fmt.Errorf("hash downloaded binary: %w", err)
		}

		if actual != expected {
			return fmt.Errorf("checksum mismatch: expected %s, got %s", expected, actual)
		}
		green.Println("ok")
	}

	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate current binary: %w", err)
	}

	if err := installBinary(tmpPath, execPath); err != nil {
		return err
	}

	green.Printf("Updated gitm: %s → %s\n", currentVersion, rel.TagName)
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}

	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	if err := out.Sync(); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

func upgradeCmd(currentVersion string) *cobra.Command {
	return &cobra.Command{
		Use:   "upgrade",
		Short: "Upgrade gitm to the latest release",
		Long:  "Download and install the latest gitm binary from GitHub releases.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUpgrade(currentVersion, newHTTPUpgradeClient())
		},
	}
}
