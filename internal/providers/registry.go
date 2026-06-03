package providers

import (
	"fmt"
	"runtime"
)

type LocationSpec struct {
	Provider          string
	Category          string
	Name              string
	Roots             []string
	ArtifactMode      string
	MinSizeBytes      int64
	ForcePathContains []string
	Notes             string
	Lazy              bool
}

func Registry(additionalRoots []string) []LocationSpec {
	roots := []LocationSpec{
		{
			Provider:          "ollama",
			Name:              "Ollama default store",
			Roots:             pickOS("~/.ollama/models", "/usr/share/ollama/.ollama/models", "~/.ollama/models"),
			MinSizeBytes:      1 << 20,
			ForcePathContains: []string{"/blobs/", "/manifests/"},
			Notes:             "Official Ollama model directory plus configurable OLLAMA_MODELS overrides.",
		},
		{
			Provider:          "lm-studio",
			Name:              "LM Studio model store",
			Roots:             []string{"~/.lmstudio/models"},
			MinSizeBytes:      1 << 20,
			ForcePathContains: []string{"/models/"},
			Notes:             "LM Studio defaults to ~/.lmstudio/models on macOS and Linux unless changed in-app.",
		},
		{
			Provider:          "anythingllm",
			Name:              "AnythingLLM desktop storage",
			Roots:             []string{"~/Library/Application Support/anythingllm-desktop/storage/models", "~/.config/anythingllm-desktop/storage/models", "~/.local/share/anythingllm-desktop/storage/models", "%APPDATA%/anythingllm-desktop/storage/models", "%LOCALAPPDATA%/anythingllm-desktop/storage/models"},
			MinSizeBytes:      1 << 20,
			ForcePathContains: []string{"/storage/models/"},
			Notes:             "AnythingLLM stores bundled and provider-managed local model assets here.",
		},
		{
			Provider:          "draw-things",
			Name:              "Draw Things container models",
			Roots:             []string{"~/Library/Containers/com.liuliu.draw-things/Data/Documents/Models"},
			MinSizeBytes:      1 << 20,
			ForcePathContains: []string{"/Models/"},
			Notes:             "Draw Things app container on macOS.",
		},
		{
			Provider: "upscayl",
			Name:     "Upscayl app storage",
			Roots: []string{
				"~/Library/Application Support/Upscayl",
				"~/Library/Application Support/upscayl",
				"/Applications/Upscayl.app/Contents/Resources/models",
				"~/.config/Upscayl",
				"~/.local/share/Upscayl",
				"/opt/Upscayl/resources/models",
				"/usr/lib/upscayl/resources/models",
				"%APPDATA%/Upscayl",
				"%LOCALAPPDATA%/Upscayl",
				"%LOCALAPPDATA%/Programs/Upscayl/resources/models",
				"%ProgramFiles%/Upscayl/resources/models",
			},
			MinSizeBytes:      1 << 20,
			ForcePathContains: []string{"/Contents/Resources/models/", "/resources/models/", "/models/", "/custom-models/"},
			Notes:             "Catches bundled app models and custom NCNN models.",
		},
		{
			Provider: "unsloth-studio",
			Name:     "Unsloth shared cache",
			Roots: []string{
				"${HF_HUB_CACHE}/models--unsloth--*",
				"${HF_HUB_CACHE}/models/unsloth/*",
				"${HF_HOME}/hub/models--unsloth--*",
				"${HF_HOME}/hub/models/unsloth/*",
				"~/.cache/huggingface/hub/models--unsloth--*",
				"~/.cache/huggingface/hub/models/unsloth/*",
			},
			MinSizeBytes:      1 << 20,
			ForcePathContains: []string{"/models--unsloth--", "/hub/models/unsloth/"},
			Notes:             "Unsloth distributes many local-run models through its Hugging Face namespace; GGUF docs also reference llama.cpp/LLAMA_CACHE.",
		},
		{
			Provider: "huggingface",
			Name:     "Hugging Face cache",
			Roots: []string{
				"${HF_HUB_CACHE}/models--*",
				"${HF_HUB_CACHE}/models/*",
				"${HF_HOME}/hub/models--*",
				"${HF_HOME}/hub/models/*",
				"~/.cache/huggingface/hub/models--*",
				"~/.cache/huggingface/hub/models/*",
			},
			MinSizeBytes:      1 << 20,
			ForcePathContains: []string{"/models--", "/hub/models/", "/snapshots/", "/blobs/"},
			Notes:             "Shared by huggingface_hub, transformers, diffusers, mlx, vllm, and many others.",
		},
		{
			Provider:          "jan",
			Name:              "Jan data folder",
			Roots:             []string{"~/Library/Application Support/Jan/data/models", "~/.config/Jan/data/models", "~/.local/share/Jan/data/models", "%APPDATA%/Jan/data/models", "%LOCALAPPDATA%/Jan/data/models"},
			MinSizeBytes:      1 << 20,
			ForcePathContains: []string{"/data/models/"},
			Notes:             "Jan desktop model store.",
		},
		{
			Provider:          "gpt4all",
			Name:              "GPT4All models",
			Roots:             pickOS("~/Library/Application Support/nomic.ai/GPT4All", "~/.local/share/nomic.ai/GPT4All", "%LOCALAPPDATA%/nomic.ai/GPT4All"),
			MinSizeBytes:      1 << 20,
			ForcePathContains: []string{"/GPT4All/"},
			Notes:             "GPT4All desktop download path.",
		},
		{
			Provider: "vllm",
			Name:     "vLLM caches",
			Roots: []string{
				"${VLLM_ASSETS_CACHE}",
				"${VLLM_CACHE_ROOT}",
				"${VLLM_CONFIG_ROOT}",
				"~/.cache/vllm",
				"~/.cache/vllm/assets",
				"~/.config/vllm",
				"%LOCALAPPDATA%/vllm",
				"%APPDATA%/vllm",
			},
			MinSizeBytes:      1 << 20,
			ForcePathContains: []string{"/.cache/vllm/", "/.config/vllm/", "/vllm/assets/"},
			Notes:             "vLLM uses ~/.cache/vllm and ~/.config/vllm by default; model weights still default to the Hugging Face cache unless --download-dir is set.",
		},
		{
			Provider:          "node-llama-cpp",
			Name:              "node-llama-cpp cache",
			Roots:             []string{"~/.node-llama-cpp"},
			MinSizeBytes:      1 << 20,
			ForcePathContains: []string{"/.node-llama-cpp/"},
			Notes:             "Downloaded binaries and model-adjacent assets for node-llama-cpp workflows.",
		},
		{
			Provider: "chrome-built-in-ai",
			Name:     "Chrome component caches",
			Roots: []string{
				"~/Library/Application Support/Google/Chrome/component_crx_cache",
				"~/Library/Application Support/Chromium/component_crx_cache",
				"~/.config/google-chrome/component_crx_cache",
				"~/.config/chromium/component_crx_cache",
				"%LOCALAPPDATA%/Google/Chrome/User Data/component_crx_cache",
				"%LOCALAPPDATA%/Chromium/User Data/component_crx_cache",
			},
			MinSizeBytes:      8 << 20,
			ForcePathContains: []string{"/component_crx_cache/", "OptimizationGuide", "OnDevice", "OptGuideOnDevice"},
			Notes:             "Chrome Built-in AI / on-device model components live in Chrome user data and component caches.",
		},
		{
			Provider:          "nvidia",
			Name:              "NVIDIA local caches",
			Roots:             []string{"${NIM_CACHE_PATH}", "~/.cache/nim", "~/.nvwb", "~/Library/Application Support/NVIDIA"},
			MinSizeBytes:      8 << 20,
			ForcePathContains: []string{"/.cache/nim", "/.nvwb/", "/NVIDIA/"},
			Notes:             "Catches NIM caches, AI Workbench workbench dir, and app-managed NVIDIA stores.",
		},
		{
			Provider: "text-generation-webui",
			Name:     "text-generation-webui models",
			Roots: []string{
				"~/text-generation-webui/user_data/models",
				"~/git/*/text-generation-webui/user_data/models",
				"~/code/*/text-generation-webui/user_data/models",
				"~/src/*/text-generation-webui/user_data/models",
				"~/Downloads/text-generation-webui/user_data/models",
				"~/Downloads/*/text-generation-webui/user_data/models",
			},
			MinSizeBytes:      1 << 20,
			ForcePathContains: []string{"/user_data/models/"},
			Notes:             "text-generation-webui stores models in user_data/models by default.",
		},
		{
			Provider: "comfy",
			Name:     "ComfyUI models",
			Roots: []string{
				"~/ComfyUI/models",
				"~/comfyui/models",
				"~/git/*/ComfyUI/models",
				"~/git/*/comfyui/models",
				"~/code/*/ComfyUI/models",
				"~/code/*/comfyui/models",
				"~/src/*/ComfyUI/models",
				"~/src/*/comfyui/models",
				"~/Downloads/ComfyUI/models",
				"~/Downloads/*/ComfyUI/models",
			},
			MinSizeBytes:      1 << 20,
			ForcePathContains: []string{"/models/checkpoints/", "/models/vae/", "/models/"},
			Notes:             "ComfyUI expects checkpoints and related assets under its models directory.",
		},
		{
			Provider: "stable-diffusion-webui",
			Name:     "stable-diffusion-webui models",
			Roots: []string{
				"~/stable-diffusion-webui/models",
				"~/git/*/stable-diffusion-webui/models",
				"~/code/*/stable-diffusion-webui/models",
				"~/src/*/stable-diffusion-webui/models",
				"~/Downloads/stable-diffusion-webui/models",
				"~/Downloads/*/stable-diffusion-webui/models",
			},
			MinSizeBytes:      1 << 20,
			ForcePathContains: []string{"/models/Stable-diffusion/", "/models/VAE/", "/models/Lora/", "/models/"},
			Notes:             "AUTOMATIC1111 stores checkpoints under stable-diffusion-webui/models by default.",
		},
		{
			Provider: "invokeai",
			Name:     "InvokeAI models",
			Roots: []string{
				"${INVOKEAI_ROOT}/models",
				"~/invokeai/models",
				"~/.invokeai/models",
				"~/Library/Application Support/InvokeAI/models",
			},
			MinSizeBytes:      1 << 20,
			ForcePathContains: []string{"/invokeai/models/", "/.invokeai/models/", "/InvokeAI/models/"},
			Notes:             "InvokeAI keeps managed models under its InvokeAI root models directory.",
		},
		{
			Provider:          "disk-scan",
			Name:              "Disk scan model roots",
			Roots:             append(pickOS("~", "~", "%USERPROFILE%"), additionalRoots...),
			MinSizeBytes:      32 << 20,
			ForcePathContains: []string{"/models/", "/model/", "/checkpoints/", "/weights/", "/gguf/", "/loras/", "/embeddings/"},
			Notes:             "Broad on-demand scan across the user home tree for one-off local model files and folders.",
			Lazy:              true,
		},
		{
			Provider:     "docker",
			Category:     "virtual_machines",
			Name:         "Docker Desktop VM disks",
			ArtifactMode: "root",
			Roots: []string{
				"~/Library/Containers/com.docker.docker/Data/vms/*/data/Docker.raw",
				"~/.docker/desktop/vms/*/data/Docker.raw",
				"%LOCALAPPDATA%/Docker/wsl/data/ext4.vhdx",
				"%LOCALAPPDATA%/Docker/wsl/distro/ext4.vhdx",
			},
			Notes: "Docker Desktop virtual machine and backing disk stores.",
		},
		{
			Provider:     "podman",
			Category:     "virtual_machines",
			Name:         "Podman machine disks",
			ArtifactMode: "root",
			Roots: []string{
				"~/.local/share/containers/podman/machine/*/*.raw",
				"~/.local/share/containers/podman/machine/*/*.qcow2",
				"~/.local/share/containers/podman/machine/*/*.vhdx",
			},
			Notes: "Podman machine backing disk images.",
		},
		{
			Provider:     "lima",
			Category:     "virtual_machines",
			Name:         "Lima instances",
			ArtifactMode: "root",
			Roots:        []string{"~/.lima/*", "~/.colima/*"},
			Notes:        "Lima and Colima VM instances.",
		},
		{
			Provider:     "apple-simulators",
			Category:     "virtual_machines",
			Name:         "Apple simulator devices",
			ArtifactMode: "root",
			Roots: []string{
				"~/Library/Developer/CoreSimulator/Devices/*/device.plist",
				"~/Library/Developer/Xcode/UserData/Previews/Simulator Devices/*",
				"~/Library/Developer/Xcode/UserData/Previews/Simulator%2520Devices/*",
			},
			Notes: "iOS, visionOS, watchOS, tvOS, and preview simulator device data.",
		},
		{
			Provider:     "apple-simulator-runtimes",
			Category:     "virtual_machines",
			Name:         "Apple simulator runtimes and device support",
			ArtifactMode: "root",
			Roots: []string{
				"~/Library/Developer/CoreSimulator/Profiles/Runtimes/*.simruntime",
				"/Library/Developer/CoreSimulator/Profiles/Runtimes/*.simruntime",
				"/Applications/Xcode*.app/Contents/Developer/Platforms/*/Library/Developer/CoreSimulator/Profiles/Runtimes/*.simruntime",
				"~/Library/Developer/Xcode/iOS DeviceSupport/*",
				"~/Library/Developer/Xcode/watchOS DeviceSupport/*",
				"~/Library/Developer/Xcode/visionOS DeviceSupport/*",
				"~/Library/Developer/Xcode/tvOS DeviceSupport/*",
			},
			Notes: "Downloaded simulator runtimes and paired-device symbol stores.",
		},
		{
			Provider:     "android-emulator",
			Category:     "virtual_machines",
			Name:         "Android emulator AVDs",
			ArtifactMode: "root",
			Roots:        []string{"~/.android/avd/*.avd"},
			Notes:        "Android Studio emulator virtual device directories.",
		},
		{
			Provider:     "claude-vm",
			Category:     "virtual_machines",
			Name:         "Claude VM bundles",
			ArtifactMode: "root",
			Roots: []string{
				"~/Library/Application Support/Claude/vm_bundles/*",
				"~/Library/Application Support/Claude/claude-code-vm",
			},
			Notes: "Claude Desktop and Claude Code VM bundles.",
		},
		{
			Provider:     "codex-vm",
			Category:     "virtual_machines",
			Name:         "Codex app VM and browser partitions",
			ArtifactMode: "root",
			Roots: []string{
				"~/Library/Application Support/Codex/Partitions/*",
				"~/Library/Application Support/Codex/.com.openai.codex.*",
			},
			Notes: "Codex app isolated partitions and VM-adjacent local state.",
		},
		{
			Provider:     "utm",
			Category:     "virtual_machines",
			Name:         "UTM virtual machines",
			ArtifactMode: "root",
			Roots:        []string{"~/Library/Containers/com.utmapp.UTM/Data/Documents/*.utm", "~/Documents/*.utm"},
			Notes:        "UTM virtual machine bundles.",
		},
		{
			Provider:     "vercel-sandbox",
			Category:     "virtual_machines",
			Name:         "Vercel Sandbox caches",
			ArtifactMode: "root",
			Roots:        []string{"~/.vercel/sandbox", "~/.cache/vercel/sandbox"},
			Notes:        "Local Vercel Sandbox VM and browser automation caches.",
		},
		{
			Provider:     "unity",
			Category:     "project_caches",
			Name:         "Unity project Libraries",
			ArtifactMode: "root",
			Roots:        unityProjectCacheRoots(additionalRoots),
			MinSizeBytes: 8 << 20,
			Notes:        "Unity Library folders are imported asset and build caches that Unity can regenerate from Assets, Packages, and ProjectSettings.",
		},
		{
			Provider:     "unreal",
			Category:     "project_caches",
			Name:         "Unreal project generated data",
			ArtifactMode: "root",
			Roots:        projectCacheRoots(additionalRoots, "DerivedDataCache", "Intermediate"),
			MinSizeBytes: 8 << 20,
			Notes:        "Unreal DerivedDataCache and Intermediate folders are generated project data and can usually be rebuilt by the editor.",
		},
		{
			Provider:     "unreal-ddc",
			Category:     "project_caches",
			Name:         "Unreal shared DerivedDataCache",
			ArtifactMode: "root",
			Roots: []string{
				"~/Library/Application Support/Epic/UnrealEngine/Common/DerivedDataCache",
				"~/.cache/Epic/UnrealEngine/Common/DerivedDataCache",
				"%LOCALAPPDATA%/UnrealEngine/Common/DerivedDataCache",
			},
			MinSizeBytes: 8 << 20,
			Notes:        "Shared Unreal Engine derived data cache; it can be regenerated but may cost shader and asset rebuild time.",
		},
		{
			Provider:     "needle-engine",
			Category:     "project_caches",
			Name:         "Needle Engine build caches",
			ArtifactMode: "root",
			Roots:        projectCacheRoots(additionalRoots, ".vite", ".svelte-kit", ".next/cache"),
			MinSizeBytes: 1 << 20,
			Notes:        "Needle Engine web build caches are regenerated by the JavaScript toolchain.",
		},
		{
			Provider:     "javascript-deps",
			Category:     "project_caches",
			Name:         "JavaScript project dependencies",
			ArtifactMode: "root",
			Roots:        projectCacheRoots(additionalRoots, "node_modules"),
			MinSizeBytes: 8 << 20,
			Notes:        "node_modules folders are dependency installs that can be recreated from package manifests and lockfiles.",
		},
		{
			Provider:     "git-lfs",
			Category:     "project_caches",
			Name:         "Git LFS object caches",
			ArtifactMode: "root",
			Roots:        projectCacheRoots(additionalRoots, ".git/lfs/objects"),
			MinSizeBytes: 8 << 20,
			Notes:        "Git LFS object stores can often be pruned with git lfs prune; avoid deleting active local-only objects blindly.",
		},
		{
			Provider:     "adobe-after-effects",
			Category:     "project_caches",
			Name:         "After Effects disk caches",
			ArtifactMode: "root",
			Roots: []string{
				"~/Library/Caches/Adobe/After Effects/*/Disk Cache",
				"~/Library/Caches/Adobe/After Effects/*/Conformed Media Cache",
				"%APPDATA%/Adobe/After Effects/*/Disk Cache",
				"%APPDATA%/Adobe/After Effects/*/Conformed Media Cache",
			},
			MinSizeBytes: 8 << 20,
			Notes:        "After Effects disk and conformed media caches are preview/render caches that can be rebuilt.",
		},
		{
			Provider:     "adobe-media-cache",
			Category:     "project_caches",
			Name:         "Adobe shared media caches",
			ArtifactMode: "root",
			Roots: []string{
				"~/Library/Application Support/Adobe/Common/Media Cache",
				"~/Library/Application Support/Adobe/Common/Media Cache Files",
				"~/Library/Caches/Adobe/Common/Media Cache",
				"~/Library/Caches/Adobe/Common/Media Cache Files",
				"%APPDATA%/Adobe/Common/Media Cache",
				"%APPDATA%/Adobe/Common/Media Cache Files",
				"%LOCALAPPDATA%/Adobe/Common/Media Cache",
				"%LOCALAPPDATA%/Adobe/Common/Media Cache Files",
			},
			MinSizeBytes: 8 << 20,
			Notes:        "Shared Adobe media cache files used by Premiere Pro, After Effects, and Media Encoder.",
		},
		{
			Provider:     "premiere-projects",
			Category:     "project_caches",
			Name:         "Premiere Pro project previews",
			ArtifactMode: "root",
			Roots:        projectCacheRoots(additionalRoots, "Adobe Premiere Pro Video Previews", "Adobe Premiere Pro Audio Previews"),
			MinSizeBytes: 8 << 20,
			Notes:        "Premiere Pro preview render folders are generated project media previews.",
		},
		{
			Provider:     "davinci-resolve",
			Category:     "project_caches",
			Name:         "DaVinci Resolve cache media",
			ArtifactMode: "root",
			Roots: append([]string{
				"~/Movies/CacheClip",
				"~/Movies/ProxyMedia",
				"~/Library/Application Support/Blackmagic Design/DaVinci Resolve/CacheClip",
				"~/Library/Application Support/Blackmagic Design/DaVinci Resolve/ProxyMedia",
				"%LOCALAPPDATA%/Blackmagic Design/DaVinci Resolve/CacheClip",
				"%LOCALAPPDATA%/Blackmagic Design/DaVinci Resolve/ProxyMedia",
			}, projectCacheRoots(additionalRoots, "CacheClip", "ProxyMedia", "OptimizedMedia")...),
			MinSizeBytes: 8 << 20,
			Notes:        "DaVinci Resolve render cache, optimized media, and proxy media can be regenerated from source footage.",
		},
		{
			Provider:     "claude",
			Category:     "llm_sessions",
			Name:         "Claude session files",
			ArtifactMode: "root",
			Roots: []string{
				"~/.claude/projects",
				"~/.claude/sessions",
				"~/.claude/todos",
				"~/.claude/session-env",
				"~/.claude/file-history",
				"~/.claude/plans",
				"~/.local/share/claude",
				"~/.local/state/claude",
				"~/Library/Application Support/Claude/blob_storage",
				"~/Library/Logs/Claude",
			},
			Notes: "Claude Code and Claude Desktop conversation, project, and session state.",
		},
		{
			Provider:     "codex",
			Category:     "llm_sessions",
			Name:         "Codex session files",
			ArtifactMode: "root",
			Roots: []string{
				"~/.codex/sessions",
				"~/.codex/sqlite",
				"~/.codex/memories",
				"~/.codex/log",
				"~/Library/Logs/com.openai.codex",
			},
			Notes: "Codex CLI and app session databases, transcripts, memories, and logs.",
		},
		{
			Provider:     "copilot",
			Category:     "llm_sessions",
			Name:         "GitHub Copilot session files",
			ArtifactMode: "root",
			Roots: []string{
				"~/Library/Application Support/Code/User/globalStorage/github.copilot-chat",
				"~/Library/Application Support/github-copilot",
				"~/.config/github-copilot",
				"~/.config/gh-copilot",
				"~/.local/state/gh-copilot",
				"~/.copilot",
			},
			Notes: "GitHub Copilot Chat, CLI, and editor session state.",
		},
		{
			Provider:     "antigravity",
			Category:     "llm_sessions",
			Name:         "Antigravity session files",
			ArtifactMode: "root",
			Roots: []string{
				"~/Library/Application Support/Antigravity/User/globalStorage/state.vscdb",
				"~/Library/Application Support/Antigravity/User/workspaceStorage/*/state.vscdb",
			},
			Notes: "Google Antigravity local global and workspace session state.",
		},
		{
			Provider:     "opencode",
			Category:     "llm_sessions",
			Name:         "OpenCode session files",
			ArtifactMode: "root",
			Roots: []string{
				"~/.local/share/opencode",
				"~/.local/state/opencode",
			},
			Notes: "OpenCode session databases and state files.",
		},
		{
			Provider:     "cursor",
			Category:     "llm_sessions",
			Name:         "Cursor session files",
			ArtifactMode: "root",
			Roots: []string{
				"~/Library/Application Support/Cursor/User/globalStorage/state.vscdb",
				"~/Library/Application Support/Cursor/User/globalStorage/cursor.cursor/cursor-ai/chat-history",
				"~/Library/Application Support/Cursor/User/workspaceStorage/*/state.vscdb",
				"~/.config/Cursor/User/globalStorage/state.vscdb",
				"~/.config/Cursor/User/globalStorage/cursor.cursor/cursor-ai/chat-history",
				"~/.config/Cursor/User/workspaceStorage/*/state.vscdb",
			},
			Notes: "Cursor stores local chat history in user/global and workspace SQLite state.",
		},
		{
			Provider:     "windsurf",
			Category:     "llm_sessions",
			Name:         "Windsurf session files",
			ArtifactMode: "root",
			Roots: []string{
				"~/.codeium/windsurf/cascade",
				"~/.codeium/windsurf/memories",
				"~/Library/Application Support/Windsurf/User/globalStorage/state.vscdb",
				"~/Library/Application Support/Windsurf/User/workspaceStorage/*/state.vscdb",
				"~/.config/Windsurf/User/globalStorage/state.vscdb",
				"~/.config/Windsurf/User/workspaceStorage/*/state.vscdb",
			},
			Notes: "Windsurf Cascade local session and memory stores.",
		},
		{
			Provider:     "cline",
			Category:     "llm_sessions",
			Name:         "Cline task files",
			ArtifactMode: "root",
			Roots: []string{
				"~/.cline/data/tasks",
				"~/Library/Application Support/Code/User/globalStorage/saoudrizwan.claude-dev/tasks",
				"~/.config/Code/User/globalStorage/saoudrizwan.claude-dev/tasks",
			},
			Notes: "Cline task history and conversation JSON files.",
		},
		{
			Provider:     "roo-code",
			Category:     "llm_sessions",
			Name:         "Roo Code task files",
			ArtifactMode: "root",
			Roots: []string{
				"~/Library/Application Support/Code/User/globalStorage/rooveterinaryinc.roo-cline/tasks",
				"~/.config/Code/User/globalStorage/rooveterinaryinc.roo-cline/tasks",
			},
			Notes: "Roo Code task history in editor global storage.",
		},
		{
			Provider:     "kilo-code",
			Category:     "llm_sessions",
			Name:         "Kilo Code task files",
			ArtifactMode: "root",
			Roots: []string{
				"~/Library/Application Support/Code/User/globalStorage/kilocode.kilo-code",
				"~/.config/Code/User/globalStorage/kilocode.kilo-code",
			},
			Notes: "Kilo Code task history in editor global storage.",
		},
		{
			Provider:     "aider",
			Category:     "llm_sessions",
			Name:         "Aider chat history files",
			ArtifactMode: "root",
			Roots: []string{
				"~/.aider.chat.history.md",
				"~/.aider.input.history",
				"~/.aider.llm.history",
				"~/git/*/.aider.chat.history.md",
				"~/code/*/.aider.chat.history.md",
				"~/src/*/.aider.chat.history.md",
				"~/Downloads/*/.aider.chat.history.md",
			},
			Notes: "Aider's default chat, input, and optional LLM history files.",
		},
		{
			Provider:     "gemini-cli",
			Category:     "llm_sessions",
			Name:         "Gemini CLI session files",
			ArtifactMode: "root",
			Roots: []string{
				"~/.gemini/tmp/*/chats",
				"~/.gemini/tmp/*/checkpoints",
			},
			Notes: "Gemini CLI per-project saved chats and checkpoints.",
		},
		{
			Provider:     "qwen-code",
			Category:     "llm_sessions",
			Name:         "Qwen Code session files",
			ArtifactMode: "root",
			Roots: []string{
				"~/.qwen/tmp/*/checkpoints",
				"~/.qwen/tmp/*/shell_history",
			},
			Notes: "Qwen Code checkpoint and shell history files.",
		},
	}

	return normalize(roots)
}

