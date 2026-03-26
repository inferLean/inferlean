# InferLean

InferLean is the optimization copilot for self-hosted LLM inference.

It helps teams running production vLLM deployments answer three questions:

1. What is holding this deployment back right now?
2. What should we change next?
3. How much practical headroom likely remains before more hardware is needed?

Phase 1 is the installable discovery slice. It does not yet run the full collect/analyze/report workflow. This release focuses on reliably finding a local vLLM deployment, parsing its runtime configuration, and explaining what InferLean selected.

## Install

Unix-like systems:

```bash
curl -fsSL https://raw.githubusercontent.com/inferLean/inferlean/main/scripts/install.sh | bash
```

Windows PowerShell:

```powershell
irm https://raw.githubusercontent.com/inferLean/inferlean/main/scripts/install.ps1 | iex
```

The Unix installer defaults to `~/.local/bin`. The PowerShell installer defaults to `%LOCALAPPDATA%\InferLean\bin`.

## Use

Discover a local vLLM deployment:

```bash
inferlean discover
```

Select a specific process explicitly:

```bash
inferlean discover --pid 12345
```

Disable the interactive chooser in scripts or CI:

```bash
inferlean discover --no-interactive
```

Enable global debug output:

```bash
inferlean --debug discover
```

## What Phase 1 Does

- Finds current `vllm serve` processes and legacy vLLM API-server entrypoints.
- Groups related worker processes into one logical deployment.
- Parses runtime settings such as model, host, port, parallelism, token limits, quantization, and selected safe environment hints.
- Explains what InferLean selected and what was ambiguous or missing.

## What Phase 1 Does Not Do Yet

- Collect metrics
- Upload artifacts
- Generate recommendations
- Estimate headroom
- Render the full report UI

Those come in later delivery phases. Phase 1 is the first step toward the full “run a scan” workflow.

## Release Policy

- Linux release archives include the CLI plus Prometheus, node exporter, and DCGM payloads for Phase 1.
- Windows and macOS release archives are CLI-only in Phase 1.
- GitHub Actions publishes a semantic-version release for each commit that lands on `main`.
