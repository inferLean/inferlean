# InferLean CLI

InferLean is a CLI-first optimizer for self-hosted LLM inference. The CLI discovers a running vLLM target, collects local evidence, uploads one artifact to the InferLean backend, and renders the backend report.

The CLI is intentionally thin: it collects configurations, raw observations, and explicit workload intent. Optimization decisions, entitlement, diagnosis, and canonical report shaping live in the backend.

## Install

Install the latest release:

```sh
curl -fsSL https://raw.githubusercontent.com/inferLean/inferlean/main/scripts/install.sh | bash
```

By default the installer writes `inferlean` to `~/.local/bin`. Override that with:

```sh
curl -fsSL https://raw.githubusercontent.com/inferLean/inferlean/main/scripts/install.sh \
  | INFERLEAN_INSTALL_DIR=/usr/local/bin bash
```

Install a specific release:

```sh
curl -fsSL https://raw.githubusercontent.com/inferLean/inferlean/main/scripts/install.sh \
  | INFERLEAN_VERSION=v0.1.0 bash
```

Verify the install:

```sh
inferlean version
```

## Quick Start

Start from a host where your vLLM server is already running:

```sh
inferlean run
```

`inferlean run` performs:

1. discovers a vLLM target
2. collects local configuration and metrics evidence
3. asks for missing workload intent when interactive
4. writes `artifact.json`
5. uploads the artifact
6. renders the backend report

For scripted use, provide the target and required intent up front:

```sh
inferlean run \
  --pid 12345 \
  --no-interactive \
  --workload-mode online \
  --workload-target latency \
  --prefix-heavy false \
  --multimodal false \
  --repeated-multimodal-media false
```

## Commands

### `inferlean discover`

Find running vLLM targets through local processes, Docker containers, and Kubernetes pods.

```sh
inferlean discover
inferlean discover --pid 12345
inferlean discover --container vllm-server
inferlean discover --pod vllm-server --namespace inference
```

Useful target flags:

- `--pid`: target process ID
- `--container`: Docker container name
- `--pod`: Kubernetes pod name
- `--namespace`: Kubernetes namespace
- `--no-interactive`: disable interactive target selection
- `--exclude-processes`: skip process discovery
- `--exclude-docker`: skip Docker discovery
- `--exclude-kubernetes`: skip Kubernetes discovery

### `inferlean collect`

Collect local evidence and write one run artifact.

```sh
inferlean collect --pid 12345
inferlean collect --collect-for 2m --scrape-every 10s
inferlean collect --output ./artifact.json
```

Default output path:

```text
~/.inferlean/runs/<run_id>/artifact.json
```

Collection flags:

- `--collect-for`: metrics collection duration, default `30s`
- `--scrape-every`: metrics scrape interval, default `5s`
- `--output`: artifact output path
- `--workload-mode`: declared workload mode
- `--workload-target`: declared optimization target
- `--prefix-heavy`: `true`, `false`, or `auto`
- `--multimodal`: `true`, `false`, or `auto`
- `--repeated-multimodal-media`: `true`, `false`, or `auto`

`collect` also accepts the target discovery flags from `discover`.

### `inferlean upload`

Upload an existing artifact and render the returned report:

```sh
inferlean upload ~/.inferlean/runs/<run_id>/artifact.json
```

Render a previously uploaded report by run ID:

```sh
inferlean upload --run-id <run_id>
```

Upload flags:

- `--backend-url`: backend base URL, default `https://app.inferlean.com`
- `--require-report`: fail if report retrieval after upload fails
- `--run-id`: load and render an existing report
- `--no-interactive`: disable interactive report prompts and viewer

### `inferlean run`

Run the full workflow in one command:

```sh
inferlean run
inferlean run --container vllm-server --collect-for 90s
inferlean run --require-upload
```

`run` accepts target discovery flags, collection flags, workload intent flags, and:

- `--backend-url`: backend base URL, default `https://app.inferlean.com`
- `--require-upload`: fail the command when upload or report retrieval fails

