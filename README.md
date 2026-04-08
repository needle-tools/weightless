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

## Quick Start

Install with the included script:

```bash
cd /path/to/weightloss
./install.sh
```

By default this installs `weightloss` to `~/.local/bin`.

Run it:

```bash
weightloss
```

JSON mode:

```bash
weightloss --json
```

Show the installed version:

```bash
weightloss --version
```

## Install Options

Install to a custom prefix:

```bash
./install.sh --prefix /usr/local/bin
```

Build manually without the installer:

```bash
PATH=/opt/homebrew/bin:$PATH go build -o weightloss .
mv ./weightloss ~/.local/bin/weightloss
```

Remove it:

```bash
rm -f ~/.local/bin/weightloss
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

Run from source:

```bash
go run .
```

Build a local binary:

```bash
PATH=/opt/homebrew/bin:$PATH go build -o weightloss .
```

The provider registry is intentionally simple: most new locations are just an extra `LocationSpec` in [internal/providers/registry.go](/Users/herbst/git/temp/llm-finder/internal/providers/registry.go).

## Changelog

See [CHANGELOG.md](/Users/herbst/git/temp/llm-finder/CHANGELOG.md).
