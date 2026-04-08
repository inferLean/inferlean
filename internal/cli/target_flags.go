package cli

import "github.com/spf13/cobra"

func bindTargetFlags(cmd *cobra.Command, pid *int32, container, pod, namespace *string, noInteractive *bool) {
	cmd.Flags().Int32Var(pid, "pid", 0, "select a specific vLLM process by pid")
	cmd.Flags().StringVar(container, "container", "", "select a specific vLLM deployment by Docker container id or name")
	cmd.Flags().StringVar(pod, "pod", "", "select a specific vLLM deployment by Kubernetes pod name")
	cmd.Flags().StringVar(namespace, "namespace", "", "Kubernetes namespace for --pod (defaults to default)")
	cmd.Flags().BoolVar(noInteractive, "no-interactive", false, "disable the interactive target selector")
}
