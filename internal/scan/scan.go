package scan

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"weightless/internal/providers"
)

var knownModelExtensions = map[string]struct{}{
	".bin":             {},
	".ckpt":            {},
	".ckpt-tensordata": {},
	".engine":          {},
	".ggml":            {},
	".gguf":            {},
	".mlmodelc":        {},
	".onnx":            {},
	".ot":              {},
	".param":           {},
	".pdparams":        {},
	".pte":             {},
	".pt":              {},
	".pth":             {},
	".safetensors":     {},
	".tflite":          {},
}

type Config struct {
	MinSizeBytes      int64
	AdditionalRoots   []string
	Providers         []string
	IncludeHiddenDirs bool
	Progress          func(string)
}

type Match struct {
	Provider string `json:"provider"`
	Location string `json:"location"`
	Path     string `json:"path"`
}

type Artifact struct {
	Name             string   `json:"name"`
	FileName         string   `json:"file_name"`
	ModelName        string   `json:"model_name,omitempty"`
	Status           string   `json:"status"`
	PrimaryProvider  string   `json:"primary_provider"`
	PrimaryLocation  string   `json:"primary_location"`
	Path             string   `json:"path"`
	CanonicalPath    string   `json:"canonical_path"`
	SizeBytes        int64    `json:"size_bytes"`
	SizeHuman        string   `json:"size_human"`
	Timestamp        string   `json:"timestamp,omitempty"`
	FileCount        int      `json:"file_count,omitempty"`
	Matches          []Match  `json:"matches"`
	AllPaths         []string `json:"all_paths"`
	OwnerReferences  []string `json:"owners,omitempty"`
	Notes            []string `json:"notes,omitempty"`
	CanonicalMissing bool     `json:"canonical_missing,omitempty"`
}

type ProviderSummary struct {
	Provider            string   `json:"provider"`
	Artifacts           int      `json:"artifacts"`
	CompleteArtifacts   int      `json:"complete_artifacts"`
	IncompleteArtifacts int      `json:"incomplete_artifacts"`
	Bytes               int64    `json:"size_bytes"`
	BytesHuman          string   `json:"size_human"`
	Examples            []string `json:"examples,omitempty"`
}

type LocationSummary struct {
	Provider   string `json:"provider"`
	Name       string `json:"name"`
	Root       string `json:"root"`
	Exists     bool   `json:"exists"`
	Bytes      int64  `json:"size_bytes"`
	BytesHuman string `json:"size_human"`
}

type Report struct {
	GeneratedAt       time.Time         `json:"generated_at"`
	Host              string            `json:"host"`
	WorkingDir        string            `json:"working_dir"`
	Summary           []ProviderSummary `json:"summary"`
	Artifacts         []Artifact        `json:"artifacts"`
	Locations         []LocationSummary `json:"locations"`
	ProviderSummaries []ProviderSummary `json:"provider_summaries,omitempty"`
	LocationSummaries []LocationSummary `json:"location_summaries,omitempty"`
	TotalArtifacts    int               `json:"total_artifacts"`
	TotalBytes        int64             `json:"total_bytes"`
	TotalBytesHuman   string            `json:"total_size_human"`
}

type record struct {
	Artifact
	key string
}

func Run(cfg Config) (Report, error) {
	host, _ := os.Hostname()
	cwd, err := os.Getwd()
	if err != nil {
		return Report{}, err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return Report{}, err
	}

	locationSpecs := filterProviders(providers.Registry(cfg.AdditionalRoots), cfg.Providers)
	locationSummaries := make([]LocationSummary, 0)
	records := map[string]*record{}

	totalProviders := len(locationSpecs)
	for idx, spec := range locationSpecs {
		if cfg.Progress != nil {
			cfg.Progress(fmt.Sprintf("[%d/%d] %s", idx+1, totalProviders, spec.Provider))
		}
		for _, rootTemplate := range spec.Roots {
			roots, ok := expandRoots(rootTemplate, home, cwd)
			if !ok {
				continue
			}
			for _, root := range roots {
				root = filepath.Clean(root)
				info, statErr := os.Stat(root)
				exists := statErr == nil && info.IsDir()
				locationSummaries = append(locationSummaries, LocationSummary{
					Provider: spec.Provider,
					Name:     spec.Name,
					Root:     root,
					Exists:   exists,
				})
				if !exists {
					continue
				}
				if cfg.Progress != nil {
					cfg.Progress(fmt.Sprintf("[%d/%d] %s: %s", idx+1, totalProviders, spec.Provider, root))
				}
				if err := walkRoot(root, spec, records, cfg.Progress); err != nil {
					return Report{}, err
				}
			}
		}
	}

	rawArtifacts := flattenRecords(records)
	enrichOllamaOwners(rawArtifacts, home, cwd)
	deduplicateOwnerRefs(rawArtifacts)
	finalizeArtifacts(rawArtifacts)

	artifacts := aggregateArtifacts(rawArtifacts)
	enrichLlamaCppAttribution(artifacts)
	summary := summarizeProviders(locationSpecs, artifacts)
	report := Report{
		GeneratedAt:       time.Now(),
		Host:              host,
		WorkingDir:        cwd,
		Summary:           summary,
		Artifacts:         artifacts,
		Locations:         locationSummaries,
		ProviderSummaries: summary,
		LocationSummaries: locationSummaries,
		TotalArtifacts:    len(artifacts),
		TotalBytes:        totalBytes(artifacts),
	}
	report.TotalBytesHuman = humanBytes(report.TotalBytes)
	report.Locations = enrichLocationSizes(report.Locations, report.Artifacts)
	report.LocationSummaries = report.Locations
	if cfg.Progress != nil {
		cfg.Progress(fmt.Sprintf("Found %d models across %d providers", report.TotalArtifacts, len(report.Summary)))
	}
	return report, nil
}

