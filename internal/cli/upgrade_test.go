package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
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
	if _, err := f.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

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

func TestFakeUpgradeClientDownloadBytes(t *testing.T) {
	uc := &fakeUpgradeClient{
		files: map[string][]byte{
			"https://example.com/x": []byte("hello"),
		},
	}
	got, err := uc.downloadBytes("https://example.com/x")
	if err != nil {
		t.Fatalf("downloadBytes error: %v", err)
	}
	if string(got) != "hello" {
		t.Errorf("got %q, want %q", got, "hello")
	}

	if _, err := uc.downloadBytes("https://nope.example.com"); err == nil {
		t.Fatal("expected error for unknown URL, got nil")
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

func (f *fakeUpgradeClient) downloadBytes(url string) ([]byte, error) {
	data, ok := f.files[url]
	if !ok {
		return nil, fmt.Errorf("not found: %s", url)
	}
	out := make([]byte, len(data))
	copy(out, data)
	return out, nil
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

func TestRunUpgradeMissingAsset(t *testing.T) {
	name, err := assetName(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		t.Fatal(err)
	}
	uc := &fakeUpgradeClient{
		release: &ghRelease{
			TagName: "v2.0.0",
			Assets:  []ghAsset{{Name: "some-other-binary", BrowserDownloadURL: "https://example.com/other"}},
		},
	}
	err = runUpgrade("v1.0.0", uc)
	if err == nil {
		t.Fatal("expected error for missing asset")
	}
	if !strings.Contains(err.Error(), name) {
		t.Errorf("error should mention expected asset %q, got: %v", name, err)
	}
}

func TestRunUpgradeChecksumMismatch(t *testing.T) {
	name, err := assetName(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		t.Fatal(err)
	}

	binaryContent := []byte("fake binary content")
	wrongChecksum := "0000000000000000000000000000000000000000000000000000000000000000"
	checksumData := fmt.Sprintf("%s  %s\n", wrongChecksum, name)

	uc := &fakeUpgradeClient{
		release: &ghRelease{
			TagName: "v2.0.0",
			Assets: []ghAsset{
				{Name: name, BrowserDownloadURL: "https://example.com/binary"},
				{Name: "checksums.txt", BrowserDownloadURL: "https://example.com/checksums"},
			},
		},
		files: map[string][]byte{
			"https://example.com/binary":    binaryContent,
			"https://example.com/checksums": []byte(checksumData),
		},
	}
	err = runUpgrade("v1.0.0", uc)
	if err == nil {
		t.Fatal("expected checksum mismatch error")
	}
	if !strings.Contains(err.Error(), "checksum mismatch") {
		t.Errorf("expected checksum mismatch error, got: %v", err)
	}
}

func TestInstallBinarySuccess(t *testing.T) {
	dir := t.TempDir()

	oldBinary := filepath.Join(dir, "gitm")
	if err := os.WriteFile(oldBinary, []byte("old"), 0755); err != nil {
		t.Fatal(err)
	}

	newBinary := filepath.Join(dir, "new-gitm")
	if err := os.WriteFile(newBinary, []byte("new"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := installBinary(newBinary, oldBinary); err != nil {
		t.Fatalf("installBinary: %v", err)
	}

	content, err := os.ReadFile(oldBinary)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "new" {
		t.Errorf("expected 'new', got %q", string(content))
	}

	if runtime.GOOS != "windows" {
		info, err := os.Stat(oldBinary)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode()&0111 == 0 {
			t.Error("expected executable permissions on installed binary")
		}
	}

	if _, err := os.Stat(oldBinary + ".old"); !os.IsNotExist(err) {
		t.Error("backup file should have been cleaned up")
	}
}

func TestInstallBinaryRemovesStaleBackup(t *testing.T) {
	dir := t.TempDir()

	oldBinary := filepath.Join(dir, "gitm")
	if err := os.WriteFile(oldBinary, []byte("old"), 0755); err != nil {
		t.Fatal(err)
	}

	staleBackup := oldBinary + ".old"
	if err := os.WriteFile(staleBackup, []byte("stale"), 0644); err != nil {
		t.Fatal(err)
	}

	newBinary := filepath.Join(dir, "new-gitm")
	if err := os.WriteFile(newBinary, []byte("new"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := installBinary(newBinary, oldBinary); err != nil {
		t.Fatalf("installBinary with stale backup: %v", err)
	}

	content, err := os.ReadFile(oldBinary)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "new" {
		t.Errorf("expected 'new', got %q", string(content))
	}
}

func TestInstallBinaryRollbackOnFailure(t *testing.T) {
	dir := t.TempDir()

	oldBinary := filepath.Join(dir, "gitm")
	if err := os.WriteFile(oldBinary, []byte("old"), 0755); err != nil {
		t.Fatal(err)
	}

	err := installBinary("/nonexistent/path", oldBinary)
	if err == nil {
		t.Fatal("expected error for nonexistent source")
	}

	content, err := os.ReadFile(oldBinary)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "old" {
		t.Errorf("expected original content 'old' after rollback, got %q", string(content))
	}
}

func TestCopyFile(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	dst := filepath.Join(dir, "dst")

	if err := os.WriteFile(src, []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := copyFile(src, dst); err != nil {
		t.Fatalf("copyFile: %v", err)
	}

	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "content" {
		t.Errorf("expected 'content', got %q", string(got))
	}
}

func TestCopyFileSourceNotFound(t *testing.T) {
	dir := t.TempDir()
	err := copyFile("/nonexistent", filepath.Join(dir, "dst"))
	if err == nil {
		t.Fatal("expected error for nonexistent source")
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

func TestUpgradeSubcommandRegistered(t *testing.T) {
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
