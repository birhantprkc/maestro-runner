package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// writeTestBundle creates a .app directory carrying an Info.plist with the
// given version keys, omitting a key entirely when its value is empty.
func writeTestBundle(t *testing.T, shortVersion, bundleVersion string) string {
	t.Helper()
	appDir := filepath.Join(t.TempDir(), "Example.app")
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		t.Fatal(err)
	}

	entries := ""
	if shortVersion != "" {
		entries += "\t<key>CFBundleShortVersionString</key>\n\t<string>" + shortVersion + "</string>\n"
	}
	if bundleVersion != "" {
		entries += "\t<key>CFBundleVersion</key>\n\t<string>" + bundleVersion + "</string>\n"
	}
	plist := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleIdentifier</key>
	<string>com.example.app</string>
` + entries + `</dict>
</plist>
`
	if err := os.WriteFile(filepath.Join(appDir, "Info.plist"), []byte(plist), 0o644); err != nil {
		t.Fatal(err)
	}
	return appDir
}

// TestReadBundleVersionAndBuild covers the path that gives a physical iOS
// device any version at all. simctl can only reach an installed simulator app,
// so on real hardware the keys come from the .app under test — without this a
// real-device report shows no version, which is the gap behind #144 for anyone
// not on a simulator.
func TestReadBundleVersionAndBuild(t *testing.T) {
	if _, err := os.Stat("/usr/libexec/PlistBuddy"); err != nil {
		t.Skip("PlistBuddy not available")
	}

	tests := []struct {
		name          string
		shortVersion  string
		bundleVersion string
		wantVersion   string
		wantBuild     string
	}{
		{"both keys", "1.16.0", "10009107", "1.16.0", "10009107"},
		{"marketing version only", "1.16.0", "", "1.16.0", ""},
		{"build only", "", "10009107", "", "10009107"},
		{"neither", "", "", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			appDir := writeTestBundle(t, tt.shortVersion, tt.bundleVersion)

			version, build := readBundleVersionAndBuild(appDir)
			if version != tt.wantVersion {
				t.Errorf("version = %q, want %q", version, tt.wantVersion)
			}
			if build != tt.wantBuild {
				t.Errorf("build = %q, want %q", build, tt.wantBuild)
			}
		})
	}
}

// A missing bundle must read as unknown rather than failing the run — a report
// without a version is worth having; a run aborted over one is not.
func TestReadBundleVersionAndBuildMissingBundle(t *testing.T) {
	if _, err := exec.LookPath("/usr/libexec/PlistBuddy"); err != nil {
		t.Skip("PlistBuddy not available")
	}

	version, build := readBundleVersionAndBuild(filepath.Join(t.TempDir(), "Nope.app"))
	if version != "" || build != "" {
		t.Errorf("expected empty results for a missing bundle, got %q / %q", version, build)
	}
}