func expandPath(value, home, cwd string) (string, bool) {
	if strings.TrimSpace(value) == "" {
		return "", false
	}
	if strings.HasPrefix(value, "~") {
		suffix := strings.TrimPrefix(value, "~")
		suffix = strings.TrimPrefix(suffix, string(filepath.Separator))
		if suffix == "" {
			value = home
		} else {
			value = filepath.Join(home, suffix)
		}
	}
	value = os.ExpandEnv(value)
	if strings.TrimSpace(value) == "" {
		return "", false
	}
	if value == "." {
		return cwd, true
	}
	if strings.Contains(value, "${") {
		return "", false
	}
	if strings.Contains(value, "%") && strings.Contains(value, "APPDATA") {
		return "", false
	}
	return value, true
}

func expandRoots(value, home, cwd string) ([]string, bool) {
	root, ok := expandPath(value, home, cwd)
	if !ok {
		return nil, false
	}
	if hasGlob(root) {
		matches, err := filepath.Glob(root)
		if err != nil {
			return []string{root}, true
		}
		if len(matches) == 0 {
			return []string{root}, true
		}
		return matches, true
	}
	return []string{root}, true
}

func hasGlob(value string) bool {
	return strings.ContainsAny(value, "*?[")
}

func walkRoot(root string, spec providers.LocationSpec, records map[string]*record, progress func(string)) error {
	lastProgress := time.Now()
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if progress != nil && time.Since(lastProgress) >= time.Second {
			rel := path
			if relative, err := filepath.Rel(root, path); err == nil && relative != "." {
				rel = relative
			} else {
				rel = filepath.Base(root)
			}
			progress(fmt.Sprintf("%s: %s", spec.Provider, rel))
			lastProgress = time.Now()
		}
		if d.IsDir() {
			base := d.Name()
			if shouldSkipDir(root, path, base, spec) {
				return filepath.SkipDir
			}
			return nil
		}

		info, err := os.Stat(path)
		if err != nil {
			return nil
		}
		if !candidate(path, info.Size(), spec) {
			return nil
		}

		canonical := canonicalPath(path)
		canonInfo, err := os.Stat(canonical)
		if err != nil {
			return nil
		}
		if canonInfo.IsDir() {
			return nil
		}

		key := canonical
		if key == "" {
			key = path
		}
		if existing, ok := records[key]; ok {
			existing.Matches = append(existing.Matches, Match{
				Provider: spec.Provider,
				Location: spec.Name,
				Path:     path,
			})
			if betterName(path, existing.Name) {
				existing.Name = filepath.Base(path)
				existing.Path = path
				existing.PrimaryProvider = spec.Provider
				existing.PrimaryLocation = spec.Name
			}
			return nil
		}

		records[key] = &record{
			key: key,
			Artifact: Artifact{
				Name:            filepath.Base(path),
				PrimaryProvider: spec.Provider,
				PrimaryLocation: spec.Name,
				Path:            path,
				CanonicalPath:   canonical,
				SizeBytes:       canonInfo.Size(),
				SizeHuman:       humanBytes(canonInfo.Size()),
				Matches: []Match{{
					Provider: spec.Provider,
					Location: spec.Name,
					Path:     path,
				}},
				Notes: nonEmpty(spec.Notes),
			},
		}
		return nil
	})
}

func shouldSkipDir(root, path, base string, spec providers.LocationSpec) bool {
	if base == ".git" || base == "node_modules" || base == ".venv" || base == "venv" || base == "__pycache__" {
		return true
	}

	if spec.Provider != "project-local" {
		return false
	}

	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	segments := strings.Split(filepath.ToSlash(rel), "/")
	depth := len(segments)
	if rel == "." || depth <= 2 {
		return false
	}

	for _, segment := range segments {
		if likelyModelSegment(segment) {
			return false
		}
	}
	return true
}

func likelyModelSegment(value string) bool {
	lower := strings.ToLower(value)
	needles := []string{
		"model",
		"models",
		"checkpoint",
		"checkpoints",
		"weight",
		"weights",
		"gguf",
		"ggml",
		"huggingface",
		"transformers",
		"diffusers",
		"mlx",
		"ollama",
		"lmstudio",
		"unsloth",
		"lora",
		"loras",
		"embedding",
		"embeddings",
		"cache",
		"caches",
	}
	for _, needle := range needles {
		if strings.Contains(lower, needle) {
			return true
		}
	}
	return false
}

func candidate(path string, size int64, spec providers.LocationSpec) bool {
	if size < spec.MinSizeBytes {
		return false
	}
	lower := strings.ToLower(filepath.ToSlash(path))
	if strings.HasSuffix(lower, ".incomplete") || strings.HasSuffix(lower, ".partial") || strings.HasSuffix(lower, ".part") {
		return true
	}
	if _, ok := knownModelExtensions[strings.ToLower(filepath.Ext(lower))]; ok {
		return true
	}
	for _, needle := range spec.ForcePathContains {
		if strings.Contains(lower, strings.ToLower(filepath.ToSlash(needle))) {
			return true
		}
	}
	return false
}

