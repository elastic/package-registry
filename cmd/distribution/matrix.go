// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	"gopkg.in/yaml.v3"
)

const (
	defaultArtifactsURL = "https://storage.googleapis.com/artifacts-api/snapshots"
	defaultKibanaRawURL = "https://raw.githubusercontent.com/elastic/kibana"
)

var (
	reSpecMin = regexp.MustCompile(`REGISTRY_SPEC_MIN_VERSION\s*=\s*['"]([^'"]+)['"]`)
	reSpecMax = regexp.MustCompile(`REGISTRY_SPEC_MAX_VERSION\s*=\s*['"]([^'"]+)['"]`)
)

// matrixSource holds the base URLs for the artifacts API and the Kibana
// source repository. Both are fields so tests can point them at httptest servers.
type matrixSource struct {
	artifactsURL string
	kibanaRawURL string
	client       *http.Client
}

// specBounds holds the Fleet package spec version constraints for a Kibana branch.
// min is empty when the branch does not declare REGISTRY_SPEC_MIN_VERSION.
type specBounds struct {
	min string
	max string
}

// activeBranches returns the list of active Elastic build branches from the
// artifacts API (e.g. ["main", "9.5", "9.4", "8.19"]).
func (s matrixSource) activeBranches(ctx context.Context) ([]string, error) {
	u := s.artifactsURL + "/branches.json"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("building request for %s: %w", u, err)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching %s: %w", u, err)
	}
	defer drainAndClose(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetching %s: status %d", u, resp.StatusCode)
	}
	var body struct {
		Branches []string `json:"branches"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("decoding %s: %w", u, err)
	}
	return body.Branches, nil
}

// nextVersion returns the next planned version for a branch with any pre-release
// suffix (e.g. "-SNAPSHOT") stripped.
func (s matrixSource) nextVersion(ctx context.Context, branch string) (*semver.Version, error) {
	u := s.artifactsURL + "/" + branch + ".json"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("building request for %s: %w", u, err)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching %s: %w", u, err)
	}
	defer drainAndClose(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetching %s: status %d", u, resp.StatusCode)
	}
	var body struct {
		Version string `json:"version"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("decoding %s: %w", u, err)
	}
	// Strip -SNAPSHOT and any other pre-release suffix.
	versionStr := body.Version
	if idx := strings.IndexByte(versionStr, '-'); idx >= 0 {
		versionStr = versionStr[:idx]
	}
	v, err := semver.NewVersion(versionStr)
	if err != nil {
		return nil, fmt.Errorf("parsing version %q from %s: %w", body.Version, u, err)
	}
	return v, nil
}

// fetchSpecBounds returns the Fleet spec version constraints for a Kibana branch
// by parsing the Fleet server config.ts. Missing SPEC_MIN_VERSION is allowed
// (some branches only declare MAX). Missing SPEC_MAX_VERSION is a hard error.
func (s matrixSource) fetchSpecBounds(ctx context.Context, branch string) (specBounds, error) {
	u := s.kibanaRawURL + "/" + branch + "/x-pack/platform/plugins/shared/fleet/server/config.ts"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return specBounds{}, fmt.Errorf("building request for %s: %w", u, err)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return specBounds{}, fmt.Errorf("fetching %s: %w", u, err)
	}
	defer drainAndClose(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return specBounds{}, fmt.Errorf("fetching %s: status %d", u, resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return specBounds{}, fmt.Errorf("reading %s: %w", u, err)
	}
	maxMatch := reSpecMax.FindSubmatch(body)
	if maxMatch == nil {
		return specBounds{}, fmt.Errorf("REGISTRY_SPEC_MAX_VERSION not found in %s", u)
	}
	bounds := specBounds{max: string(maxMatch[1])}
	if minMatch := reSpecMin.FindSubmatch(body); minMatch != nil {
		bounds.min = string(minMatch[1])
	}
	return bounds, nil
}

// generateEntries returns one configQuery per patch version from X.Y.0 through
// next (inclusive), all with the given spec bounds.
func generateEntries(next *semver.Version, bounds specBounds) []configQuery {
	major := next.Major()
	minor := next.Minor()
	patch := next.Patch()
	entries := make([]configQuery, 0, int(patch)+1)
	for p := uint64(0); p <= patch; p++ {
		entries = append(entries, configQuery{
			KibanaVersion: fmt.Sprintf("%d.%d.%d", major, minor, p),
			SpecMin:       bounds.min,
			SpecMax:       bounds.max,
		})
	}
	return entries
}