func projectCacheRoots(additionalRoots []string, cachePaths ...string) []string {
	projectGlobs := defaultProjectGlobs(additionalRoots)

	out := make([]string, 0, len(projectGlobs)*len(cachePaths))
	for _, projectGlob := range projectGlobs {
		for _, cachePath := range cachePaths {
			out = append(out, projectGlob+"/"+cachePath)
		}
	}
	return out
}

func unityProjectCacheRoots(additionalRoots []string) []string {
	out := projectCacheRoots(additionalRoots, "Library")
	for _, workspace := range projectWorkspaceRoots(additionalRoots) {
		for _, container := range []string{"development", "modules", "projects", "project", "samples", "Samples", "tests", "Tests", "TestProjects", "temp"} {
			out = append(out,
				workspace+"/*/"+container+"/*/Library",
				workspace+"/*/"+container+"/*/*/Library",
			)
		}
	}
	return out
}

func defaultProjectGlobs(additionalRoots []string) []string {
	out := make([]string, 0)
	for _, root := range projectWorkspaceRoots(additionalRoots) {
		out = append(out, root, root+"/*", root+"/*/*")
	}
	return out
}

func projectWorkspaceRoots(additionalRoots []string) []string {
	out := []string{
		"~/git",
		"~/code",
		"~/src",
		"~/work",
		"~/Projects",
		"~/Documents",
		"~/Desktop",
		"~/Downloads",
	}
	for _, root := range additionalRoots {
		if root != "" {
			out = append(out, root)
		}
	}
	return out
}

func normalize(specs []LocationSpec) []LocationSpec {
	out := make([]LocationSpec, 0, len(specs))
	for _, spec := range specs {
		if spec.Category == "" {
			spec.Category = "models"
		}
		if spec.ArtifactMode == "" {
			spec.ArtifactMode = "files"
		}
		if spec.ArtifactMode == "root" && spec.MinSizeBytes == 0 {
			spec.MinSizeBytes = 1
		}
		filtered := make([]string, 0, len(spec.Roots))
		seen := map[string]struct{}{}
		for _, root := range spec.Roots {
			if root == "" {
				continue
			}
			if _, ok := seen[root]; ok {
				continue
			}
			seen[root] = struct{}{}
			filtered = append(filtered, root)
		}
		spec.Roots = filtered
		out = append(out, spec)
	}
	return out
}

func pickOS(darwin, linux, windows string) []string {
	switch runtime.GOOS {
	case "darwin":
		return []string{darwin}
	case "linux":
		return []string{linux}
	case "windows":
		return []string{windows}
	default:
		panic(fmt.Sprintf("unsupported OS: %s", runtime.GOOS))
	}
}