func canonicalPath(path string) string {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return path
	}
	return resolved
}

func betterName(candidate, existing string) bool {
	cOpaque := isOpaqueName(filepath.Base(candidate))
	eOpaque := isOpaqueName(filepath.Base(existing))
	if eOpaque && !cOpaque {
		return true
	}
	if cOpaque == eOpaque {
		return len(candidate) < len(existing)
	}
	return false
}

func flattenRecords(records map[string]*record) []Artifact {
	out := make([]Artifact, 0, len(records))
	for _, item := range records {
		sort.Slice(item.Matches, func(i, j int) bool {
			if item.Matches[i].Provider == item.Matches[j].Provider {
				return item.Matches[i].Path < item.Matches[j].Path
			}
			return item.Matches[i].Provider < item.Matches[j].Provider
		})
		out = append(out, item.Artifact)
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].SizeBytes == out[j].SizeBytes {
			if out[i].PrimaryProvider == out[j].PrimaryProvider {
				return out[i].Path < out[j].Path
			}
			return out[i].PrimaryProvider < out[j].PrimaryProvider
		}
		return out[i].SizeBytes > out[j].SizeBytes
	})
	return out
}

func aggregateArtifacts(raw []Artifact) []Artifact {
	type grouped struct {
		key       string
		groupPath string
		modelName string
		items     []Artifact
	}

	order := make([]string, 0)
	groups := map[string]*grouped{}
	for _, artifact := range raw {
		key, groupPath, modelName := aggregationIdentity(artifact)
		group, ok := groups[key]
		if !ok {
			group = &grouped{
				key:       key,
				groupPath: groupPath,
				modelName: modelName,
			}
			groups[key] = group
			order = append(order, key)
		}
		if betterAggregatePath(group.groupPath, groupPath) {
			group.groupPath = groupPath
		}
		if group.modelName == "" && modelName != "" {
			group.modelName = modelName
		}
		group.items = append(group.items, artifact)
	}

	out := make([]Artifact, 0, len(groups))
	for _, key := range order {
		group := groups[key]
		out = append(out, buildAggregateArtifact(group.groupPath, group.modelName, group.items))
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].SizeBytes == out[j].SizeBytes {
			if out[i].PrimaryProvider == out[j].PrimaryProvider {
				return out[i].Path < out[j].Path
			}
			return out[i].PrimaryProvider < out[j].PrimaryProvider
		}
		return out[i].SizeBytes > out[j].SizeBytes
	})
	return out
}

func aggregationIdentity(artifact Artifact) (string, string, string) {
	if key, path, modelName, ok := providerAggregationIdentity(artifact); ok {
		return key, path, modelName
	}
	modelName := artifact.ModelName
	if modelName == "" {
		modelName = displayModelName(inferModelName(artifact))
	}
	key := artifact.PrimaryProvider + "|" + modelName
	return key, artifact.Path, modelName
}

func providerAggregationIdentity(artifact Artifact) (string, string, string, bool) {
	switch artifact.PrimaryProvider {
	case "lm-studio":
		return lmStudioAggregationIdentity(artifact.Path)
	case "anythingllm":
		return anythingLLMAggregationIdentity(artifact.Path)
	case "draw-things":
		return drawThingsAggregationIdentity(artifact.Path)
	case "huggingface":
		return huggingFaceAggregationIdentity(artifact.Path)
	case "project-local":
		return projectLocalAggregationIdentity(artifact.Path)
	}
	if artifact.PrimaryProvider == "ollama" && artifact.ModelName != "" {
		return "ollama|" + artifact.ModelName, artifact.Path, artifact.ModelName, true
	}
	return "", "", "", false
}

func lmStudioAggregationIdentity(path string) (string, string, string, bool) {
	slash := filepath.ToSlash(path)
	needle := "/.lmstudio/models/"
	idx := strings.Index(slash, needle)
	if idx < 0 {
		return "", "", "", false
	}
	rest := slash[idx+len(needle):]
	parts := strings.Split(rest, "/")
	if len(parts) < 2 {
		return "", "", "", false
	}
	groupRel := filepath.Join(parts[0], parts[1])
	groupPath := filepath.Clean(filepath.Join(slash[:idx+len(needle)], groupRel))
	modelName := parts[1]
	return "lm-studio|" + groupPath, filepath.FromSlash(groupPath), modelName, true
}

func anythingLLMAggregationIdentity(path string) (string, string, string, bool) {
	slash := filepath.ToSlash(path)
	needle := "/storage/models/"
	idx := strings.Index(slash, needle)
	if idx < 0 {
		return "", "", "", false
	}
	rest := slash[idx+len(needle):]
	parts := strings.Split(rest, "/")
	if len(parts) == 0 || parts[0] == "" {
		return "", "", "", false
	}
	groupParts := []string{parts[0]}
	modelName := trimKnownExtensions(parts[0])
	if len(parts) >= 2 && parts[1] != "" {
		groupParts = append(groupParts, parts[1])
		modelName = trimKnownExtensions(parts[1])
		if strings.EqualFold(parts[0], "tesseract") {
			modelName = "tesseract " + trimKnownExtensions(parts[1])
		}
	}
	groupRel := filepath.Join(groupParts...)
	groupPath := filepath.Clean(filepath.Join(slash[:idx+len(needle)], groupRel))
	return "anythingllm|" + groupPath, filepath.FromSlash(groupPath), modelName, true
}