// mergeMatrix merges generated entries into existing ones:
//   - New versions (not in existing) are added with the generated spec bounds.
//   - Versions in nextVersions have their spec bounds refreshed from generated,
//     because they are still unreleased and their spec bounds may change before
//     they ship. Released versions are never modified.
//   - Spec-only entries (no kibana.version) are always kept at the end.
//
// The returned slice is sorted by kibana.version (semver).
func mergeMatrix(existing, generated []configQuery, nextVersions map[string]bool) []configQuery {
	// Index generated entries by kibana.version for quick lookup.
	byVersion := make(map[string]configQuery, len(generated))
	for _, g := range generated {
		if g.KibanaVersion != "" {
			byVersion[g.KibanaVersion] = g
		}
	}

	seen := make(map[string]bool, len(existing))
	result := make([]configQuery, 0, len(existing))
	for _, e := range existing {
		if e.KibanaVersion != "" {
			seen[e.KibanaVersion] = true
		}
		// For unreleased ("next") versions, refresh spec bounds so the entry
		// stays current with whatever the branch has today.
		if e.KibanaVersion != "" && nextVersions[e.KibanaVersion] {
			if g, ok := byVersion[e.KibanaVersion]; ok {
				e.SpecMin = g.SpecMin
				e.SpecMax = g.SpecMax
			}
		}
		result = append(result, e)
	}
	for _, g := range generated {
		if g.KibanaVersion != "" && !seen[g.KibanaVersion] {
			result = append(result, g)
			seen[g.KibanaVersion] = true
		}
	}
	// Stable sort so that equal entries keep their original relative order.
	// Spec-only entries (no kibana.version) sort to the end.
	slices.SortStableFunc(result, func(a, b configQuery) int {
		switch {
		case a.KibanaVersion == "" && b.KibanaVersion == "":
			return 0
		case a.KibanaVersion == "":
			return 1
		case b.KibanaVersion == "":
			return -1
		default:
			return compareVersions(a.KibanaVersion, b.KibanaVersion)
		}
	})
	return result
}

// renderMatrixBlock formats a slice of configQuery values as YAML list items
// with a 2-space indent, matching the format used in existing config files.
func renderMatrixBlock(entries []configQuery) []byte {
	var buf bytes.Buffer
	for _, e := range entries {
		first := true
		writeField := func(k, v string) {
			if first {
				fmt.Fprintf(&buf, "  - %s: %s\n", k, v)
				first = false
			} else {
				fmt.Fprintf(&buf, "    %s: %s\n", k, v)
			}
		}
		if e.KibanaVersion != "" {
			writeField("kibana.version", e.KibanaVersion)
		}
		if e.SpecMin != "" {
			writeField("spec.min", e.SpecMin)
		}
		if e.SpecMax != "" {
			writeField("spec.max", e.SpecMax)
		}
		if e.Package != "" {
			writeField("package", e.Package)
		}
		if e.Type != "" {
			writeField("type", e.Type)
		}
		if e.All {
			writeField("all", "true")
		}
		if e.Prerelease {
			writeField("prerelease", "true")
		}
		if e.VersionLimit != nil {
			writeField("version.limit", fmt.Sprintf("%d", *e.VersionLimit))
		}
	}
	return buf.Bytes()
}

// rewriteMatrix replaces the matrix: block in src with the given entries,
// preserving all content outside the block (header comments, other sections)
// and any comment-only lines at the top of the existing matrix block.
func rewriteMatrix(src []byte, entries []configQuery) ([]byte, error) {
	lines := bytes.Split(src, []byte("\n"))

	// Locate the matrix: key at column 0.
	matrixLine := -1
	for i, line := range lines {
		if bytes.Equal(bytes.TrimRight(line, " \t"), []byte("matrix:")) {
			matrixLine = i
			break
		}
	}
	if matrixLine < 0 {
		return nil, fmt.Errorf("matrix: key not found in config")
	}

	// Find the last content line of the matrix block and the start of the next
	// top-level section. Blank lines between sections are treated as separators
	// and placed after the matrix block in the output.
	lastContentLine := matrixLine
	nextSection := len(lines)
	for i := matrixLine + 1; i < len(lines); i++ {
		line := lines[i]
		if len(line) == 0 {
			continue
		}
		// Any non-indented line begins the next section (including top-level comments).
		if line[0] != ' ' && line[0] != '\t' {
			nextSection = i
			break
		}
		lastContentLine = i
	}

	// Collect preamble: comment-only lines at the top of the matrix block
	// (before the first real entry). These are preserved verbatim.
	var preamble [][]byte
	for i := matrixLine + 1; i <= lastContentLine; i++ {
		trimmed := bytes.TrimSpace(lines[i])
		if len(trimmed) == 0 {
			continue
		}
		if bytes.HasPrefix(trimmed, []byte("#")) {
			preamble = append(preamble, lines[i])
		} else {
			break
		}
	}

	// Render the new matrix block.
	var newBlock [][]byte
	newBlock = append(newBlock, []byte("matrix:"))
	newBlock = append(newBlock, preamble...)
	rendered := renderMatrixBlock(entries)
	// Split on newline; drop the final empty element from the trailing \n.
	renderedLines := bytes.Split(rendered, []byte("\n"))
	if n := len(renderedLines); n > 0 && len(renderedLines[n-1]) == 0 {
		renderedLines = renderedLines[:n-1]
	}
	newBlock = append(newBlock, renderedLines...)

	// Preserve blank lines that separate the matrix block from the next section.
	var separator [][]byte
	for i := lastContentLine + 1; i < nextSection; i++ {
		separator = append(separator, lines[i])
	}

	// Reconstruct the file.
	var result [][]byte
	result = append(result, lines[:matrixLine]...)
	result = append(result, newBlock...)
	result = append(result, separator...)
	result = append(result, lines[nextSection:]...)

	return bytes.Join(result, []byte("\n")), nil
}

