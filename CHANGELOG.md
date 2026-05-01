# Changelog

## 1.1.0 - 2026-05-01

Expanded `weightless` beyond model weights into broader local AI storage.

Highlights:

- Added dedicated TUI tabs for Models, Virtual Machines, and LLM Sessions
- Added JSON `category` metadata per artifact plus top-level category summaries
- Added high-performance exact sizing for known VM/session roots with parallel root sizing
- Added summary type labels for `MODEL`, `VM`, and `SESSION`

Virtual machine and runtime coverage:

- Added Docker Desktop VM disk detection
- Added Podman machine backing disk detection
- Added Lima and Colima instance detection
- Added Apple simulator device detection with human-readable names and runtimes from `device.plist`
- Added Apple simulator runtime and Xcode device support detection
- Added Android emulator AVD detection
- Added Claude VM, Codex VM, UTM, and Vercel Sandbox detection

LLM session coverage:

- Added Claude, Codex, Copilot, Antigravity, and OpenCode session stores
- Added Cursor, Windsurf, Cline, Roo Code, Kilo Code, Aider, Gemini CLI, and Qwen Code session stores
- Narrowed Antigravity detection to actual global/workspace state databases instead of whole app/browser caches

## 1.0.2 - 2026-04-10

Second patch release for `weightless`.

Fixes:

- Moved the initial provider scan into the TUI so startup and refresh use the same in-app progress flow
- Turned `disk-scan` into a true on-demand home-tree scan instead of a narrow project-folder sweep
- Kept `disk-scan` status and layout stable while drilling in and back out
- Stopped double-counting shared-cache and explicit-provider models by assigning each artifact to a single most-specific provider
- Made `disk-scan` report only leftover models instead of re-reporting storage already claimed by explicit providers

## 1.0.1 - 2026-04-10

First patch release for `weightless`.

Fixes:

- Made scans cwd-independent so running `weightless` from `~` or a repo returns the same results
- Added startup progress reporting before the TUI opens so long scans no longer look frozen
- Changed the broad fallback scan into a lazy `disk-scan` provider that only runs when drilled into from Summary
- Tightened ambiguous file detection so generic `.bin` files and unrelated assets under `models/` are no longer treated as model weights
- Added regression coverage for ambiguous file detection and cross-provider scan behavior

## 1.0.0 - 2026-04-08

Initial stable release of `weightless`.

Highlights:

- Added a terminal UI with Summary and Models views for browsing local model storage
- Added JSON output for exact machine-readable scans and scripting
- Added provider drill-down, refresh, created dates, and open/reveal actions
- Grouped raw files into logical model rows instead of file-per-file output

Provider coverage:

- Added support for major local model stores including Ollama, LM Studio, AnythingLLM, Draw Things, Upscayl, GPT4All, Jan, vLLM, Chrome Built-in AI, NVIDIA, and shared Hugging Face caches
- Added attribution for `llama.cpp` models that use the shared Hugging Face cache
- Added fallback discovery for common `models`, `weights`, and `checkpoints` directories

Distribution:

- Added versioned builds and `--version`
- Added GitHub Actions CI and release automation
- Added GoReleaser config for macOS, Linux, and Windows release artifacts
- Added a release-downloading install script
