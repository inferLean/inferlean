package discovery

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

var containerIDPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?:docker|cri-containerd|crio)-([0-9a-f]{64})(?:\.scope)?`),
	regexp.MustCompile(`/(?:docker|cri-containerd|crio)/([0-9a-f]{64})(?:$|/)`),
	regexp.MustCompile(`/([0-9a-f]{64})(?:\.scope|$|/)`),
}

func containerIDForPID(pid int32) (string, error) {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/cgroup", pid))
	if err != nil {
		return "", err
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		for _, pattern := range containerIDPatterns {
			matches := pattern.FindStringSubmatch(line)
			if len(matches) == 2 {
				return matches[1], nil
			}
		}
	}

	return "", nil
}