func drawThingsAggregationIdentity(path string) (string, string, string, bool) {
	base := drawThingsBasePath(path)
	modelName := trimKnownExtensions(filepath.Base(base))
	return "draw-things|" + base, base, modelName, true
}

func drawThingsBasePath(path string) string {
	if strings.HasSuffix(strings.ToLower(path), ".ckpt-tensordata") {
		candidate := strings.TrimSuffix(path, "-tensordata")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
		return candidate
	}
	return path
}

func huggingFaceAggregationIdentity(path string) (string, string, string, bool) {
	slash := filepath.ToSlash(path)
	if idx := strings.Index(slash, "models--"); idx >= 0 {
		prefix := slash[:idx]
		rest := slash[idx+len("models--"):]
		parts := strings.SplitN(rest, "/", 2)
		repoDir := parts[0]
		groupPath := filepath.Clean(filepath.Join(prefix, "models--"+repoDir))
		modelName := displayModelName(strings.ReplaceAll(repoDir, "--", "/"))
		return "huggingface|" + groupPath, filepath.FromSlash(groupPath), modelName, true
	}
	if idx := strings.Index(slash, "/hub/models/"); idx >= 0 {
		prefix := slash[:idx+len("/hub/models/")]
		rest := slash[idx+len("/hub/models/"):]
		parts := strings.Split(rest, "/")
		if len(parts) < 2 {
			return "", "", "", false
		}
		groupRel := filepath.Join(parts[0], parts[1])
		groupPath := filepath.Clean(filepath.Join(prefix, groupRel))
		return "huggingface|" + groupPath, filepath.FromSlash(groupPath), parts[1], true
	}
	return "", "", "", false
}

func projectLocalAggregationIdentity(path string) (string, string, string, bool) {
	slash := filepath.ToSlash(path)
	for _, marker := range []string{"/models/", "/model/", "/checkpoints/", "/weights/", "/loras/", "/embeddings/"} {
		idx := strings.Index(slash, marker)
		if idx < 0 {
			continue
		}
		prefix := slash[:idx+len(marker)]
		rest := slash[idx+len(marker):]
		parts := strings.Split(rest, "/")
		if len(parts) == 0 || parts[0] == "" {
			return "", "", "", false
		}
		groupParts := []string{parts[0]}
		modelName := trimKnownExtensions(parts[0])
		if len(parts) >= 2 && parts[1] != "" && !looksLikeFile(parts[0]) {
			groupParts = append(groupParts, parts[1])
			modelName = trimKnownExtensions(parts[1])
		}
		groupRel := filepath.Join(groupParts...)
		groupPath := filepath.Clean(filepath.Join(prefix, groupRel))
		return "project-local|" + groupPath, filepath.FromSlash(groupPath), modelName, true
	}
	return "", "", "", false
}

func looksLikeFile(value string) bool {
	base := strings.ToLower(filepath.Base(value))
	if strings.Contains(base, ".") {
		return true
	}
	return false
}

func betterAggregatePath(existing, candidate string) bool {
	if strings.TrimSpace(existing) == "" {
		return true
	}
	if strings.TrimSpace(candidate) == "" {
		return false
	}
	existingInfo, existingErr := os.Stat(existing)
	candidateInfo, candidateErr := os.Stat(candidate)
	if existingErr == nil && candidateErr == nil && existingInfo.IsDir() != candidateInfo.IsDir() {
		return candidateInfo.IsDir()
	}
	if len(candidate) != len(existing) {
		return len(candidate) < len(existing)
	}
	return candidate < existing
}

func buildAggregateArtifact(groupPath, modelName string, items []Artifact) Artifact {
	aggregate := items[0]
	aggregate.Path = groupPath
	aggregate.CanonicalPath = groupPath
	aggregate.FileName = filepath.Base(groupPath)
	aggregate.ModelName = displayModelName(modelName)
	if aggregate.ModelName == "" {
		aggregate.ModelName = displayModelName(inferModelName(aggregate))
	}
	aggregate.Name = aggregate.ModelName
	if aggregate.Name == "" {
		aggregate.Name = trimKnownExtensions(aggregate.FileName)
	}
	aggregate.SizeBytes = 0
	aggregate.FileCount = len(items)
	aggregate.Matches = collectMatches(items)
	aggregate.AllPaths = collectPaths(items)
	aggregate.OwnerReferences = collectOwners(items)
	aggregate.Notes = collectNotes(items)
	aggregate.Status = aggregateStatus(items)
	aggregate.Timestamp = aggregateTimestamp(groupPath, items)
	aggregate.CanonicalMissing = false
	for _, item := range items {
		aggregate.SizeBytes += item.SizeBytes
	}
	aggregate.SizeHuman = humanBytes(aggregate.SizeBytes)
	applyProviderNameOverrides(&aggregate)
	return aggregate
}

