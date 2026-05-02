package cli

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
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
	err := runUpgrade("v1.0.0", uc, &fakeSignatureVerifier{}, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestRunUpgradeFetchError(t *testing.T) {
	uc := &fakeUpgradeClient{
		err: fmt.Errorf("network error"),
	}
	err := runUpgrade("v1.0.0", uc, &fakeSignatureVerifier{}, nil)
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
	err = runUpgrade("v1.0.0", uc, &fakeSignatureVerifier{}, nil)
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
	err = runUpgrade("v1.0.0", uc, &fakeSignatureVerifier{}, nil)
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
	cmd := newUpgradeCmd("v1.0.0", "darwin", installChannelManual)
	if cmd.Use != "upgrade" {
		t.Errorf("Use = %q, want %q", cmd.Use, "upgrade")
	}
	if cmd.Short == "" {
		t.Error("Short is empty")
	}
	if cmd.Hidden {
		t.Error("upgrade command should be visible for manual installs")
	}
}

func TestDetectInstallChannel(t *testing.T) {
	tests := []struct {
		name     string
		execPath string
		want     installChannel
	}{
		{name: "homebrew cask", execPath: "/opt/homebrew/Caskroom/gitm/1.0.0/gitm", want: installChannelHomebrew},
		{name: "scoop apps", execPath: `C:\Users\alex\scoop\apps\gitm\current\gitm.exe`, want: installChannelScoop},
		{name: "scoop shim", execPath: `C:\Users\alex\scoop\shims\gitm.exe`, want: installChannelScoop},
		{name: "manual", execPath: "/usr/local/bin/gitm", want: installChannelManual},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detectInstallChannel(tt.execPath)
			if got != tt.want {
				t.Errorf("detectInstallChannel(%q) = %q, want %q", tt.execPath, got, tt.want)
			}
		})
	}
}

func TestShouldHideUpgradeCommand(t *testing.T) {
	tests := []struct {
		name    string
		channel installChannel
		want    bool
	}{
		{name: "homebrew", channel: installChannelHomebrew, want: true},
		{name: "scoop", channel: installChannelScoop, want: true},
		{name: "manual", channel: installChannelManual, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldHideUpgradeCommand(tt.channel)
			if got != tt.want {
				t.Errorf("shouldHideUpgradeCommand(%q) = %v, want %v", tt.channel, got, tt.want)
			}
		})
	}
}

func TestUpgradeBlockedReason(t *testing.T) {
	tests := []struct {
		name    string
		goos    string
		channel installChannel
		want    string
	}{
		{name: "homebrew", goos: "darwin", channel: installChannelHomebrew, want: "brew upgrade --cask gitm"},
		{name: "scoop", goos: "windows", channel: installChannelScoop, want: "scoop update gitm"},
		{name: "windows manual", goos: "windows", channel: installChannelManual, want: "not supported on Windows"},
		{name: "linux manual", goos: "linux", channel: installChannelManual, want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := upgradeBlockedReason(tt.goos, tt.channel)
			if tt.want == "" {
				if got != "" {
					t.Errorf("upgradeBlockedReason(%q, %q) = %q, want empty string", tt.goos, tt.channel, got)
				}
				return
			}

			if !strings.Contains(got, tt.want) {
				t.Errorf("upgradeBlockedReason(%q, %q) = %q, expected to contain %q", tt.goos, tt.channel, got, tt.want)
			}
		})
	}
}

func TestUpgradeCmdHiddenForPackageManagers(t *testing.T) {
	tests := []struct {
		name    string
		channel installChannel
	}{
		{name: "homebrew", channel: installChannelHomebrew},
		{name: "scoop", channel: installChannelScoop},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newUpgradeCmd("v1.0.0", "darwin", tt.channel)
			if !cmd.Hidden {
				t.Errorf("upgrade command should be hidden for %q installs", tt.channel)
			}
		})
	}
}

