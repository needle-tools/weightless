package scan

import (
	"os"
	"path/filepath"
	"testing"
	"time"

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

func TestExpandPathExpandsPercentEnvironmentVariables(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("LOCALAPPDATA", tmp)

	path, ok := expandPath("%LOCALAPPDATA%/Docker/wsl/data/ext4.vhdx", "/home/tester", "/work")
	if !ok {
		t.Fatalf("expected LOCALAPPDATA path to expand")
	}
	expected := filepath.Join(tmp, "Docker", "wsl", "data", "ext4.vhdx")
	if path != expected {
		t.Fatalf("expected %q, got %q", expected, path)
	}
}

func TestExpandPathRejectsUnresolvedPercentEnvironmentVariables(t *testing.T) {
	t.Setenv("MISSING_WINDOWS_ENV", "")
	os.Unsetenv("MISSING_WINDOWS_ENV")

	if path, ok := expandPath("%MISSING_WINDOWS_ENV%/cache", "/home/tester", "/work"); ok {
		t.Fatalf("expected unresolved percent env to be rejected, got %q", path)
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

func TestRunReportsVirtualMachineCategoryForRootArtifacts(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	vmRoot := filepath.Join(tmp, ".lima", "build-vm")
	if err := os.MkdirAll(vmRoot, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(vmRoot, "diffdisk"), []byte("1234567890"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	report, err := Run(Config{Providers: []string{"lima"}})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if report.TotalArtifacts != 1 {
		t.Fatalf("expected 1 artifact, got %d", report.TotalArtifacts)
	}
	artifact := report.Artifacts[0]
	if artifact.Category != "virtual_machines" {
		t.Fatalf("expected virtual_machines category, got %q", artifact.Category)
	}
	if artifact.PrimaryProvider != "lima" {
		t.Fatalf("expected lima provider, got %q", artifact.PrimaryProvider)
	}
	if artifact.SizeBytes != 10 {
		t.Fatalf("expected VM size 10, got %d", artifact.SizeBytes)
	}
	if len(report.Categories) != 1 || report.Categories[0].Category != "virtual_machines" || report.Categories[0].Bytes != 10 {
		t.Fatalf("unexpected category summaries: %#v", report.Categories)
	}
}

func TestRunReportsLLMSessionCategoryForRootArtifacts(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	sessionRoot := filepath.Join(tmp, ".local", "share", "opencode")
	if err := os.MkdirAll(sessionRoot, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sessionRoot, "opencode.db"), []byte("session-data"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	report, err := Run(Config{Providers: []string{"opencode"}})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if report.TotalArtifacts != 1 {
		t.Fatalf("expected 1 artifact, got %d", report.TotalArtifacts)
	}
	artifact := report.Artifacts[0]
	if artifact.Category != "llm_sessions" {
		t.Fatalf("expected llm_sessions category, got %q", artifact.Category)
	}
	if artifact.PrimaryProvider != "opencode" {
		t.Fatalf("expected opencode provider, got %q", artifact.PrimaryProvider)
	}
	if artifact.SizeBytes != int64(len("session-data")) {
		t.Fatalf("expected session size %d, got %d", len("session-data"), artifact.SizeBytes)
	}
	if len(report.Summary) != 1 || report.Summary[0].Provider != "opencode" || report.Summary[0].Bytes != int64(len("session-data")) {
		t.Fatalf("unexpected provider summary: %#v", report.Summary)
	}
}

func TestRunReportsUnityLibraryAsProjectCache(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	projectRoot := filepath.Join(tmp, "git", "game")
	libraryRoot := filepath.Join(projectRoot, "Library")
	if err := os.MkdirAll(filepath.Join(projectRoot, "Assets"), 0o755); err != nil {
		t.Fatalf("mkdir assets: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(projectRoot, "ProjectSettings"), 0o755); err != nil {
		t.Fatalf("mkdir project settings: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "ProjectSettings", "ProjectVersion.txt"), []byte("m_EditorVersion: 6000.0.0f1"), 0o644); err != nil {
		t.Fatalf("write project version: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(libraryRoot, "Artifacts"), 0o755); err != nil {
		t.Fatalf("mkdir library: %v", err)
	}
	cachePath := filepath.Join(libraryRoot, "Artifacts", "cache")
	cacheSize := int64(9 << 20)
	cacheFile, err := os.Create(cachePath)
	if err != nil {
		t.Fatalf("create cache: %v", err)
	}
	if err := cacheFile.Truncate(cacheSize); err != nil {
		cacheFile.Close()
		t.Fatalf("truncate cache: %v", err)
	}
	if err := cacheFile.Close(); err != nil {
		t.Fatalf("close cache: %v", err)
	}
	modifiedAt := time.Now().Add(-72 * time.Hour).Truncate(time.Second)
	if err := os.Chtimes(libraryRoot, modifiedAt, modifiedAt); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	report, err := Run(Config{Providers: []string{"unity"}})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if report.TotalArtifacts != 1 {
		t.Fatalf("expected 1 artifact, got %d", report.TotalArtifacts)
	}
	artifact := report.Artifacts[0]
	if artifact.Category != "project_caches" {
		t.Fatalf("expected project_caches category, got %q", artifact.Category)
	}
	if artifact.PrimaryProvider != "unity" {
		t.Fatalf("expected unity provider, got %q", artifact.PrimaryProvider)
	}
	if artifact.Name != "game / Library" {
		t.Fatalf("expected project cache name, got %q", artifact.Name)
	}
	if artifact.SizeBytes != cacheSize {
		t.Fatalf("expected cache size %d, got %d", cacheSize, artifact.SizeBytes)
	}
	if artifact.Timestamp != modifiedAt.Format(time.RFC3339) {
		t.Fatalf("expected library timestamp %q, got %q", modifiedAt.Format(time.RFC3339), artifact.Timestamp)
	}
	if len(report.Categories) != 1 || report.Categories[0].Category != "project_caches" || report.Categories[0].Bytes != cacheSize {
		t.Fatalf("unexpected category summaries: %#v", report.Categories)
	}
}

func TestUnityProviderSkipsPlainLibraryFolders(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	libraryRoot := filepath.Join(tmp, "git", "notes", "Library")
	if err := os.MkdirAll(libraryRoot, 0o755); err != nil {
		t.Fatalf("mkdir library: %v", err)
	}
	if err := os.WriteFile(filepath.Join(libraryRoot, "book.db"), []byte("not unity"), 0o644); err != nil {
		t.Fatalf("write cache: %v", err)
	}

	report, err := Run(Config{Providers: []string{"unity"}})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if report.TotalArtifacts != 0 {
		t.Fatalf("expected plain library folder to be skipped, got %#v", report.Artifacts)
	}
}

func TestRunReportsNestedUnityLibraryAsProjectCache(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	projectRoot := filepath.Join(tmp, "git", "toolkit", "projects", "SampleGame")
	libraryRoot := filepath.Join(projectRoot, "Library")
	if err := os.MkdirAll(filepath.Join(projectRoot, "Assets"), 0o755); err != nil {
		t.Fatalf("mkdir assets: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(projectRoot, "ProjectSettings"), 0o755); err != nil {
		t.Fatalf("mkdir project settings: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "ProjectSettings", "ProjectVersion.txt"), []byte("m_EditorVersion: 6000.0.0f1"), 0o644); err != nil {
		t.Fatalf("write project version: %v", err)
	}
	if err := os.MkdirAll(libraryRoot, 0o755); err != nil {
		t.Fatalf("mkdir library: %v", err)
	}
	cachePath := filepath.Join(libraryRoot, "cache")
	cacheSize := int64(9 << 20)
	cacheFile, err := os.Create(cachePath)
	if err != nil {
		t.Fatalf("create cache: %v", err)
	}
	if err := cacheFile.Truncate(cacheSize); err != nil {
		cacheFile.Close()
		t.Fatalf("truncate cache: %v", err)
	}
	if err := cacheFile.Close(); err != nil {
		t.Fatalf("close cache: %v", err)
	}

	report, err := Run(Config{Providers: []string{"unity"}})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if report.TotalArtifacts != 1 {
		t.Fatalf("expected 1 artifact, got %d", report.TotalArtifacts)
	}
	artifact := report.Artifacts[0]
	if artifact.Path != libraryRoot {
		t.Fatalf("expected nested Library path %q, got %q", libraryRoot, artifact.Path)
	}
	if artifact.Name != "SampleGame / Library" {
		t.Fatalf("expected nested project cache name, got %q", artifact.Name)
	}
}

func TestRunNamesAppleSimulatorDeviceFromPlist(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	deviceRoot := filepath.Join(tmp, "Library", "Developer", "CoreSimulator", "Devices", "11111111-2222-3333-4444-555555555555")
	if err := os.MkdirAll(deviceRoot, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	plist := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
<key>name</key><string>iPhone 17 Pro</string>
<key>runtime</key><string>com.apple.CoreSimulator.SimRuntime.iOS-26-2</string>
<key>UDID</key><string>11111111-2222-3333-4444-555555555555</string>
</dict></plist>`
	if err := os.WriteFile(filepath.Join(deviceRoot, "device.plist"), []byte(plist), 0o644); err != nil {
		t.Fatalf("write plist: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(deviceRoot, "data"), 0o755); err != nil {
		t.Fatalf("mkdir data: %v", err)
	}
	if err := os.WriteFile(filepath.Join(deviceRoot, "data", "payload"), []byte("sim-data"), 0o644); err != nil {
		t.Fatalf("write data: %v", err)
	}

	report, err := Run(Config{Providers: []string{"apple-simulators"}})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if report.TotalArtifacts != 1 {
		t.Fatalf("expected 1 artifact, got %d", report.TotalArtifacts)
	}
	artifact := report.Artifacts[0]
	if artifact.Name != "iPhone 17 Pro (iOS 26.2)" {
		t.Fatalf("expected plist-derived simulator name, got %q", artifact.Name)
	}
	if artifact.Path != deviceRoot {
		t.Fatalf("expected artifact path to be the device directory, got %q", artifact.Path)
	}
}