func aggregateTimestamp(groupPath string, items []Artifact) string {
	if timestamp := inferTimestamp(groupPath); timestamp != "" {
		return timestamp
	}
	var earliest time.Time
	found := false
	for _, item := range items {
		current := item.Timestamp
		if current == "" {
			current = inferTimestamp(item.Path)
		}
		if current == "" {
			continue
		}
		parsed, err := time.Parse(time.RFC3339, current)
		if err != nil {
			continue
		}
		if !found || parsed.Before(earliest) {
			earliest = parsed
			found = true
		}
	}
	if !found {
		return ""
	}
	return earliest.Format(time.RFC3339)
}

func collectMatches(artifacts []Artifact) []Match {
	seen := map[string]struct{}{}
	out := make([]Match, 0)
	for _, artifact := range artifacts {
		for _, match := range artifact.Matches {
			key := match.Provider + "\x00" + match.Location + "\x00" + match.Path
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			out = append(out, match)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Provider == out[j].Provider {
			if out[i].Location == out[j].Location {
				return out[i].Path < out[j].Path
			}
			return out[i].Location < out[j].Location
		}
		return out[i].Provider < out[j].Provider
	})
	return out
}

func collectNotes(artifacts []Artifact) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0)
	for _, artifact := range artifacts {
		for _, note := range artifact.Notes {
			if _, ok := seen[note]; ok {
				continue
			}
			seen[note] = struct{}{}
			out = append(out, note)
		}
	}
	sort.Strings(out)
	return out
}

func aggregateStatus(artifacts []Artifact) string {
	hasComplete := false
	hasIncomplete := false
	for _, artifact := range artifacts {
		switch artifact.Status {
		case "complete":
			hasComplete = true
		default:
			hasIncomplete = true
		}
	}
	switch {
	case hasComplete && hasIncomplete:
		return "mixed"
	case hasIncomplete:
		return "incomplete"
	default:
		return "complete"
	}
}

func enrichLlamaCppAttribution(artifacts []Artifact) {
	repos := llamaCppCachedRepos()
	if len(repos) == 0 {
		return
	}
	for idx := range artifacts {
		if artifacts[idx].PrimaryProvider != "huggingface" {
			continue
		}
		if !artifactMatchesAnyRepo(artifacts[idx], repos) {
			continue
		}
		artifacts[idx].PrimaryProvider = "llama.cpp"
		artifacts[idx].PrimaryLocation = "llama-server shared cache"
		artifacts[idx].Notes = appendUniqueString(artifacts[idx].Notes, "Shared Hugging Face cache referenced by llama-server.")
		artifacts[idx].OwnerReferences = appendUniqueString(artifacts[idx].OwnerReferences, "llama-server")
	}
}

func llamaCppCachedRepos() []string {
	binary := ""
	for _, candidate := range []string{"llama-server", "/opt/homebrew/bin/llama-server", "/usr/local/bin/llama-server"} {
		if candidate == "llama-server" {
			path, err := exec.LookPath(candidate)
			if err == nil {
				binary = path
				break
			}
			continue
		}
		if _, err := os.Stat(candidate); err == nil {
			binary = candidate
			break
		}
	}
	if binary == "" {
		return nil
	}
	out, err := exec.Command(binary, "--cache-list").CombinedOutput()
	if err != nil && len(out) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	repos := make([]string, 0)
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		dot := strings.Index(line, ". ")
		if dot < 0 {
			continue
		}
		entry := strings.TrimSpace(line[dot+2:])
		if !strings.Contains(entry, "/") {
			continue
		}
		repo := entry
		if colon := strings.LastIndex(repo, ":"); colon > strings.Index(repo, "/") {
			repo = repo[:colon]
		}
		if _, ok := seen[repo]; ok {
			continue
		}
		seen[repo] = struct{}{}
		repos = append(repos, repo)
	}
	sort.Strings(repos)
	return repos
}

func artifactMatchesAnyRepo(artifact Artifact, repos []string) bool {
	for _, repo := range repos {
		if artifactMatchesRepo(artifact, repo) {
			return true
		}
	}
	return false
}

func artifactMatchesRepo(artifact Artifact, repo string) bool {
	repoPathToken := "models--" + strings.ReplaceAll(repo, "/", "--")
	legacyToken := "/" + repo + "/"
	candidates := append([]string{artifact.Path, artifact.CanonicalPath}, artifact.AllPaths...)
	for _, candidate := range candidates {
		slash := filepath.ToSlash(candidate)
		if strings.Contains(slash, repoPathToken) || strings.Contains(slash, legacyToken) {
			return true
		}
	}
	return normalizedContains(artifact.ModelName, displayModelName(repo))
}

