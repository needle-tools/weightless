# weightless

Find local AI model weights across desktop apps, shared caches, and project folders.

`weightless` is for the messy real world where Ollama, LM Studio, Hugging Face, Draw Things, Upscayl, `llama.cpp`, and one-off repos all store weights in different places. It gives you one interactive terminal UI plus a JSON mode for scripting and debugging.

## Highlights

- Scans provider-specific model stores by default, with an optional on-demand `disk-scan` for broader model folders
- Groups raw files into logical models so sharded packages show up as one row
- Shows size, provider, created date, and path
- Lets you drill from Summary into provider-specific Models
- Refreshes in place with `r`
- Emits machine-readable JSON
- Keeps provider detection easy to extend in [internal/providers/registry.go](/Users/herbst/git/temp/llm-finder/internal/providers/registry.go)

## Install

Install script:

```bash
curl -fsSL https://raw.githubusercontent.com/needle-tools/weightless/main/install.sh | bash
```

Specific version:

```bash
curl -fsSL https://raw.githubusercontent.com/needle-tools/weightless/main/install.sh | bash -s -- -s 1.0.2
```

Or download a release archive directly from GitHub Releases.

## Run

```bash
weightless
weightless --json
weightless --version
```

Common flags:

```bash
weightless --providers ollama,lm-studio,huggingface
weightless --roots ~/work/models,/Volumes/FastSSD/models
weightless --min-size-mb 8
```

## TUI

Keys:

- `←` and `→` switch tabs
- `enter` or `space` drills into a provider from Summary
- `o` opens or reveals the selected item
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
- `disk-scan` (lazy, on demand from Summary)

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
      "timestamp": "2026-04-08T09:15:00+02:00",
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
PATH=/opt/homebrew/bin:$PATH go build -o weightless .
```

Run from source:

```bash
go run .
```

Build from source with the installer:

```bash
./install.sh --build-from-source
```

## Release

This repo is set up to publish GitHub Releases directly.

One-time maintainer setup:

1. Create the GitHub repo `needle-tools/weightless`.
2. Push `main`.

Publish a release:

```bash
git push origin main
git push origin v1.0.2
```

That release flow will:

- run CI
- build macOS, Linux, and Windows binaries
- publish GitHub Release assets
- generate checksums

## Changelog

See [CHANGELOG.md](/Users/herbst/git/temp/llm-finder/CHANGELOG.md).
