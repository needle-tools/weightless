# weightloss

Terminal UI for finding local AI model weights across desktop apps, shared caches, and project folders.

`weightloss` is built for the messy real world where Ollama, LM Studio, Hugging Face, Draw Things, Upscayl, `llama.cpp`, and other tools all stash weights in different places. It gives you one sortable TUI plus a JSON mode for scripting.

## Features

- Scans provider-specific model stores and common project-local weight directories
- Groups raw files into logical models so sharded packages show up as one row
- Shows total size, created date, provider, and path
- Lets you drill from Summary into provider-specific Models
- Supports refresh in-place from the TUI with `r`
- Emits machine-readable JSON for debugging and automation
- Keeps the provider registry easy to extend in [internal/providers/registry.go](/Users/herbst/git/temp/llm-finder/internal/providers/registry.go)

## Quick Start

Install via Homebrew:

```bash
brew install hybridherbst/tap/weightloss
```

Or via install script:

```bash
curl -fsSL https://raw.githubusercontent.com/hybridherbst/weightloss/main/install.sh | bash
```

Install a specific release:

```bash
curl -fsSL https://raw.githubusercontent.com/hybridherbst/weightloss/main/install.sh | bash -s -- -s 1.0.0
```

Run it:

```bash
weightloss
weightloss --json
weightloss --version
```

## Usage

```bash
weightloss
weightloss --json
weightloss --providers ollama,lm-studio,huggingface
weightloss --roots ~/work/models,/Volumes/FastSSD/models
weightloss --min-size-mb 8
```

TUI keys:

- `←` and `→` switch tabs
- `enter` or `space` drills into a provider from Summary
- `o` reveals the selected item in Finder
- `r` refreshes the scan
- `esc` goes back from a drilled view
- `q` quits

## Providers

Current coverage includes:

- `ollama`
- `lm-studio`
- `anythingllm`
- `draw-things`
- `upscayl`
- `huggingface`
- `unsloth-studio`
- `jan`
- `gpt4all`
- `vllm`
- `node-llama-cpp`
- `llama.cpp` shared-cache attribution
- `chrome-built-in-ai`
- `nvidia`
- `text-generation-webui`
- `comfy`
- `stable-diffusion-webui`
- `invokeai`
- `project-local`

## JSON Output

Example shape:

```json
{
  "summary": [
    {
      "provider": "ollama",
      "artifacts": 2,
      "complete_artifacts": 2,
      "incomplete_artifacts": 0,
      "size_bytes": 7630497504,
      "size_human": "7.1 GiB"
    }
  ],
  "artifacts": [
    {
      "name": "qwen3.5:9b",
      "model_name": "qwen3.5:9b",
      "status": "complete",
      "primary_provider": "ollama",
      "path": "/Users/you/.ollama/models/blobs/sha256-...",
      "created_at": "2026-04-08T09:15:00+02:00",
      "file_count": 1,
      "all_paths": [
        "/Users/you/.ollama/models/blobs/sha256-..."
      ]
    }
  ],
  "total_artifacts": 31,
  "total_size_human": "104.2 GiB"
}
```

## Development

Build locally:

```bash
PATH=/opt/homebrew/bin:$PATH go build -o weightloss .
```

Run from source:

```bash
go run .
```

Build from source with the installer:

```bash
./install.sh --build-from-source
```

The provider registry is intentionally simple: most new locations are just an extra `LocationSpec` in [internal/providers/registry.go](/Users/herbst/git/temp/llm-finder/internal/providers/registry.go).

## Maintainer Setup

To publish this like `mole`, do these one-time remote steps:

1. Create the GitHub repo `hybridherbst/weightloss`.
2. Create the Homebrew tap repo `hybridherbst/homebrew-tap`.
3. Add the `HOMEBREW_TAP_GITHUB_TOKEN` GitHub Actions secret on `hybridherbst/weightloss`.
4. Push `main`.
5. Tag a release and push it:

```bash
git tag v1.0.0
git push origin main --tags
```

That release flow will:

- run CI
- build macOS, Linux, and Windows binaries
- publish GitHub Release assets
- generate checksums
- update the Homebrew formula in `hybridherbst/homebrew-tap`

## Changelog

See [CHANGELOG.md](/Users/herbst/git/temp/llm-finder/CHANGELOG.md).