// runUpdateMatrix fetches the active Elastic branches and their next planned
// versions, then adds any missing matrix entries to each config file in paths.
func runUpdateMatrix(paths []string) error {
	if len(paths) == 0 {
		return fmt.Errorf("at least one config path required")
	}

	src := matrixSource{
		artifactsURL: defaultArtifactsURL,
		kibanaRawURL: defaultKibanaRawURL,
		client:       &http.Client{Timeout: 60 * time.Second},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	branches, err := src.activeBranches(ctx)
	if err != nil {
		return fmt.Errorf("fetching active branches: %w", err)
	}

	var generated []configQuery
	// nextVersions is the set of kibana.version strings that are currently
	// unreleased (SNAPSHOT). Their spec bounds are refreshed on every run so the
	// matrix stays current even if the branch updates spec.max before shipping.
	nextVersions := make(map[string]bool)
	for _, branch := range branches {
		next, err := src.nextVersion(ctx, branch)
		if err != nil {
			return fmt.Errorf("branch %s: fetching next version: %w", branch, err)
		}
		bounds, err := src.fetchSpecBounds(ctx, branch)
		if err != nil {
			return fmt.Errorf("branch %s: fetching spec bounds: %w", branch, err)
		}
		entries := generateEntries(next, bounds)
		// The last entry is the next (unreleased) version; the preceding ones
		// are already-released patches whose spec bounds must not be overwritten.
		if len(entries) > 0 {
			nextVersions[entries[len(entries)-1].KibanaVersion] = true
		}
		generated = append(generated, entries...)
		fmt.Fprintf(os.Stderr, "branch %s → next %s (spec.min=%s spec.max=%s)\n",
			branch, next.Original(), bounds.min, bounds.max)
	}

	for _, configPath := range paths {
		if err := updateConfigMatrix(configPath, generated, nextVersions); err != nil {
			return fmt.Errorf("%s: %w", configPath, err)
		}
	}
	return nil
}

// updateConfigMatrix merges generated entries into the matrix of a single
// config file, rewriting it in place. It is a no-op when nothing changes.
func updateConfigMatrix(configPath string, generated []configQuery, nextVersions map[string]bool) error {
	rawSrc, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("reading config: %w", err)
	}
	var cfg config
	if err := yaml.Unmarshal(rawSrc, &cfg); err != nil {
		return fmt.Errorf("parsing config: %w", err)
	}

	merged := mergeMatrix(cfg.Matrix, generated, nextVersions)

	updated, err := rewriteMatrix(rawSrc, merged)
	if err != nil {
		return fmt.Errorf("rewriting matrix: %w", err)
	}

	if bytes.Equal(rawSrc, updated) {
		fmt.Fprintf(os.Stderr, "%s: matrix is already up to date\n", configPath)
		return nil
	}

	info, err := os.Stat(configPath)
	if err != nil {
		return fmt.Errorf("stating config: %w", err)
	}
	if err := os.WriteFile(configPath, updated, info.Mode()); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}

	// Count added vs spec-bounds-refreshed entries for the log.
	oldByVersion := make(map[string]configQuery, len(cfg.Matrix))
	for _, e := range cfg.Matrix {
		oldByVersion[e.KibanaVersion] = e
	}
	added, refreshed := 0, 0
	for _, e := range merged {
		old, existed := oldByVersion[e.KibanaVersion]
		switch {
		case !existed:
			added++
		case old.SpecMin != e.SpecMin || old.SpecMax != e.SpecMax:
			refreshed++
		}
	}
	noun := "entries"
	if added+refreshed == 1 {
		noun = "entry"
	}
	fmt.Fprintf(os.Stderr, "%s: added %d, refreshed spec bounds for %d %s (%d total)\n",
		configPath, added, refreshed, noun, len(merged))
	return nil
}
