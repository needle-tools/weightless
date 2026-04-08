# Changelog

## 1.0.0 - 2026-04-08

Initial stable release.

Highlights:

- Added a terminal UI with Summary and Models views for browsing local model storage
- Added JSON output for exact machine-readable scans and scripting
- Added provider drill-down, refresh, created dates, and open/reveal actions
- Grouped raw files into logical model rows instead of file-per-file output

Provider coverage:

- Added support for major local model stores including Ollama, LM Studio, AnythingLLM, Draw Things, Upscayl, GPT4All, Jan, vLLM, Chrome Built-in AI, NVIDIA, and shared Hugging Face caches
- Added attribution for `llama.cpp` models that use the shared Hugging Face cache
- Added project-local discovery for common `models`, `weights`, and `checkpoints` directories

Distribution:

- Added versioned builds and `--version`
- Added GitHub Actions CI and release automation
- Added GoReleaser config for macOS, Linux, and Windows release artifacts
- Added Homebrew tap publishing support
- Added a release-downloading install script
