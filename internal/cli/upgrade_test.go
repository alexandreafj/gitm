package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"testing"
)

func TestAssetName(t *testing.T) {
	tests := []struct {
		goos, goarch string
		want         string
		wantErr      bool
	}{
		{"darwin", "amd64", "gitm-macos-x86_64", false},
		{"darwin", "arm64", "gitm-macos-arm64", false},
		{"linux", "amd64", "gitm-linux-amd64", false},
		{"linux", "arm64", "gitm-linux-arm64", false},
		{"windows", "amd64", "gitm-windows-amd64.exe", false},
		{"freebsd", "amd64", "", true},
		{"linux", "386", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.goos+"/"+tt.goarch, func(t *testing.T) {
			got, err := assetName(tt.goos, tt.goarch)
			if (err != nil) != tt.wantErr {
				t.Fatalf("assetName(%q, %q) error = %v, wantErr %v", tt.goos, tt.goarch, err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("assetName(%q, %q) = %q, want %q", tt.goos, tt.goarch, got, tt.want)
			}
		})
	}
}

func TestFindAssetURL(t *testing.T) {
	assets := []ghAsset{
		{Name: "gitm-macos-arm64", BrowserDownloadURL: "https://example.com/macos"},
		{Name: "gitm-linux-amd64", BrowserDownloadURL: "https://example.com/linux"},
		{Name: "checksums.txt", BrowserDownloadURL: "https://example.com/checksums"},
	}

	url, ok := findAssetURL(assets, "gitm-linux-amd64")
	if !ok || url != "https://example.com/linux" {
		t.Errorf("expected linux URL, got %q ok=%v", url, ok)
	}

	_, ok = findAssetURL(assets, "gitm-windows-amd64.exe")
	if ok {
		t.Error("expected not found for windows asset")
	}
}

func TestParseChecksums(t *testing.T) {
	data := `abc123  gitm-macos-arm64
def456  gitm-linux-amd64
789abc  checksums.txt
`
	m := parseChecksums(data)
	if len(m) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(m))
	}
	if m["gitm-macos-arm64"] != "abc123" {
		t.Errorf("unexpected checksum for macos: %q", m["gitm-macos-arm64"])
	}
	if m["gitm-linux-amd64"] != "def456" {
		t.Errorf("unexpected checksum for linux: %q", m["gitm-linux-amd64"])
	}
}

func TestParseChecksumsEmpty(t *testing.T) {
	m := parseChecksums("")
	if len(m) != 0 {
		t.Errorf("expected 0 entries for empty input, got %d", len(m))
	}
}

func TestFileSHA256(t *testing.T) {
	content := []byte("hello gitm")
	f, err := os.CreateTemp("", "sha256test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	f.Write(content)
	f.Close()

	got, err := fileSHA256(f.Name())
	if err != nil {
		t.Fatal(err)
	}

	h := sha256.Sum256(content)
	want := hex.EncodeToString(h[:])
	if got != want {
		t.Errorf("fileSHA256 = %q, want %q", got, want)
	}
}

type fakeUpgradeClient struct {
	release *ghRelease
	err     error
	files   map[string][]byte
}

func (f *fakeUpgradeClient) fetchLatestRelease() (*ghRelease, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.release, nil
}

func (f *fakeUpgradeClient) downloadToFile(url, path string) error {
	data, ok := f.files[url]
	if !ok {
		return fmt.Errorf("not found: %s", url)
	}
	return os.WriteFile(path, data, 0644)
}

func TestRunUpgradeAlreadyUpToDate(t *testing.T) {
	uc := &fakeUpgradeClient{
		release: &ghRelease{TagName: "v1.0.0"},
	}
	err := runUpgrade("v1.0.0", uc)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestRunUpgradeFetchError(t *testing.T) {
	uc := &fakeUpgradeClient{
		err: fmt.Errorf("network error"),
	}
	err := runUpgrade("v1.0.0", uc)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() != "network error" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunUpgradeUnsupportedPlatform(t *testing.T) {
	uc := &fakeUpgradeClient{
		release: &ghRelease{
			TagName: "v2.0.0",
			Assets:  []ghAsset{},
		},
	}
	err := runUpgrade("v1.0.0", uc)
	if err == nil {
		t.Fatal("expected error for missing asset")
	}
}

func TestUpgradeCmd(t *testing.T) {
	cmd := upgradeCmd("v1.0.0")
	if cmd.Use != "upgrade" {
		t.Errorf("Use = %q, want %q", cmd.Use, "upgrade")
	}
	if cmd.Short == "" {
		t.Error("Short is empty")
	}
}

func TestUpgradeCmdSkipsDB(t *testing.T) {
	root := Root("v1.0.0")
	var found bool
	for _, c := range root.Commands() {
		if c.Name() == "upgrade" {
			found = true
			break
		}
	}
	if !found {
		t.Error("upgrade subcommand not registered")
	}
}

func TestRootVersion(t *testing.T) {
	cmd := Root("v1.2.3")
	if cmd.Version != "v1.2.3" {
		t.Errorf("Root().Version = %q, want %q", cmd.Version, "v1.2.3")
	}
}