func appendUniqueString(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func isOpaqueName(name string) bool {
	lower := strings.ToLower(name)
	if strings.HasPrefix(lower, "sha256-") || strings.HasSuffix(lower, ".part") || strings.HasSuffix(lower, ".partial") || strings.HasSuffix(lower, ".incomplete") {
		return true
	}
	base := strings.TrimSuffix(lower, filepath.Ext(lower))
	if len(base) < 24 {
		return false
	}
	for _, ch := range base {
		if (ch < '0' || ch > '9') && (ch < 'a' || ch > 'f') {
			return false
		}
	}
	return true
}

func summarizeProviders(specs []providers.LocationSpec, artifacts []Artifact) []ProviderSummary {
	type acc struct {
		count      int
		complete   int
		incomplete int
		bytes      int64
		examples   []string
	}
	m := map[string]acc{}
	for _, spec := range specs {
		if _, ok := m[spec.Provider]; !ok {
			m[spec.Provider] = acc{}
		}
	}
	for _, artifact := range artifacts {
		current := m[artifact.PrimaryProvider]
		current.count++
		if artifact.Status == "complete" {
			current.complete++
		} else {
			current.incomplete++
		}
		current.bytes += artifact.SizeBytes
		if len(current.examples) < 3 {
			current.examples = append(current.examples, artifact.Name)
		}
		m[artifact.PrimaryProvider] = current
	}

	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	out := make([]ProviderSummary, 0, len(keys))
	for _, key := range keys {
		value := m[key]
		if value.count == 0 {
			continue
		}
		out = append(out, ProviderSummary{
			Provider:            key,
			Artifacts:           value.count,
			CompleteArtifacts:   value.complete,
			IncompleteArtifacts: value.incomplete,
			Bytes:               value.bytes,
			BytesHuman:          humanBytes(value.bytes),
			Examples:            value.examples,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Bytes == out[j].Bytes {
			return out[i].Provider < out[j].Provider
		}
		return out[i].Bytes > out[j].Bytes
	})
	return out
}

func totalBytes(artifacts []Artifact) int64 {
	var total int64
	for _, artifact := range artifacts {
		total += artifact.SizeBytes
	}
	return total
}

func humanBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func nonEmpty(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return []string{value}
}

type ollamaManifest struct {
	Config struct {
		Digest string `json:"digest"`
	} `json:"config"`
	Layers []struct {
		Digest string `json:"digest"`
	} `json:"layers"`
}

func enrichOllamaOwners(artifacts []Artifact, home, cwd string) {
	roots := []string{
		filepath.Join(home, ".ollama", "models", "manifests"),
	}
	digestOwners := map[string][]string{}
	for _, root := range roots {
		_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			name, manifest, decodeErr := readOllamaManifest(path, root)
			if decodeErr != nil {
				return nil
			}
			if manifest.Config.Digest != "" {
				digestOwners[manifest.Config.Digest] = append(digestOwners[manifest.Config.Digest], name)
			}
			for _, layer := range manifest.Layers {
				if layer.Digest != "" {
					digestOwners[layer.Digest] = append(digestOwners[layer.Digest], name)
				}
			}
			return nil
		})
	}

	for idx := range artifacts {
		base := filepath.Base(artifacts[idx].CanonicalPath)
		if !strings.HasPrefix(base, "sha256-") {
			continue
		}
		digest := strings.Replace(base, "sha256-", "sha256:", 1)
		if owners, ok := digestOwners[digest]; ok {
			artifacts[idx].OwnerReferences = append(artifacts[idx].OwnerReferences, owners...)
		}
	}
}

func readOllamaManifest(path, root string) (string, ollamaManifest, error) {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return "", ollamaManifest{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", ollamaManifest{}, err
	}
	var manifest ollamaManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return "", ollamaManifest{}, err
	}
	return filepath.ToSlash(rel), manifest, nil
}

func deduplicateOwnerRefs(artifacts []Artifact) {
	for idx := range artifacts {
		if len(artifacts[idx].OwnerReferences) == 0 {
			continue
		}
		sort.Strings(artifacts[idx].OwnerReferences)
		filtered := artifacts[idx].OwnerReferences[:0]
		var prev string
		for _, owner := range artifacts[idx].OwnerReferences {
			if owner == prev {
				continue
			}
			filtered = append(filtered, owner)
			prev = owner
		}
		artifacts[idx].OwnerReferences = filtered
	}
}

func finalizeArtifacts(artifacts []Artifact) {
	for idx := range artifacts {
		artifacts[idx].OwnerReferences = humanizeOwners(artifacts[idx].OwnerReferences)
		artifacts[idx].AllPaths = collectPaths([]Artifact{artifacts[idx]})
		artifacts[idx].Path = preferredPath(artifacts[idx])
		artifacts[idx].FileName = filepath.Base(artifacts[idx].Path)
		artifacts[idx].Status = inferStatus(artifacts[idx])
		artifacts[idx].ModelName = displayModelName(inferModelName(artifacts[idx]))
		artifacts[idx].Name = artifacts[idx].ModelName
		artifacts[idx].Timestamp = inferTimestamp(artifacts[idx].Path)
		if artifacts[idx].Name == "" {
			artifacts[idx].Name = trimKnownExtensions(artifacts[idx].FileName)
		}
		applyProviderNameOverrides(&artifacts[idx])
	}
}

func applyProviderNameOverrides(artifact *Artifact) {
	switch artifact.PrimaryProvider {
	case "ollama":
		if artifact.ModelName != "" {
			artifact.Name = artifact.ModelName
		}
	case "chrome-built-in-ai":
		artifact.ModelName = "Chrome Built-in AI"
		artifact.Name = artifact.ModelName
	case "huggingface":
		if artifact.Status == "incomplete" {
			artifact.Name = artifact.ModelName
		}
	}
}

func preferredPath(artifact Artifact) string {
	best := artifact.Path
	for _, match := range artifact.Matches {
		if betterName(match.Path, best) {
			best = match.Path
		}
	}
	return best
}

func inferStatus(artifact Artifact) string {
	lower := strings.ToLower(artifact.Path)
	if strings.HasSuffix(lower, ".incomplete") || strings.HasSuffix(lower, ".partial") || strings.HasSuffix(lower, ".part") {
		return "incomplete"
	}
	return "complete"
}

