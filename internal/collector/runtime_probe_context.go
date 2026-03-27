package collector

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/inferLean/inferlean/internal/discovery"
)

type runtimeProbeContext struct {
	ResolvedExecutable string            `json:"resolved_executable,omitempty"`
	WorkingDirectory   string            `json:"working_directory,omitempty"`
	Environment        map[string]string `json:"environment,omitempty"`
	SitePackages       []string          `json:"site_packages,omitempty"`
}

func inspectRuntimeContext(target discovery.CandidateGroup) runtimeProbeContext {
	ctx := runtimeProbeContext{
		ResolvedExecutable: target.Executable,
		Environment:        map[string]string{},
	}
	if target.PrimaryPID <= 0 {
		ctx.SitePackages = candidateSitePackages(ctx.ResolvedExecutable, ctx.WorkingDirectory, ctx.Environment)
		return ctx
	}

	pid := strconv.Itoa(int(target.PrimaryPID))
	ctx.ResolvedExecutable = firstNonEmpty(procLink(pid, "exe"), ctx.ResolvedExecutable)
	ctx.WorkingDirectory = procLink(pid, "cwd")
	ctx.Environment = readProbeEnvironment(pid)
	ctx.SitePackages = candidateSitePackages(ctx.ResolvedExecutable, ctx.WorkingDirectory, ctx.Environment)
	return ctx
}

func procLink(pid, name string) string {
	link, err := os.Readlink(filepath.Join("/proc", pid, name))
	if err != nil {
		return ""
	}
	return link
}

func readProbeEnvironment(pid string) map[string]string {
	data, err := os.ReadFile(filepath.Join("/proc", pid, "environ"))
	if err != nil {
		return map[string]string{}
	}
	values := map[string]string{}
	for _, entry := range strings.Split(string(data), "\x00") {
		key, value, ok := strings.Cut(entry, "=")
		if ok && shouldKeepProbeEnv(key) {
			values[key] = value
		}
	}
	return values
}

func shouldKeepProbeEnv(key string) bool {
	switch key {
	case "CONDA_PREFIX", "LD_LIBRARY_PATH", "PATH", "PYTHONHOME", "PYTHONPATH", "VIRTUAL_ENV", "VLLM_ATTENTION_BACKEND":
		return true
	default:
		return false
	}
}

func candidateSitePackages(executable, cwd string, env map[string]string) []string {
	candidates := []string{}
	for _, root := range candidateRoots(executable, cwd, env) {
		addCandidate(&candidates, filepath.Join(root, "Lib", "site-packages"))
		addCandidate(&candidates, filepath.Join(root, "lib", "site-packages"))
		addCandidate(&candidates, filepath.Join(root, "lib64", "site-packages"))
		addGlobCandidates(&candidates, filepath.Join(root, "lib", "python*", "site-packages"))
		addGlobCandidates(&candidates, filepath.Join(root, "lib64", "python*", "site-packages"))
	}
	return candidates
}

func candidateRoots(executable, cwd string, env map[string]string) []string {
	roots := []string{}
	for _, root := range []string{
		filepath.Dir(filepath.Dir(executable)),
		env["VIRTUAL_ENV"],
		env["CONDA_PREFIX"],
		cwd,
	} {
		addCandidate(&roots, root)
	}
	return roots
}

func addGlobCandidates(candidates *[]string, pattern string) {
	matches, _ := filepath.Glob(pattern)
	for _, match := range matches {
		addCandidate(candidates, match)
	}
}

func addCandidate(candidates *[]string, value string) {
	value = strings.TrimSpace(value)
	if value == "" || stringSliceContains(*candidates, value) {
		return
	}
	*candidates = append(*candidates, value)
}

func stringSliceContains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
