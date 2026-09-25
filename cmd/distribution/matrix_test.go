// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Masterminds/semver/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── generateEntries ──────────────────────────────────────────────────────────

func TestGenerateEntries(t *testing.T) {
	tests := []struct {
		name      string
		next      string
		bounds    specBounds
		wantFirst string
		wantLast  string
		wantLen   int
	}{
		{
			name:      "patch 0",
			next:      "9.6.0",
			bounds:    specBounds{min: "2.3", max: "3.6"},
			wantFirst: "9.6.0",
			wantLast:  "9.6.0",
			wantLen:   1,
		},
		{
			name:      "multiple patches",
			next:      "9.5.4",
			bounds:    specBounds{min: "2.3", max: "3.6"},
			wantFirst: "9.5.0",
			wantLast:  "9.5.4",
			wantLen:   5,
		},
		{
			name:      "no spec.min",
			next:      "8.19.3",
			bounds:    specBounds{max: "3.4"},
			wantFirst: "8.19.0",
			wantLast:  "8.19.3",
			wantLen:   4,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, err := semver.NewVersion(tt.next)
			require.NoError(t, err)
			entries := generateEntries(v, tt.bounds)
			require.Len(t, entries, tt.wantLen)
			assert.Equal(t, tt.wantFirst, entries[0].KibanaVersion)
			assert.Equal(t, tt.wantLast, entries[tt.wantLen-1].KibanaVersion)
			for _, e := range entries {
				assert.Equal(t, tt.bounds.min, e.SpecMin)
				assert.Equal(t, tt.bounds.max, e.SpecMax)
			}
		})
	}
}

// ─── mergeMatrix ─────────────────────────────────────────────────────────────

func TestMergeMatrixAddsNewEntries(t *testing.T) {
	existing := []configQuery{
		{KibanaVersion: "9.5.0", SpecMin: "2.3", SpecMax: "3.6"},
		{KibanaVersion: "9.5.1", SpecMin: "2.3", SpecMax: "3.6"},
	}
	generated := []configQuery{
		{KibanaVersion: "9.5.0", SpecMin: "2.3", SpecMax: "3.6"},
		{KibanaVersion: "9.5.1", SpecMin: "2.3", SpecMax: "3.6"},
		{KibanaVersion: "9.5.2", SpecMin: "2.3", SpecMax: "3.6"},
	}
	result := mergeMatrix(existing, generated, nil)
	require.Len(t, result, 3)
	assert.Equal(t, "9.5.2", result[2].KibanaVersion)
}

func TestMergeMatrixKeepsSpecBoundsForReleasedVersion(t *testing.T) {
	// Released versions keep historical spec bounds even when generated has different ones.
	existing := []configQuery{
		{KibanaVersion: "9.5.0", SpecMin: "2.3", SpecMax: "3.5"},
	}
	generated := []configQuery{
		{KibanaVersion: "9.5.0", SpecMin: "2.3", SpecMax: "3.6"},
	}
	// 9.5.0 is NOT in nextVersions (it has been released).
	result := mergeMatrix(existing, generated, nil)
	require.Len(t, result, 1)
	assert.Equal(t, "3.5", result[0].SpecMax, "released version spec bounds must not be overwritten")
}

func TestMergeMatrixRefreshesSpecBoundsForNextVersion(t *testing.T) {
	// The "next" (unreleased SNAPSHOT) version has its spec bounds refreshed
	// each run so stale bounds do not persist before the version ships.
	existing := []configQuery{
		{KibanaVersion: "9.5.5", SpecMin: "2.3", SpecMax: "3.5"},
	}
	generated := []configQuery{
		{KibanaVersion: "9.5.5", SpecMin: "2.3", SpecMax: "3.6"},
	}
	result := mergeMatrix(existing, generated, map[string]bool{"9.5.5": true})
	require.Len(t, result, 1)
	assert.Equal(t, "3.6", result[0].SpecMax, "next version spec bounds must be refreshed")
}