func inferModelName(artifact Artifact) string {
	if len(artifact.OwnerReferences) > 0 {
		return artifact.OwnerReferences[0]
	}
	for _, path := range artifact.AllPaths {
		if model := modelNameFromPath(path); model != "" {
			return model
		}
	}
	return trimKnownExtensions(filepath.Base(artifact.Path))
}

func inferReadableName(artifact Artifact) string {
	model := displayModelName(artifact.ModelName)
	label := humanLeafLabel(artifact)
	if shard := shardLabel(artifact.FileName); shard != "" {
		label = shard
	}
	if model == "" {
		model = trimKnownExtensions(artifact.FileName)
	}
	if label == "" || label == model {
		if artifact.Status == "incomplete" {
			return model + " (incomplete)"
		}
		return model
	}
	if artifact.Status == "incomplete" {
		return model + " / " + label + " (incomplete)"
	}
	return model + " / " + label
}

func humanLeafLabel(artifact Artifact) string {
	label := bestLeafLabel(artifact)
	switch artifact.PrimaryProvider {
	case "ollama":
		return "model blob"
	case "huggingface":
		if isOpaqueName(filepath.Base(label)) && artifact.Status == "incomplete" {
			return "incomplete blob"
		}
		if isOpaqueName(filepath.Base(label)) {
			return "blob"
		}
		return simplifyLabel(artifact, label)
	case "chrome-built-in-ai":
		return "component " + shortID(filepath.Base(artifact.Path))
	default:
		return simplifyLabel(artifact, label)
	}
}

func bestLeafLabel(artifact Artifact) string {
	for _, path := range artifact.AllPaths {
		if label := leafLabel(path); label != "" && !isOpaqueName(filepath.Base(label)) {
			return label
		}
	}
	if label := leafLabel(artifact.Path); label != "" {
		return label
	}
	return artifact.FileName
}

func shortID(value string) string {
	if len(value) <= 12 {
		return value
	}
	return value[:12]
}

func displayModelName(value string) string {
	if value == "" {
		return ""
	}
	if strings.Contains(value, ":") {
		return value
	}
	parts := strings.Split(value, "/")
	return parts[len(parts)-1]
}

func simplifyLabel(artifact Artifact, label string) string {
	label = filepath.ToSlash(label)
	base := filepath.Base(label)
	shortModel := displayModelName(artifact.ModelName)
	if shard := shardLabel(base); shard != "" {
		return shard
	}
	if isGenericWeightName(base) {
		dir := filepath.Base(filepath.Dir(label))
		if dir != "." && dir != "/" && dir != "" && dir != base {
			return dir + "/" + base
		}
	}
	if normalizedContains(base, shortModel) {
		return base
	}
	if normalizedContains(trimKnownExtensions(base), trimKnownExtensions(shortModel)) {
		return base
	}
	return base
}

func shardLabel(name string) string {
	name = trimKnownExtensions(name)
	if strings.HasPrefix(name, "model-") && strings.Contains(name, "-of-") {
		rest := strings.TrimPrefix(name, "model-")
		parts := strings.Split(rest, "-of-")
		if len(parts) == 2 {
			return "shard " + parts[0] + "/" + parts[1]
		}
	}
	return ""
}

func isGenericWeightName(name string) bool {
	switch name {
	case "weights.safetensors", "model.safetensors", "diffusion_pytorch_model.safetensors", "tokenizer.json", "merges.txt", "vocab.json", "model_quantized.onnx", "encoder_model_quantized.onnx", "decoder_model_merged_quantized.onnx":
		return true
	default:
		return false
	}
}

func normalizedContains(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	normalize := func(v string) string {
		v = strings.ToLower(v)
		replacer := strings.NewReplacer("-", "", "_", "", ".", "", " ", "")
		return replacer.Replace(v)
	}
	return strings.Contains(normalize(a), normalize(b))
}

func leafLabel(path string) string {
	slash := filepath.ToSlash(path)
	switch {
	case strings.Contains(slash, "/snapshots/"):
		parts := strings.SplitN(slash, "/snapshots/", 2)
		rest := parts[1]
		items := strings.SplitN(rest, "/", 2)
		if len(items) == 2 {
			return items[1]
		}
	case strings.Contains(slash, "/storage/models/"):
		parts := strings.SplitN(slash, "/storage/models/", 2)
		return parts[1]
	case strings.Contains(slash, "/.lmstudio/models/"):
		parts := strings.SplitN(slash, "/.lmstudio/models/", 2)
		items := strings.Split(parts[1], "/")
		if len(items) >= 3 {
			return strings.Join(items[2:], "/")
		}
	case strings.Contains(slash, "/Data/Documents/Models/"):
		parts := strings.SplitN(slash, "/Data/Documents/Models/", 2)
		return parts[1]
	}
	return filepath.Base(path)
}

