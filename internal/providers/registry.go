package providers

import (
	"fmt"
	"runtime"
)

type LocationSpec struct {
	Provider          string
	Name              string
	Roots             []string
	MinSizeBytes      int64
	ForcePathContains []string
	Notes             string
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
			Provider: "project-local",
			Name:     "Project-local model roots",
			Roots: append([]string{
				"~/git/*/models",
				"~/git/*/model",
				"~/git/*/checkpoints",
				"~/git/*/weights",
				"~/git/*/loras",
				"~/git/*/embeddings",
				"~/code/*/models",
				"~/code/*/checkpoints",
				"~/code/*/weights",
				"~/src/*/models",
				"~/src/*/checkpoints",
				"~/src/*/weights",
				"~/Downloads/models",
				"~/Downloads/checkpoints",
				"~/Downloads/weights",
				"~/Downloads/*/models",
				"~/Downloads/*/checkpoints",
				"~/Downloads/*/weights",
				"~/Documents/models",
				"~/Documents/checkpoints",
				"~/Documents/weights",
				"~/Documents/*/models",
				"~/Documents/*/checkpoints",
				"~/Documents/*/weights",
			}, additionalRoots...),
			MinSizeBytes:      32 << 20,
			ForcePathContains: []string{"/models/", "/model/", "/checkpoints/", "/weights/", "/gguf/", "/loras/", "/embeddings/"},
			Notes:             "Fallback discovery for llama.cpp, flux.c, koboldcpp, ComfyUI, and one-off repos.",
		},
	}

	return normalize(roots)
}

func normalize(specs []LocationSpec) []LocationSpec {
	out := make([]LocationSpec, 0, len(specs))
	for _, spec := range specs {
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
