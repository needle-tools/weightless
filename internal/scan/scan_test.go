package scan

import (
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