func TestMergeMatrixNextVersionBoundsOnlyWhenInSet(t *testing.T) {
	// A version is only refreshed when it appears in nextVersions.
	// Without the set, the existing bounds are kept.
	existing := []configQuery{
		{KibanaVersion: "9.5.5", SpecMin: "2.3", SpecMax: "3.5"},
	}
	generated := []configQuery{
		{KibanaVersion: "9.5.5", SpecMin: "2.3", SpecMax: "3.6"},
	}
	result := mergeMatrix(existing, generated, nil)
	require.Len(t, result, 1)
	assert.Equal(t, "3.5", result[0].SpecMax, "bounds must not change when not in nextVersions")
}

func TestMergeMatrixSortsByVersion(t *testing.T) {
	existing := []configQuery{
		{KibanaVersion: "9.5.1", SpecMin: "2.3", SpecMax: "3.6"},
		{KibanaVersion: "8.19.0", SpecMax: "3.4"},
	}
	generated := []configQuery{
		{KibanaVersion: "9.6.0", SpecMin: "2.3", SpecMax: "3.6"},
	}
	result := mergeMatrix(existing, generated, nil)
	require.Len(t, result, 3)
	assert.Equal(t, "8.19.0", result[0].KibanaVersion)
	assert.Equal(t, "9.5.1", result[1].KibanaVersion)
	assert.Equal(t, "9.6.0", result[2].KibanaVersion)
}

func TestMergeMatrixSpecOnlyEntriesLast(t *testing.T) {
	existing := []configQuery{
		{KibanaVersion: "9.5.0", SpecMin: "2.3", SpecMax: "3.6"},
		{SpecMin: "2.3", SpecMax: "3.6"}, // spec-only, no kibana.version
	}
	generated := []configQuery{
		{KibanaVersion: "9.6.0", SpecMin: "2.3", SpecMax: "3.6"},
	}
	result := mergeMatrix(existing, generated, nil)
	require.Len(t, result, 3)
	assert.Equal(t, "9.5.0", result[0].KibanaVersion)
	assert.Equal(t, "9.6.0", result[1].KibanaVersion)
	assert.Equal(t, "", result[2].KibanaVersion, "spec-only entry must be last")
}

func TestMergeMatrixIsIdempotent(t *testing.T) {
	existing := []configQuery{
		{KibanaVersion: "9.5.0", SpecMin: "2.3", SpecMax: "3.6"},
		{KibanaVersion: "9.5.1", SpecMin: "2.3", SpecMax: "3.6"},
		{SpecMin: "2.3", SpecMax: "3.6"},
	}
	generated := []configQuery{
		{KibanaVersion: "9.5.0", SpecMin: "2.3", SpecMax: "3.6"},
		{KibanaVersion: "9.5.1", SpecMin: "2.3", SpecMax: "3.6"},
	}
	nextVersions := map[string]bool{"9.5.1": true}
	first := mergeMatrix(existing, generated, nextVersions)
	second := mergeMatrix(first, generated, nextVersions)
	assert.Equal(t, first, second)
}

func TestMergeMatrixSkipsSpecOnlyFromGenerated(t *testing.T) {
	existing := []configQuery{
		{KibanaVersion: "9.5.0", SpecMin: "2.3", SpecMax: "3.6"},
	}
	// A spec-only entry in generated (no kibana.version) must not be added.
	generated := []configQuery{
		{SpecMin: "2.3", SpecMax: "3.6"},
		{KibanaVersion: "9.5.1", SpecMin: "2.3", SpecMax: "3.6"},
	}
	result := mergeMatrix(existing, generated, nil)
	for _, e := range result {
		assert.NotEmpty(t, e.KibanaVersion, "generated spec-only entry must not be added")
	}
	assert.Len(t, result, 2)
}

// ─── rewriteMatrix ───────────────────────────────────────────────────────────

