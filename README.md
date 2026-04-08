# InferLean

InferLean is the optimization copilot for self-hosted LLM inference.

It helps teams running production vLLM deployments answer three questions:

1. What is holding this deployment back right now?
2. What should we change next?
3. How much practical headroom likely remains before more hardware is needed?

InferLean now supports the phase-9 product loop in the CLI: authenticate, discover, collect, publish, fetch the canonical report, and render it locally in an interactive terminal UI.

## Install

Unix-like systems:

```bash
curl -fsSL https://raw.githubusercontent.com/inferLean/inferlean/main/scripts/install.sh | bash
```

On Linux NVIDIA hosts, the Unix installer makes a best-effort attempt to install the DCGM runtime when `libdcgm` is missing, then build `dcgm-exporter` from the pinned NVIDIA source tag and copy the resulting binary into the local tool bundle for `collect`. Automatic DCGM package installation is only supported on apt-based `x86_64` systems and may require `sudo`; the exporter build also needs `git`, `make`, and a C toolchain such as `build-essential`/`gcc`, and the installer can bootstrap a local Go 1.24.13 toolchain when the system Go is older.

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

Select a Docker-managed deployment explicitly:

```bash
inferlean discover --container vllm-server
```

Select a Kubernetes-managed deployment explicitly:

```bash
inferlean discover --pod vllm-0 --namespace inference
```

Disable the interactive chooser in scripts or CI:

```bash
inferlean discover --no-interactive
```

Enable global debug output:

```bash
inferlean --debug discover
```

Write debug output to a specific file:

```bash
inferlean --debug-file /tmp/inferlean-debug.log discover
```

Run the end-to-end product flow on Linux:

```bash
inferlean scan
```

`scan` is the primary product entrypoint. It authenticates when needed, collects locally, uploads the artifact, fetches the canonical report, and opens the terminal report UI.

Open a previously claimed report:

```bash
inferlean runs
```

Build a local artifact without opening the report flow:

```bash
inferlean collect
```

`collect` remains the lower-level artifact command. It is Linux-only and uses a 30-second default collection window with a 5-second default scrape cadence.

Collect for longer or change the scrape cadence:

```bash
inferlean collect --collect-for 30s --scrape-every 5s
```

Select a specific process explicitly:

```bash
inferlean collect --pid 12345
```

Select a Docker-managed deployment explicitly:

```bash
inferlean collect --container vllm-server
```

Select a Kubernetes-managed deployment explicitly:

```bash
inferlean collect --pod vllm-0 --namespace inference
```

Disable the interactive chooser in scripts or CI:

```bash
inferlean collect --no-interactive
```

Write the artifact to a custom location:

```bash
inferlean collect --output /tmp/artifact.json
```

## What The CLI Does

- Finds current `vllm serve` processes and legacy vLLM API-server entrypoints.
- Enriches discovered local vLLM targets with Docker container and Kubernetes pod metadata when available.
- Groups related worker processes into one logical deployment.
- Parses runtime settings such as model, host, port, parallelism, token limits, quantization, and selected safe environment hints.
- Collects local evidence from a supported Linux host.
- Writes a validated run artifact to `~/.inferlean/runs/<run_id>/artifact.json` unless `--output` is provided.
- Uploads the artifact to the backend with the saved InferLean login session.
- Fetches the canonical report for the run and renders it locally in the terminal.
- Lets the user switch between brief/full report depth and latency/balanced/throughput overlays without another backend call.
- Reopens previously claimed reports with `inferlean runs`.

## Release Policy

- Linux release archives include the CLI plus Prometheus and node exporter payloads for collection.
- Linux release archives no longer bundle the DCGM exporter source tree by default. On supported Linux NVIDIA hosts, the installer can build `dcgm-exporter` from the pinned NVIDIA tag and stage the resulting binary under the local tool bundle for `collect`.
- Required rich GPU telemetry no longer depends on bundled DCGM being runnable; NVML is the default local completeness path and `nvidia-smi` remains an additional evidence source.
- Windows and macOS release archives remain CLI-only.
- GitHub Actions publishes a semantic-version release for each commit that lands on `main`.
