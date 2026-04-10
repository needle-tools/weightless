package scan

import (
	"os"
	"path/filepath"
	"testing"

	"weightless/internal/providers"
)

func TestCandidateAcceptsAmbiguousBinWithModelSignals(t *testing.T) {
	spec := providers.LocationSpec{
		Provider:          "test",
		MinSizeBytes:      1,
		ForcePathContains: []string{"/BNNSModels/", "/models/"},
	}

	path := "/Users/herbst/Library/Caches/SiriTTS/BNNSModels/25E246/p2a_dual_prompt_encoder_7e661f6b-b803-4b63-8510-6bde7f12c852.bin"
	if !candidate(path, 1024, spec) {
		t.Fatalf("expected %s to be treated as a model artifact", path)
	}
}

func TestCandidateRejectsGenericBinWithoutModelSignals(t *testing.T) {
	spec := providers.LocationSpec{
		Provider:     "test",
		MinSizeBytes: 1,
	}

	path := "/Users/herbst/Downloads/GSTestScene.bin"
	if candidate(path, 1024, spec) {
		t.Fatalf("expected %s to be rejected", path)
	}
}

func TestCandidateRejectsGlbEvenInsideModelsFolder(t *testing.T) {
	spec := providers.LocationSpec{
		Provider:          "test",
		MinSizeBytes:      1,
		ForcePathContains: []string{"/models/"},
	}

	path := "/Users/herbst/foo/models/GSTestScene.glb"
	if candidate(path, 1024, spec) {
		t.Fatalf("expected %s to be rejected", path)
	}
}

func TestCandidateAcceptsExtensionlessBlobInsideForcedModelPath(t *testing.T) {
	spec := providers.LocationSpec{
		Provider:          "ollama",
		MinSizeBytes:      1,
		ForcePathContains: []string{"/blobs/"},
	}

	path := "/Users/herbst/.ollama/models/blobs/sha256-deadbeef"
	if !candidate(path, 1024, spec) {
		t.Fatalf("expected %s to be accepted", path)
	}
}

func TestChoosePrimaryProviderPrefersUnslothOverHuggingFace(t *testing.T) {
	artifact := Artifact{
		PrimaryProvider: "huggingface",
		PrimaryLocation: "Hugging Face cache",
		Matches: []Match{
			{Provider: "huggingface", Location: "Hugging Face cache"},
			{Provider: "unsloth-studio", Location: "Unsloth shared cache"},
		},
	}

	provider, location := choosePrimaryProvider(artifact, nil)
	if provider != "unsloth-studio" || location != "Unsloth shared cache" {
		t.Fatalf("expected unsloth-studio to win, got %q / %q", provider, location)
	}
}

func TestFilterExclusiveDiskArtifactsDropsExplicitRoots(t *testing.T) {
	tmp := t.TempDir()
	hfRoot := filepath.Join(tmp, ".cache", "huggingface", "hub", "models--foo--bar")
	if err := os.MkdirAll(hfRoot, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	artifacts := []Artifact{
		{
			Name:            "bar",
			PrimaryProvider: "disk-scan",
			Path:            filepath.Join(hfRoot, "weights.safetensors"),
			CanonicalPath:   filepath.Join(hfRoot, "weights.safetensors"),
			AllPaths:        []string{filepath.Join(hfRoot, "weights.safetensors")},
		},
		{
			Name:            "leftover",
			PrimaryProvider: "disk-scan",
			Path:            filepath.Join(tmp, "Downloads", "drawThings", "leftover.safetensors"),
			CanonicalPath:   filepath.Join(tmp, "Downloads", "drawThings", "leftover.safetensors"),
			AllPaths:        []string{filepath.Join(tmp, "Downloads", "drawThings", "leftover.safetensors")},
		},
	}

	filtered := filterExclusiveDiskArtifacts(artifacts, []resolvedProviderRoot{{
		provider: "huggingface",
		location: "Hugging Face cache",
		root:     hfRoot,
	}})
	if len(filtered) != 1 {
		t.Fatalf("expected 1 leftover artifact, got %d", len(filtered))
	}
	if filtered[0].Name != "leftover" {
		t.Fatalf("expected leftover artifact to remain, got %q", filtered[0].Name)
	}
}
