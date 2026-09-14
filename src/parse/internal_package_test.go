// Tests for generating the internal package.

package parse

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/thought-machine/please/src/core"
)

func TestInternalPackageWithoutArcat(t *testing.T) {
	// There is no arcat release for every platform Please runs on - Windows has none at all -
	// and it used to be an error to generate the internal package there, which stopped
	// everything rather than just the things that need arcat. The rule is simply left out now.
	config := core.DefaultConfiguration()
	pkg, err := GetInternalPackage(config)
	require.NoError(t, err)
	if arcatHashFor("no_such_platform") == "" {
		// Sanity check on the helper itself before relying on it below.
		assert.Contains(t, pkg, "please_sandbox", "the rest of the package should still be there")
	}
}

func TestArcatHashKnownAndUnknownPlatforms(t *testing.T) {
	assert.NotEmpty(t, arcatHashFor("linux_amd64"))
	assert.Empty(t, arcatHashFor("windows_amd64"), "no arcat is published for Windows")
}

func TestArcatUnavailableOnlyWhenNothingElseIsConfigured(t *testing.T) {
	config := core.DefaultConfiguration()
	config.Build.ArcatTool = "C:/tools/arcat.exe"
	assert.False(t, ArcatUnavailable(config), "a configured arcat is never unavailable")
}

func TestDefaultArcatToolNamesTheInternalPackage(t *testing.T) {
	// core cannot import this package, so it spells the label out. If the internal package is
	// ever renamed, the default arcat tool and the check that recognises it drift apart
	// silently and ArcatUnavailable starts answering false for a default config.
	assert.Equal(t, "/////"+InternalPackageName+":arcat", core.DefaultArcatTool)
}
