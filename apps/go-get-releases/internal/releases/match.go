package releases

import (
	"regexp"
	"slices"
	"strings"

	"github.com/Masterminds/semver/v3"
)

var supportedFormats = map[string]bool{}
var formats = []string{"appimage.tar.gz", "appimage.zst", "tar.gz", "tar.xz", "tar.bz2", "tar.zst", "appimage", "msixbundle", "msix", "exe", "msi", "dmg", "pkg", "zip", "7z", "deb", "rpm", "tgz", "tbz2", "txz", "gz", "xz", "zst", "bin", "binary"}

func init() {
	for _, f := range formats {
		supportedFormats[f] = true
	}
}

func normalizeOS(s string) string {
	switch strings.ToLower(s) {
	case "win", "win32", "win64", "windows":
		return "windows"
	case "mac", "macos", "osx", "darwin", "macosx":
		return "darwin"
	case "linux":
		return "linux"
	default:
		return strings.ToLower(s)
	}
}

func normalizeArch(s string) string {
	switch strings.ToLower(s) {
	case "x64", "x86_64", "x86-64", "amd64", "intel", "win64":
		return "amd64"
	case "aarch64", "arm64", "arm64-v8a", "apple-silicon", "applesilicon":
		return "arm64"
	case "x86", "i386", "i686", "386", "ia32":
		return "386"
	case "arm", "armv7", "armv7l", "armhf":
		return "arm"
	default:
		return strings.ToLower(s)
	}
}

func packageFormat(name string) string {
	name = strings.ToLower(name)
	for _, f := range formats {
		if f != "binary" && strings.HasSuffix(name, "."+f) {
			return f
		}
	}
	if !strings.Contains(name, ".") || binarySuffix.MatchString(name) {
		return "binary"
	}
	return ""
}

var binarySuffix = regexp.MustCompile(`(?:^|[-_])(?:amd64|x86_64|x86-64|x64|arm64|aarch64|i386|i686|386|armv7|armhf)$`)

var separators = regexp.MustCompile(`[^a-z0-9]+`)
var unstable = regexp.MustCompile(`(?i)(?:^|[^a-z])(?:alpha|beta|rc|pre|prerelease|preview|nightly|dev|development|canary|snapshot|experimental|insider|insiders|unstable)(?:[0-9]|[^a-z]|$)`)

var versionSuffix = regexp.MustCompile(`(?:^|[/_\s-])v?([0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?)$`)

func stable(r *Release) bool {
	for _, text := range []string{r.GetTagName(), r.GetName()} {
		if m := versionSuffix.FindStringSubmatch(text); m != nil {
			if version, err := semver.StrictNewVersion(m[1]); err == nil && version.Prerelease() != "" {
				return false
			}
		}
	}
	return !r.GetDraft() && !r.GetPrerelease() && r.PublishedAt != nil && r.GetTagName() != "" && !unstable.MatchString(r.GetTagName()) && !unstable.MatchString(r.GetName())
}

// inferPlatform uses token boundaries so ARM64 and x86 never match AMD64.
func inferPlatform(name string) (string, []string) {
	name = strings.ToLower(name)
	// Preserve architecture aliases before tokenization.
	for _, alias := range []string{"x86_64", "x86-64"} {
		name = strings.ReplaceAll(name, alias, "amd64")
	}
	name = strings.ReplaceAll(name, "apple-silicon", "applesilicon")
	tokens := separators.Split(name, -1)
	osName := ""
	archs := []string{}
	universal := false
	for _, token := range tokens {
		o := normalizeOS(token)
		if o == "windows" || o == "darwin" || o == "linux" {
			osName = o
		}
		a := normalizeArch(token)
		if slices.Contains([]string{"amd64", "arm64", "386", "arm"}, a) && !slices.Contains(archs, a) {
			archs = append(archs, a)
		}
		if token == "universal" || token == "universal2" || token == "all" {
			universal = true
		}
	}
	f := packageFormat(name)
	if osName == "" {
		switch f {
		case "appimage", "appimage.tar.gz", "appimage.zst", "deb", "rpm":
			osName = "linux"
		case "exe", "msi", "msix", "msixbundle":
			osName = "windows"
		case "dmg", "pkg":
			osName = "darwin"
		}
	}
	if universal && osName == "darwin" {
		archs = []string{"amd64", "arm64"}
	}
	return osName, archs
}

func (t Target) matches(name string) bool {
	if !slices.Contains(t.Formats, packageFormat(name)) {
		return false
	}
	if t.Exclude != "" && regexp.MustCompile(t.Exclude).MatchString(name) {
		return false
	}
	if t.Match != "" {
		return regexp.MustCompile(t.Match).MatchString(name)
	}
	osName, archs := inferPlatform(name)
	return osName == t.OS && slices.Contains(archs, t.Arch)
}

func selectAssets(assets []*Asset, targets []Target) ([]*Asset, []Target) {
	selected := []*Asset{}
	found := make([]bool, len(targets))
	for _, a := range assets {
		if a.GetState() != "uploaded" || checksumAsset(a.GetName()) {
			continue
		}
		matched := false
		for i, t := range targets {
			if t.matches(a.GetName()) {
				found[i] = true
				matched = true
			}
		}
		if matched {
			selected = append(selected, a)
		}
	}
	missing := []Target{}
	for i, ok := range found {
		if !ok {
			missing = append(missing, targets[i])
		}
	}
	return selected, missing
}
