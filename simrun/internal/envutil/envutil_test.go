package envutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLookup(t *testing.T) {
	t.Run("nil map falls back to process env when key absent", func(t *testing.T) {
		t.Setenv("SR_TEST_ENVUTIL_KEY", "from-process")
		assert.Equal(t, "from-process", Lookup(nil, "SR_TEST_ENVUTIL_KEY"))
	})

	t.Run("nil map returns empty when process env unset", func(t *testing.T) {
		// Ensure unset for this test.
		t.Setenv("SR_TEST_ENVUTIL_UNSET", "")
		assert.Equal(t, "", Lookup(nil, "SR_TEST_ENVUTIL_UNSET"))
	})

	t.Run("non-nil map returns map value", func(t *testing.T) {
		t.Setenv("SR_TEST_ENVUTIL_KEY", "from-process")
		got := Lookup(map[string]string{"SR_TEST_ENVUTIL_KEY": "from-map"}, "SR_TEST_ENVUTIL_KEY")
		assert.Equal(t, "from-map", got)
	})

	t.Run("non-nil map returns empty for absent key even if process env set", func(t *testing.T) {
		t.Setenv("SR_TEST_ENVUTIL_LEAKED", "should-not-leak")
		got := Lookup(map[string]string{"OTHER_KEY": "x"}, "SR_TEST_ENVUTIL_LEAKED")
		assert.Equal(t, "", got)
	})
}