const sampleConfig = `# Header comment line 1
# Header comment line 2

address: "https://epr.elastic.co"

matrix:
  # - kibana.version: 7.17.0
  - kibana.version: 9.5.0
    spec.min: 2.3
    spec.max: 3.6
  - spec.min: 2.3
    spec.max: 3.6

queries:
  - {}

actions:
  - print:
`

func TestRewriteMatrixPreservesFileStructure(t *testing.T) {
	entries := []configQuery{
		{KibanaVersion: "9.5.0", SpecMin: "2.3", SpecMax: "3.6"},
		{KibanaVersion: "9.5.1", SpecMin: "2.3", SpecMax: "3.6"},
		{SpecMin: "2.3", SpecMax: "3.6"},
	}
	result, err := rewriteMatrix([]byte(sampleConfig), entries)
	require.NoError(t, err)

	out := string(result)
	assert.Contains(t, out, "# Header comment line 1")
	assert.Contains(t, out, `address: "https://epr.elastic.co"`)
	assert.Contains(t, out, "queries:\n  - {}")
	assert.Contains(t, out, "actions:\n  - print:")
}

func TestRewriteMatrixPreservesPreambleComments(t *testing.T) {
	entries := []configQuery{
		{KibanaVersion: "9.5.0", SpecMin: "2.3", SpecMax: "3.6"},
	}
	result, err := rewriteMatrix([]byte(sampleConfig), entries)
	require.NoError(t, err)
	assert.Contains(t, string(result), "  # - kibana.version: 7.17.0",
		"commented-out entries at the top of the matrix block must be preserved")
}

func TestRewriteMatrixPreservesPreambleCommentsAfterBlankLine(t *testing.T) {
	const configWithBlankLinePreamble = `address: "https://epr.elastic.co"

matrix:

  # - kibana.version: 7.17.0
  - kibana.version: 9.5.0
    spec.min: 2.3
    spec.max: 3.6

queries:
  - {}
`
	entries := []configQuery{
		{KibanaVersion: "9.5.0", SpecMin: "2.3", SpecMax: "3.6"},
	}
	result, err := rewriteMatrix([]byte(configWithBlankLinePreamble), entries)
	require.NoError(t, err)
	assert.Contains(t, string(result), "  # - kibana.version: 7.17.0",
		"commented-out entries after a blank line must not be silently dropped")
}

func TestRewriteMatrixAddsNewEntry(t *testing.T) {
	entries := []configQuery{
		{KibanaVersion: "9.5.0", SpecMin: "2.3", SpecMax: "3.6"},
		{KibanaVersion: "9.5.1", SpecMin: "2.3", SpecMax: "3.6"},
		{SpecMin: "2.3", SpecMax: "3.6"},
	}
	result, err := rewriteMatrix([]byte(sampleConfig), entries)
	require.NoError(t, err)
	assert.Contains(t, string(result), "  - kibana.version: 9.5.1")
}

func TestRewriteMatrixIsIdempotent(t *testing.T) {
	entries := []configQuery{
		{KibanaVersion: "9.5.0", SpecMin: "2.3", SpecMax: "3.6"},
		{SpecMin: "2.3", SpecMax: "3.6"},
	}
	first, err := rewriteMatrix([]byte(sampleConfig), entries)
	require.NoError(t, err)
	second, err := rewriteMatrix(first, entries)
	require.NoError(t, err)
	assert.Equal(t, string(first), string(second))
}

func TestRewriteMatrixPreservesBlankLineSeparator(t *testing.T) {
	entries := []configQuery{
		{KibanaVersion: "9.5.0", SpecMin: "2.3", SpecMax: "3.6"},
	}
	result, err := rewriteMatrix([]byte(sampleConfig), entries)
	require.NoError(t, err)
	// The blank line between the matrix block and queries: must be preserved.
	assert.Contains(t, string(result), "    spec.max: 3.6\n\nqueries:")
}

func TestRewriteMatrixNoMatrixKey(t *testing.T) {
	_, err := rewriteMatrix([]byte("address: foo\nqueries:\n  - {}\n"), nil)
	assert.Error(t, err)
}

// ─── matrixSource HTTP methods ────────────────────────────────────────────────

