package core

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Windows tools find their home, temp and cache directories through variables Unix tools never
// read, so each has to point into the build's own directory or the build reaches outside it.
func TestSetPlatformTmpEnv(t *testing.T) {
	env := BuildEnv{}
	setPlatformTmpEnv(env, "tmpdir")
	if runtime.GOOS != "windows" {
		assert.Empty(t, env, "Unix tools use HOME and TMPDIR, which are set elsewhere")
		return
	}
	for _, name := range []string{"USERPROFILE", "TEMP", "TMP", "LOCALAPPDATA", "APPDATA"} {
		assert.Equal(t, "tmpdir", env[name], name)
	}
}