func TestUpgradeCmdBlockedRunReturnsActionableMessage(t *testing.T) {
	tests := []struct {
		name    string
		goos    string
		channel installChannel
		want    string
	}{
		{name: "homebrew", goos: "darwin", channel: installChannelHomebrew, want: "brew upgrade --cask gitm"},
		{name: "scoop", goos: "windows", channel: installChannelScoop, want: "scoop update gitm"},
		{name: "windows manual", goos: "windows", channel: installChannelManual, want: "not supported on Windows"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newUpgradeCmd("v1.0.0", tt.goos, tt.channel)
			err := cmd.RunE(cmd, nil)
			if err == nil {
				t.Fatal("expected blocked upgrade to return an error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error = %q, expected to contain %q", err.Error(), tt.want)
			}
		})
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

// fakeSignatureVerifier implements signatureVerifier for testing.
type fakeSignatureVerifier struct {
	called bool
	err    error
}

func (f *fakeSignatureVerifier) Verify(_, _ []byte) error {
	f.called = true
	return f.err
}

// testExecPath creates a fake binary in a temp dir that installBinary can safely
// overwrite without corrupting the test binary.
func testExecPath(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "gitm")
	if err := os.WriteFile(p, []byte("old-binary"), 0755); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestRunUpgradeSignatureVerificationSuccess(t *testing.T) {
	name, err := assetName(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		t.Fatal(err)
	}

	binaryContent := []byte("real binary")
	h := sha256.Sum256(binaryContent)
	checksum := hex.EncodeToString(h[:])
	checksumData := fmt.Sprintf("%s  %s\n", checksum, name)

	uc := &fakeUpgradeClient{
		release: &ghRelease{
			TagName: "v2.0.0",
			Assets: []ghAsset{
				{Name: name, BrowserDownloadURL: "https://example.com/binary"},
				{Name: "checksums.txt", BrowserDownloadURL: "https://example.com/checksums"},
				{Name: "checksums.txt.bundle", BrowserDownloadURL: "https://example.com/bundle"},
			},
		},
		files: map[string][]byte{
			"https://example.com/binary":    binaryContent,
			"https://example.com/checksums": []byte(checksumData),
			"https://example.com/bundle":    []byte("fake-bundle-bytes"),
		},
	}

	sv := &fakeSignatureVerifier{}
	opts := &upgradeOpts{execPath: testExecPath(t)}

	err = runUpgrade("v1.0.0", uc, sv, opts)
	if err != nil {
		t.Fatalf("expected successful upgrade, got: %v", err)
	}
	if !sv.called {
		t.Fatal("expected verifier to be called")
	}
}

func TestRunUpgradeSignatureVerificationFailure(t *testing.T) {
	name, err := assetName(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		t.Fatal(err)
	}

	binaryContent := []byte("real binary")
	h := sha256.Sum256(binaryContent)
	checksum := hex.EncodeToString(h[:])
	checksumData := fmt.Sprintf("%s  %s\n", checksum, name)

	uc := &fakeUpgradeClient{
		release: &ghRelease{
			TagName: "v2.0.0",
			Assets: []ghAsset{
				{Name: name, BrowserDownloadURL: "https://example.com/binary"},
				{Name: "checksums.txt", BrowserDownloadURL: "https://example.com/checksums"},
				{Name: "checksums.txt.bundle", BrowserDownloadURL: "https://example.com/bundle"},
			},
		},
		files: map[string][]byte{
			"https://example.com/binary":    binaryContent,
			"https://example.com/checksums": []byte(checksumData),
			"https://example.com/bundle":    []byte("bad-bundle"),
		},
	}

	sv := &fakeSignatureVerifier{err: fmt.Errorf("invalid signature")}

	err = runUpgrade("v1.0.0", uc, sv, nil)
	if err == nil {
		t.Fatal("expected signature verification error")
	}
	if !strings.Contains(err.Error(), "signature verification failed") {
		t.Errorf("expected 'signature verification failed', got: %v", err)
	}
}

func TestRunUpgradeBundleMissingFallback(t *testing.T) {
	name, err := assetName(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		t.Fatal(err)
	}

	binaryContent := []byte("real binary")
	h := sha256.Sum256(binaryContent)
	checksum := hex.EncodeToString(h[:])
	checksumData := fmt.Sprintf("%s  %s\n", checksum, name)

	uc := &fakeUpgradeClient{
		release: &ghRelease{
			TagName: "v2.0.0",
			Assets: []ghAsset{
				{Name: name, BrowserDownloadURL: "https://example.com/binary"},
				{Name: "checksums.txt", BrowserDownloadURL: "https://example.com/checksums"},
				// No bundle asset — simulates an older release that predates signing
			},
		},
		files: map[string][]byte{
			"https://example.com/binary":    binaryContent,
			"https://example.com/checksums": []byte(checksumData),
		},
	}

	sv := &fakeSignatureVerifier{}
	opts := &upgradeOpts{execPath: testExecPath(t)}

	err = runUpgrade("v1.0.0", uc, sv, opts)
	if err != nil {
		t.Fatalf("expected successful fallback upgrade, got: %v", err)
	}
	if sv.called {
		t.Error("verifier must not be called when bundle is absent")
	}
}

func TestRunUpgradeNilVerifierWithBundleErrors(t *testing.T) {
	name, err := assetName(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		t.Fatal(err)
	}

	binaryContent := []byte("real binary")
	h := sha256.Sum256(binaryContent)
	checksum := hex.EncodeToString(h[:])
	checksumData := fmt.Sprintf("%s  %s\n", checksum, name)

	uc := &fakeUpgradeClient{
		release: &ghRelease{
			TagName: "v2.0.0",
			Assets: []ghAsset{
				{Name: name, BrowserDownloadURL: "https://example.com/binary"},
				{Name: "checksums.txt", BrowserDownloadURL: "https://example.com/checksums"},
				{Name: "checksums.txt.bundle", BrowserDownloadURL: "https://example.com/bundle"},
			},
		},
		files: map[string][]byte{
			"https://example.com/binary":    binaryContent,
			"https://example.com/checksums": []byte(checksumData),
			"https://example.com/bundle":    []byte("bundle-bytes"),
		},
	}

	// nil verifier with bundle present should hard-error (verification is mandatory)
	err = runUpgrade("v1.0.0", uc, nil, nil)
	if err == nil {
		t.Fatal("expected error when bundle present but verifier is nil")
	}
	if !strings.Contains(err.Error(), "no verifier is available") {
		t.Errorf("expected 'no verifier is available' error, got: %v", err)
	}
}

// newTestHTTPClient returns an *http.Client that routes all requests through
// the provided httptest.Server, bypassing DNS and TLS entirely.
func newTestHTTPClient(srv *httptest.Server) *http.Client {
	return srv.Client()
}

func TestHTTPDownloadBytesOversizedRejected(t *testing.T) {
	oversized := bytes.Repeat([]byte("x"), int(maxBundleSize)+1)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write(oversized); err != nil {
			t.Errorf("write oversized payload: %v", err)
		}
	}))
	defer srv.Close()

	uc := &httpUpgradeClient{client: newTestHTTPClient(srv)}

	_, err := uc.downloadBytes(srv.URL + "/bundle")
	if err == nil {
		t.Fatal("expected error for oversized response, got nil")
	}
	if !strings.Contains(err.Error(), "response exceeds maximum allowed size") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestHTTPDownloadBytesExactLimitAccepted(t *testing.T) {
	exactPayload := bytes.Repeat([]byte("y"), int(maxBundleSize))

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write(exactPayload); err != nil {
			t.Errorf("write exact-limit payload: %v", err)
		}
	}))
	defer srv.Close()

	uc := &httpUpgradeClient{client: newTestHTTPClient(srv)}

	got, err := uc.downloadBytes(srv.URL + "/bundle")
	if err != nil {
		t.Fatalf("expected success for exact-limit payload, got: %v", err)
	}
	if len(got) != int(maxBundleSize) {
		t.Errorf("expected %d bytes, got %d", maxBundleSize, len(got))
	}
}

func TestHTTPDownloadBytesNonOKStatusRejected(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()

	uc := &httpUpgradeClient{client: newTestHTTPClient(srv)}

	_, err := uc.downloadBytes(srv.URL + "/bundle")
	if err == nil {
		t.Fatal("expected error for 404 response, got nil")
	}
	if !strings.Contains(err.Error(), "download returned 404") {
		t.Errorf("unexpected error message: %v", err)
	}
}
