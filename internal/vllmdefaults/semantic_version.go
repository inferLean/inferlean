package vllmdefaults

import (
	"regexp"
	"strconv"
	"strings"
)

var versionPattern = regexp.MustCompile(`(?i)^v?(\d+)\.(\d+)(?:\.(\d+))?(?:(rc|post)(\d+))?$`)

func parseSemanticVersion(raw string) (semanticVersion, bool) {
	matches := versionPattern.FindStringSubmatch(strings.TrimSpace(raw))
	if matches == nil {
		return semanticVersion{}, false
	}
	major, _ := strconv.Atoi(matches[1])
	minor, _ := strconv.Atoi(matches[2])
	patch := 0
	if matches[3] != "" {
		patch, _ = strconv.Atoi(matches[3])
	}
	version := semanticVersion{
		Major:      major,
		Minor:      minor,
		Patch:      patch,
		PreRelease: strings.ToLower(matches[4]),
	}
	if matches[5] != "" {
		version.PreReleaseN, _ = strconv.Atoi(matches[5])
	}
	return version, true
}

func compareSemanticVersion(left, right semanticVersion) int {
	switch {
	case left.Major != right.Major:
		return compareInts(left.Major, right.Major)
	case left.Minor != right.Minor:
		return compareInts(left.Minor, right.Minor)
	case left.Patch != right.Patch:
		return compareInts(left.Patch, right.Patch)
	}
	leftRank := prereleaseRank(left.PreRelease)
	rightRank := prereleaseRank(right.PreRelease)
	if leftRank != rightRank {
		return compareInts(leftRank, rightRank)
	}
	return compareInts(left.PreReleaseN, right.PreReleaseN)
}

func prereleaseRank(kind string) int {
	switch strings.ToLower(kind) {
	case "rc":
		return 0
	case "":
		return 1
	case "post":
		return 2
	default:
		return 0
	}
}

func compareInts(left, right int) int {
	switch {
	case left < right:
		return -1
	case left > right:
		return 1
	default:
		return 0
	}
}
