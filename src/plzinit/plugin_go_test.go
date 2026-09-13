package plzinit

import (
	"os"
	"testing"

	"github.com/please-build/buildtools/build"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// inTempDir runs the rest of the test in a fresh directory, since initGo writes relative paths.
func inTempDir(t *testing.T) {
	t.Helper()
	old, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(t.TempDir()))
	t.Cleanup(func() { os.Chdir(old) })
}

func withGoVersion(t *testing.T, version string) {
	t.Helper()
	old := getLatestGoVersion
	getLatestGoVersion = func() (string, error) { return version, nil }
	t.Cleanup(func() { getLatestGoVersion = old })
}

func readThirdPartyGo(t *testing.T) *build.File {
	t.Helper()
	b, err := os.ReadFile(buildFilePath)
	require.NoError(t, err)
	f, err := build.Parse(buildFilePath, b)
	require.NoError(t, err)
	return f
}

func TestInitGoWritesAToolchainAndAStdlib(t *testing.T) {
	inTempDir(t)
	withGoVersion(t, "go1.22.3")

	config, err := initGo()
	require.NoError(t, err)
	assert.Equal(t, map[string]string{
		"GoTool": "//third_party/go:toolchain|go",
		"STDLib": "//third_party/go:std",
	}, config)

	f := readThirdPartyGo(t)
	toolchains := f.Rules("go_toolchain")
	require.Len(t, toolchains, 1)
	assert.Equal(t, "1.22.3", toolchains[0].AttrString("version"))
	assert.Len(t, f.Rules("go_stdlib"), 1)
}

// An existing go_stdlib under another name used to overwrite GoTool with the stdlib's label, and
// leave STDLib naming a target that does not exist.
func TestInitGoUsesAnExistingStdlibAndToolchain(t *testing.T) {
	inTempDir(t)
	withGoVersion(t, "go1.22.3")
	require.NoError(t, os.MkdirAll("third_party/go", 0o755))
	require.NoError(t, os.WriteFile(buildFilePath, []byte(`go_toolchain(
    name = "tc",
    version = "1.21",
)

go_stdlib(
    name = "stdlib",
)
`), 0o644))

	config, err := initGo()
	require.NoError(t, err)
	assert.Equal(t, "//third_party/go:tc|go", config["GoTool"])
	assert.Equal(t, "//third_party/go:stdlib", config["STDLib"])

	f := readThirdPartyGo(t)
	assert.Len(t, f.Rules("go_toolchain"), 1, "nothing is added beside an existing toolchain")
	assert.Len(t, f.Rules("go_stdlib"), 1, "nothing is added beside an existing stdlib")
}
