// Package version provides semver comparison and reference classification utilities
// for GitHub Action version strings.
package version

import (
	"math"
	"regexp"
	"strings"

	"github.com/Masterminds/semver/v3"
)

func IsVersionOutdated(currentRef, latestTag string) (bool, error) {
	currentRef = strings.TrimSpace(currentRef)
	latestTag = strings.TrimSpace(latestTag)

	if currentRef == latestTag {
		return false, nil
	}

	if IsCommitSHA(currentRef) {
		return false, nil
	}

	if IsBranchName(currentRef) {
		return false, nil
	}

	currentSemver, err := parseSemver(currentRef)
	if err != nil {
		return false, err
	}

	latestSemver, err := parseSemver(latestTag)
	if err != nil {
		return false, err
	}

	return currentSemver.LessThan(latestSemver), nil
}

func IsMajorVersionTag(ref string) bool {
	ref = strings.TrimSpace(ref)
	ref = strings.TrimPrefix(ref, "v")
	return regexp.MustCompile(`^\d+$`).MatchString(ref)
}

func GetMajorVersion(version string) int64 {
	version = strings.TrimSpace(version)
	version = strings.TrimPrefix(version, "v")

	v, err := semver.NewVersion(version)
	if err != nil {
		return -1
	}

	major := v.Major()
	if major > math.MaxInt64 {
		return -1
	}
	return int64(major)
}

func SameMajorVersion(v1, v2 string) bool {
	major1 := GetMajorVersion(v1)
	major2 := GetMajorVersion(v2)

	if major1 < 0 || major2 < 0 {
		return false
	}

	return major1 == major2
}

func parseSemver(version string) (*semver.Version, error) {
	version = strings.TrimPrefix(version, "v")

	if regexp.MustCompile(`^\d+$`).MatchString(version) {
		version += ".0.0"
	}

	if regexp.MustCompile(`^\d+\.\d+$`).MatchString(version) {
		version += ".0"
	}

	return semver.NewVersion(version)
}

func IsCommitSHA(s string) bool {
	matched, _ := regexp.MatchString(`^[a-fA-F0-9]{7,40}$`, s)
	return matched
}

func IsBranchName(s string) bool {
	commonBranches := []string{"main", "master", "develop", "dev", "staging", "production", "prod"}
	sLower := strings.ToLower(s)
	for _, branch := range commonBranches {
		if sLower == branch {
			return true
		}
	}
	return false
}