Without `--require-upload`, the command still preserves the local artifact if upload/report retrieval fails.

### `inferlean login`

Authenticate through the browser OIDC flow:

```sh
inferlean login
```

Use another backend:

```sh
inferlean login --backend-url http://localhost:8080
```

### `inferlean logout`

Clear the local auth session:

```sh
inferlean logout
```

## Evidence Collected

The CLI writes a single artifact with:

- `job`: run ID, installation ID, schema version, collector version, timestamps
- `target`: selected vLLM process, command line, metrics endpoint, container or pod identity when present
- `configurations`: vLLM args, selected environment, OS, CPU, RAM, GPU, CUDA/runtime, and `nvidia-smi` facts
- `observations`: raw metric samples from vLLM, host, GPU, and local Prometheus collection when available
- `raw_process_io`: raw command or helper output such as `nvidia-smi`
- `user_intent`: explicit workload intent values
- `collection_quality`: source status, fallbacks, missing/degraded sources, duration, and scrape interval

The CLI does not collect prompt content or request payloads.

## Local Files

InferLean stores local state under `~/.inferlean`:

```text
~/.inferlean/config
~/.inferlean/runs/<run_id>/artifact.json
~/.inferlean/runs/<run_id>/report.json
~/.inferlean/runs/<run_id>/observations/
~/.inferlean/runs/<run_id>/process-io/
~/.inferlean/tools/
```

`~/.inferlean/config` contains the installation ID and auth tokens. Run artifacts and reports are written with user-only file permissions.

## Optional Local Tools

The collector can use local Prometheus-compatible tooling when available:

- `prometheus`
- `node_exporter`
- `dcgm-exporter`
- `nvidia-smi`

The CLI resolves tools from `~/.inferlean/tools` first, then `PATH`. Set `INFERLEAN_TOOLS_DIR` to point at a different tool directory.

Package Prometheus and Node Exporter into the tool directory from a source checkout:

```sh
./scripts/package-linux-tools.sh
```

## Air-Gapped Environments

InferLean can collect evidence without outbound network access. In an air-gapped environment, run collection locally, preserve the generated artifact, and upload it later from a connected machine or to an internally reachable InferLean backend.

Install the CLI by copying a release binary into the environment:

```sh
install -m 0755 inferlean ~/.local/bin/inferlean
inferlean version
```

If helper tools are not already installed on the target host, stage them under `~/.inferlean/tools` or point the CLI at another directory:

```sh
mkdir -p ~/.inferlean/tools
cp prometheus node_exporter ~/.inferlean/tools/
INFERLEAN_TOOLS_DIR=/opt/inferlean-tools inferlean collect --pid 12345
```

Collect without upload:

```sh
inferlean collect \
  --pid 12345 \
  --no-interactive \
  --workload-mode online \
  --workload-target latency \
  --prefix-heavy false \
  --multimodal false \
  --repeated-multimodal-media false
```

The artifact is written to:

```text
~/.inferlean/runs/<run_id>/artifact.json
```

Move that artifact to a connected environment and upload it:

```sh
inferlean upload ./artifact.json
```

If your organization runs an internal InferLean backend, use its URL directly:

```sh
inferlean login --backend-url https://inferlean.internal
inferlean upload ./artifact.json --backend-url https://inferlean.internal
```

For fully offline validation, keep the whole run directory so raw observations and process output stay available with the artifact:

```text
~/.inferlean/runs/<run_id>/
```

## Development

Build from source:

```sh
go build ./cmd/inferlean
```

Run tests:

```sh
go test ./...
```

Run the CLI from source:

```sh
go run ./cmd/inferlean --help
```

Useful debug flags:

- `--debug`: print debug output
- `--debug-file <path>`: write debug output to a file

## Architecture

The CLI follows a model-view-presenter split:

- model: discovery, collection, artifact building, storage, auth, and API calls
- view: terminal rendering and prompts
- presenter: flow orchestration, validation, progress state, and view model mapping

Shared artifact and report contracts live in `pkg/contracts`. Backend-owned optimization logic must not be duplicated in the CLI.
