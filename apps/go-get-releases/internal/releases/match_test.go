package releases

import (
	"fmt"
	"testing"
	"time"

	"github.com/google/go-github/v89/github"
)

func TestTargetMatching(t *testing.T) {
	cases := []struct {
		name, os, arch string
		match          bool
	}{
		{"App_1.2_Windows_x86_64.zip", "windows", "amd64", true},
		{"App-win-x64.exe", "windows", "amd64", true},
		{"App-win64.msi", "windows", "amd64", true},
		{"App-amd64.msix", "windows", "amd64", true},
		{"App-Windows-arm64.exe", "windows", "amd64", false},
		{"App-Windows-ia32.exe", "windows", "amd64", false},
		{"App-Windows-ia32.exe", "windows", "386", true},
		{"App-linux-aarch64.AppImage", "linux", "arm64", true},
		{"App-aarch64.AppImage", "linux", "arm64", true},
		{"App-x86-64.AppImage", "linux", "amd64", true},
		{"App-armv7.AppImage", "linux", "arm64", false},
		{"App-armv7.AppImage", "linux", "arm", true},
		{"App-x86_64.AppImage.tar.gz", "linux", "amd64", true},
		{"App-macOS-Apple-Silicon.dmg", "darwin", "arm64", true},
		{"App-OSX-Intel.pkg", "darwin", "amd64", true},
		{"App-Darwin-arm64.tar.gz", "darwin", "arm64", true},
		{"App-mac-universal2.zip", "darwin", "arm64", true},
		{"App-mac-universal2.zip", "darwin", "amd64", true},
		{"App_Darwin_all.tar.gz", "darwin", "amd64", true},
		{"App-x64.zip", "windows", "amd64", false},
		{"App.dmg", "darwin", "amd64", false},
		{"App-linux-arm64.deb", "linux", "arm64", true},
		{"App-linux-amd64.rpm", "linux", "amd64", true},
		{"App-linux-amd64", "linux", "amd64", true},
		{"App-v1.2.3-linux-amd64", "linux", "amd64", true},
		{"App-freebsd-amd64.zip", "linux", "amd64", false},
	}
	for _, tc := range cases {
		t.Run(tc.name+tc.arch, func(t *testing.T) {
			target := Target{OS: tc.os, Arch: tc.arch, Formats: formats}
			if got := target.matches(tc.name); got != tc.match {
				t.Errorf("match=%v, want %v", got, tc.match)
			}
		})
	}
	target := Target{OS: "windows", Arch: "amd64", Formats: []string{"exe"}, Match: `(?i)^App-setup\.exe$`, Exclude: `portable`}
	if !target.matches("App-setup.exe") || target.matches("App-portable.exe") || target.matches("App-setup.zip") {
		t.Fatal("explicit rules did not apply")
	}
	target.Match = ""
	target.Exclude = "setup"
	if target.matches("App-win-x64-setup.exe") {
		t.Fatal("exclude did not apply")
	}
}

func releaseFixture(id int64, tag string, published int64) *github.RepositoryRelease {
	return &github.RepositoryRelease{ID: id, TagName: tag, PublishedAt: &github.Timestamp{Time: time.Unix(published, 0)}}
}

func TestStableAndLatest(t *testing.T) {
	for _, marker := range []string{"beta", "beta1", "alpha.2", "rc1", "pre", "preview", "nightly", "dev", "canary", "snapshot", "experimental", "insiders", "unstable"} {
		for _, tag := range []string{"v1.2.3-" + marker, "release/" + marker, "v1.2.3"} {
			r := releaseFixture(1, tag, 1)
			if tag == "v1.2.3" {
				r.Name = github.Ptr("App " + marker)
			}
			if stable(r) {
				t.Errorf("accepted %s (%s)", tag, r.GetName())
			}
		}
	}
	for _, tag := range []string{"v1.0.0", "release/2026.10.12", "build-45", "apps/tool/v2.0.0", "alphabet-1", "device-2"} {
		if !stable(releaseFixture(1, tag, 1)) {
			t.Errorf("rejected %s", tag)
		}
	}
	for _, tag := range []string{"v1.2.3-test.1", "apps/tool/v2.0.0-custom", "release-v2.0.0-next"} {
		if stable(releaseFixture(1, tag, 1)) {
			t.Errorf("accepted semantic prerelease: %s", tag)
		}
	}
	for _, flag := range []string{"draft", "prerelease", "unpublished", "empty tag"} {
		r := releaseFixture(1, "v1", 1)
		switch flag {
		case "draft":
			r.Draft = true
		case "prerelease":
			r.Prerelease = true
		case "unpublished":
			r.PublishedAt = nil
		case "empty tag":
			r.TagName = ""
		}
		if stable(r) {
			t.Errorf("accepted %s", flag)
		}
	}
	input := []*github.RepositoryRelease{nil, releaseFixture(1, "v1", 10), releaseFixture(3, "v3", 30), releaseFixture(5, "v5-beta", 50), releaseFixture(2, "v2", 20), releaseFixture(4, "v3", 30)}
	got := latest(input, 2)
	if len(got) != 2 || got[0].GetID() != 4 || got[1].GetTagName() != "v2" {
		t.Fatal(got)
	}
	if len(latest(input, 10)) != 3 || len(latest(nil, 3)) != 0 {
		t.Fatal("wrong release count")
	}
}

func TestSelectAssets(t *testing.T) {
	c := Defaults()
	a := assetFixture(1, "App-mac-universal.dmg", "body")
	pending := assetFixture(2, "App-Windows-amd64.exe", "body")
	pending.State = github.Ptr("new")
	checks := assetFixture(3, "App-Windows-amd64.exe.sha256", "body")
	selected, missing := selectAssets([]*github.ReleaseAsset{a, pending, checks}, c.Targets)
	if len(selected) != 1 || len(missing) != 3 {
		t.Fatal(fmt.Sprint(selected, missing))
	}
}

func TestPackageFormats(t *testing.T) {
	for _, format := range formats {
		if format == "binary" {
			continue
		}
		if got := packageFormat("app." + format); got != format {
			t.Fatalf("%s: %s", format, got)
		}
	}
	if packageFormat("app.txt") != "" {
		t.Fatal("text file is a package")
	}
}
