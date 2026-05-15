package cli

import (
	"context"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func TestRootCommandSupportsBackendURLFlag(t *testing.T) {
	cmd := newRootCommand(context.Background())

	if err := cmd.ParseFlags([]string{"--backend-url", "http://127.0.0.1:8080"}); err != nil {
		t.Fatalf("parse backend-url: %v", err)
	}
	got, err := cmd.Flags().GetString("backend-url")
	if err != nil {
		t.Fatalf("get backend-url: %v", err)
	}
	if got != "http://127.0.0.1:8080" {
		t.Fatalf("backend-url = %q, want local backend url", got)
	}
}

func TestRootCommandKeepsDeprecatedAppURLAlias(t *testing.T) {
	cmd := newRootCommand(context.Background())

	if err := cmd.ParseFlags([]string{"--app-url", "http://127.0.0.1:8080"}); err != nil {
		t.Fatalf("parse app-url: %v", err)
	}
	got, err := cmd.Flags().GetString("backend-url")
	if err != nil {
		t.Fatalf("get backend-url after app-url parse: %v", err)
	}
	if got != "http://127.0.0.1:8080" {
		t.Fatalf("backend-url via app-url alias = %q, want local backend url", got)
	}

	flag := cmd.Flags().Lookup("app-url")
	if flag == nil {
		t.Fatal("expected deprecated app-url flag to exist")
	}
	if flag.Deprecated == "" {
		t.Fatal("expected app-url flag to be marked deprecated")
	}
}

func TestRootCommandUsesRunHelpAndFlags(t *testing.T) {
	root := newRootCommand(context.Background())
	run := newRunCommand()

	if root.Short != run.Short {
		t.Fatalf("root short = %q, want run short %q", root.Short, run.Short)
	}
	run.Flags().VisitAll(func(flag *pflag.Flag) {
		if root.Flags().Lookup(flag.Name) == nil {
			t.Fatalf("root command missing run flag --%s", flag.Name)
		}
	})
}

func TestRootCommandParsesRunFlags(t *testing.T) {
	cmd := newRootCommand(context.Background())
	args := []string{
		"--pid", "123",
		"--container", "vllm",
		"--pod", "vllm-pod",
		"--namespace", "llm",
		"--exclude-processes",
		"--exclude-docker",
		"--exclude-kubernetes",
		"--collect-for", "2m",
		"--scrape-every", "5s",
		"--output", "artifact.json",
		"--dcgm-endpoint", "http://127.0.0.1:9400/metrics",
		"--no-dcgm-use-estimation",
		"--workload-mode", "chat",
		"--workload-target", "latency",
		"--prefix-heavy", "true",
		"--multimodal", "false",
		"--repeated-multimodal-media", "false",
		"--require-upload",
	}

	if err := cmd.ParseFlags(args); err != nil {
		t.Fatalf("parse root run flags: %v", err)
	}
	assertInt32Flag(t, cmd, "pid", 123)
	assertStringFlag(t, cmd, "container", "vllm")
	assertStringFlag(t, cmd, "pod", "vllm-pod")
	assertStringFlag(t, cmd, "namespace", "llm")
	assertBoolFlag(t, cmd, "exclude-processes", true)
	assertBoolFlag(t, cmd, "exclude-docker", true)
	assertBoolFlag(t, cmd, "exclude-kubernetes", true)
	assertDurationFlag(t, cmd, "collect-for", 2*time.Minute)
	assertDurationFlag(t, cmd, "scrape-every", 5*time.Second)
	assertStringFlag(t, cmd, "output", "artifact.json")
	assertStringFlag(t, cmd, "dcgm-endpoint", "http://127.0.0.1:9400/metrics")
	assertBoolFlag(t, cmd, "no-dcgm-use-estimation", true)
	assertStringFlag(t, cmd, "workload-mode", "chat")
	assertStringFlag(t, cmd, "workload-target", "latency")
	assertStringFlag(t, cmd, "prefix-heavy", "true")
	assertStringFlag(t, cmd, "multimodal", "false")
	assertStringFlag(t, cmd, "repeated-multimodal-media", "false")
	assertBoolFlag(t, cmd, "require-upload", true)
}

func assertStringFlag(t *testing.T, cmd *cobra.Command, name, want string) {
	t.Helper()
	got, err := cmd.Flags().GetString(name)
	if err != nil {
		t.Fatalf("get %s: %v", name, err)
	}
	if got != want {
		t.Fatalf("%s = %q, want %q", name, got, want)
	}
}

func assertBoolFlag(t *testing.T, cmd *cobra.Command, name string, want bool) {
	t.Helper()
	got, err := cmd.Flags().GetBool(name)
	if err != nil {
		t.Fatalf("get %s: %v", name, err)
	}
	if got != want {
		t.Fatalf("%s = %t, want %t", name, got, want)
	}
}

func assertDurationFlag(t *testing.T, cmd *cobra.Command, name string, want time.Duration) {
	t.Helper()
	got, err := cmd.Flags().GetDuration(name)
	if err != nil {
		t.Fatalf("get %s: %v", name, err)
	}
	if got != want {
		t.Fatalf("%s = %s, want %s", name, got, want)
	}
}

func assertInt32Flag(t *testing.T, cmd *cobra.Command, name string, want int32) {
	t.Helper()
	got, err := cmd.Flags().GetInt32(name)
	if err != nil {
		t.Fatalf("get %s: %v", name, err)
	}
	if got != want {
		t.Fatalf("%s = %d, want %d", name, got, want)
	}
}