func newTestMatrixSource(t *testing.T, artifactsMux, kibanaMux http.Handler) matrixSource {
	t.Helper()
	artSrv := httptest.NewServer(artifactsMux)
	kibSrv := httptest.NewServer(kibanaMux)
	t.Cleanup(artSrv.Close)
	t.Cleanup(kibSrv.Close)
	// Use a plain client so either server can switch to TLS independently.
	return matrixSource{
		artifactsURL: artSrv.URL,
		kibanaRawURL: kibSrv.URL,
		client:       &http.Client{},
	}
}

func TestActiveBranchesHappyPath(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/branches.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"branches": []string{"main", "9.5", "9.4"}})
	})
	src := newTestMatrixSource(t, mux, http.NewServeMux())
	branches, err := src.activeBranches(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []string{"main", "9.5", "9.4"}, branches)
}

func TestActiveBranchesHTTPError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/branches.json", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	})
	src := newTestMatrixSource(t, mux, http.NewServeMux())
	_, err := src.activeBranches(context.Background())
	assert.Error(t, err)
}

func TestNextVersionStripsSnapshot(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/9.5.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"version": "9.5.5-SNAPSHOT"})
	})
	src := newTestMatrixSource(t, mux, http.NewServeMux())
	v, err := src.nextVersion(context.Background(), "9.5")
	require.NoError(t, err)
	assert.Equal(t, "9.5.5", v.Original())
}

func TestNextVersionHTTPError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/9.5.json", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	})
	src := newTestMatrixSource(t, mux, http.NewServeMux())
	_, err := src.nextVersion(context.Background(), "9.5")
	assert.Error(t, err)
}

func TestNextVersionBadVersion(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/main.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"version": "not-a-version"})
	})
	src := newTestMatrixSource(t, mux, http.NewServeMux())
	_, err := src.nextVersion(context.Background(), "main")
	assert.Error(t, err)
}

const configTSWithMinMax = `
const REGISTRY_SPEC_MIN_VERSION = '2.3';
const REGISTRY_SPEC_MAX_VERSION = '3.6';
`

const configTSMaxOnly = `
const REGISTRY_SPEC_MAX_VERSION = '3.4';
`

const configTSNoMax = `
const REGISTRY_SPEC_MIN_VERSION = '2.3';
`

func TestFetchSpecBoundsWithMinAndMax(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/9.5/x-pack/platform/plugins/shared/fleet/server/config.ts", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(configTSWithMinMax))
	})
	src := newTestMatrixSource(t, http.NewServeMux(), mux)
	bounds, err := src.fetchSpecBounds(context.Background(), "9.5")
	require.NoError(t, err)
	assert.Equal(t, "2.3", bounds.min)
	assert.Equal(t, "3.6", bounds.max)
}

func TestFetchSpecBoundsMaxOnly(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/8.19/x-pack/platform/plugins/shared/fleet/server/config.ts", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(configTSMaxOnly))
	})
	src := newTestMatrixSource(t, http.NewServeMux(), mux)
	bounds, err := src.fetchSpecBounds(context.Background(), "8.19")
	require.NoError(t, err)
	assert.Empty(t, bounds.min, "missing SPEC_MIN should yield empty min")
	assert.Equal(t, "3.4", bounds.max)
}

func TestFetchSpecBoundsNoMax(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/main/x-pack/platform/plugins/shared/fleet/server/config.ts", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(configTSNoMax))
	})
	src := newTestMatrixSource(t, http.NewServeMux(), mux)
	_, err := src.fetchSpecBounds(context.Background(), "main")
	assert.Error(t, err, "missing SPEC_MAX must return an error")
}

func TestFetchSpecBoundsHTTPError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/9.5/x-pack/platform/plugins/shared/fleet/server/config.ts", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	})
	src := newTestMatrixSource(t, http.NewServeMux(), mux)
	_, err := src.fetchSpecBounds(context.Background(), "9.5")
	assert.Error(t, err)
}
