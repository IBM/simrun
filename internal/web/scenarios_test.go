package web

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// The precedence contract for a tag key is: org-wide default tag <
// pack-level default_tags entry. Packs override per-key; they cannot
// delete an org tag.
func TestMergeOrgDefaultTags_OrgTagAppliesWhenPackHasNone(t *testing.T) {
	params := map[string]any{"aws_region": "us-east-1"}
	got := mergeOrgDefaultTags(params, map[string]string{"owner": "secops"})

	assert.Equal(t, map[string]any{
		"aws_region":   "us-east-1",
		"default_tags": map[string]string{"owner": "secops"},
	}, got)
}

func TestMergeOrgDefaultTags_PackLevelKeyWinsPerKey(t *testing.T) {
	params := map[string]any{
		"default_tags": map[string]any{"owner": "red-team"},
	}
	got := mergeOrgDefaultTags(params, map[string]string{"owner": "secops", "env": "sim"})

	assert.Equal(t, map[string]any{
		"default_tags": map[string]string{"owner": "red-team", "env": "sim"},
	}, got)
}

// An empty org map must be a no-op so behavior is identical to before
// org-wide default tags existed: the same map, not a copy.
func TestMergeOrgDefaultTags_EmptyOrgMapPassesThrough(t *testing.T) {
	params := map[string]any{
		"default_tags": map[string]any{"team": "red"},
	}
	for _, orgTags := range []map[string]string{nil, {}} {
		got := mergeOrgDefaultTags(params, orgTags)
		assert.Equal(t, params, got)
	}
}

// A pack-level default_tags that is not a string→string map cannot be
// merged; it must pass through unmodified so terraform sees exactly what
// the pack stored (current behavior preserved).
func TestMergeOrgDefaultTags_MalformedPackValueUntouched(t *testing.T) {
	for name, malformed := range map[string]any{
		"string":           "owner=secops",
		"array":            []any{"owner"},
		"non-string value": map[string]any{"owner": 123},
	} {
		t.Run(name, func(t *testing.T) {
			params := map[string]any{"default_tags": malformed}
			got := mergeOrgDefaultTags(params, map[string]string{"owner": "secops"})
			assert.Equal(t, map[string]any{"default_tags": malformed}, got)
		})
	}
}

// Nil pack parameters with org tags set must still produce a default_tags
// entry so packs configured with no parameters inherit org tags.
func TestMergeOrgDefaultTags_NilParamsGetOrgTags(t *testing.T) {
	got := mergeOrgDefaultTags(nil, map[string]string{"owner": "secops"})
	assert.Equal(t, map[string]any{
		"default_tags": map[string]string{"owner": "secops"},
	}, got)
}

// The merge must not mutate the pack's stored map or the org map, so a
// later reload sees the original stored parameters.
func TestMergeOrgDefaultTags_DoesNotMutateInputs(t *testing.T) {
	params := map[string]any{
		"default_tags": map[string]any{"owner": "red-team"},
	}
	orgTags := map[string]string{"owner": "secops", "env": "sim"}

	_ = mergeOrgDefaultTags(params, orgTags)

	assert.Equal(t, map[string]any{"default_tags": map[string]any{"owner": "red-team"}}, params)
	assert.Equal(t, map[string]string{"owner": "secops", "env": "sim"}, orgTags)
}