func modelNameFromPath(path string) string {
	slash := filepath.ToSlash(path)
	if idx := strings.Index(slash, "models--"); idx >= 0 {
		rest := slash[idx+len("models--"):]
		parts := strings.SplitN(rest, "/", 2)
		repo := strings.ReplaceAll(parts[0], "--", "/")
		return repo
	}
	if idx := strings.Index(slash, "/hub/models/"); idx >= 0 {
		rest := slash[idx+len("/hub/models/"):]
		parts := strings.Split(rest, "/")
		if len(parts) >= 2 {
			return parts[0] + "/" + parts[1]
		}
	}
	if idx := strings.Index(slash, "/.lmstudio/models/"); idx >= 0 {
		rest := slash[idx+len("/.lmstudio/models/"):]
		parts := strings.Split(rest, "/")
		if len(parts) >= 2 {
			return parts[0] + "/" + parts[1]
		}
	}
	if idx := strings.Index(slash, "/storage/models/"); idx >= 0 {
		rest := slash[idx+len("/storage/models/"):]
		parts := strings.Split(rest, "/")
		if len(parts) >= 2 {
			return parts[0] + "/" + parts[1]
		}
	}
	if idx := strings.Index(slash, "/Data/Documents/Models/"); idx >= 0 {
		rest := slash[idx+len("/Data/Documents/Models/"):]
		parts := strings.Split(rest, "/")
		if len(parts) >= 1 {
			return trimKnownExtensions(parts[0])
		}
	}
	if idx := strings.Index(slash, "/models/"); idx >= 0 {
		rest := slash[idx+len("/models/"):]
		parts := strings.Split(rest, "/")
		if len(parts) >= 2 {
			return parts[0] + "/" + parts[1]
		}
		if len(parts) == 1 {
			return trimKnownExtensions(parts[0])
		}
	}
	return ""
}

func trimKnownExtensions(name string) string {
	exts := []string{
		".ckpt-tensordata",
		".safetensors",
		".traineddata",
		".mlmodelc",
		".incomplete",
		".partial",
		".gguf",
		".ggml",
		".ckpt",
		".onnx",
		".json",
		".bin",
	}
	out := name
	for _, ext := range exts {
		out = strings.TrimSuffix(out, ext)
	}
	return out
}

func inferTimestamp(path string) string {
	timestamp, ok := fileTimestamp(path)
	if !ok {
		return ""
	}
	return timestamp.Format(time.RFC3339)
}

func fileTimestamp(path string) (time.Time, bool) {
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}, false
	}
	modifiedAt := info.ModTime()
	if modifiedAt.IsZero() {
		return time.Time{}, false
	}
	return modifiedAt, true
}

func humanizeOwners(owners []string) []string {
	out := make([]string, 0, len(owners))
	for _, owner := range owners {
		if strings.HasPrefix(owner, "registry.ollama.ai/library/") {
			trimmed := strings.TrimPrefix(owner, "registry.ollama.ai/library/")
			parts := strings.Split(trimmed, "/")
			if len(parts) >= 2 {
				out = append(out, parts[0]+":"+parts[1])
				continue
			}
		}
		out = append(out, owner)
	}
	return out
}

func collectOwners(artifacts []Artifact) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0)
	for _, artifact := range artifacts {
		for _, owner := range artifact.OwnerReferences {
			if _, ok := seen[owner]; ok {
				continue
			}
			seen[owner] = struct{}{}
			out = append(out, owner)
		}
		if artifact.ModelName != "" {
			if _, ok := seen[artifact.ModelName]; !ok {
				seen[artifact.ModelName] = struct{}{}
				out = append(out, artifact.ModelName)
			}
		}
	}
	sort.Strings(out)
	return out
}

func collectPaths(artifacts []Artifact) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0)
	for _, artifact := range artifacts {
		for _, match := range artifact.Matches {
			if _, ok := seen[match.Path]; ok {
				continue
			}
			seen[match.Path] = struct{}{}
			out = append(out, match.Path)
		}
	}
	sort.Strings(out)
	return out
}

func filterProviders(specs []providers.LocationSpec, filters []string) []providers.LocationSpec {
	if len(filters) == 0 {
		return specs
	}
	allowed := map[string]struct{}{}
	for _, value := range filters {
		value = strings.TrimSpace(strings.ToLower(value))
		if value != "" {
			allowed[value] = struct{}{}
		}
	}
	out := make([]providers.LocationSpec, 0, len(specs))
	for _, spec := range specs {
		if _, ok := allowed[strings.ToLower(spec.Provider)]; ok {
			out = append(out, spec)
		}
	}
	return out
}

func enrichLocationSizes(locations []LocationSummary, artifacts []Artifact) []LocationSummary {
	for idx := range locations {
		if !locations[idx].Exists {
			continue
		}
		root := filepath.Clean(locations[idx].Root)
		for _, artifact := range artifacts {
			if artifactInRoot(artifact, root) {
				locations[idx].Bytes += artifact.SizeBytes
			}
		}
		locations[idx].BytesHuman = humanBytes(locations[idx].Bytes)
	}
	sort.Slice(locations, func(i, j int) bool {
		if locations[i].Bytes == locations[j].Bytes {
			if locations[i].Provider == locations[j].Provider {
				return locations[i].Root < locations[j].Root
			}
			return locations[i].Provider < locations[j].Provider
		}
		return locations[i].Bytes > locations[j].Bytes
	})
	return locations
}

func artifactInRoot(artifact Artifact, root string) bool {
	for _, match := range artifact.Matches {
		clean := filepath.Clean(match.Path)
		if clean == root || strings.HasPrefix(clean, root+string(filepath.Separator)) {
			return true
		}
	}
	return false
}
