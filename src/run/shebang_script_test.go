package run

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/thought-machine/please/src/core"
)

const busybox = `C:\please\busybox.exe`

// onlyOnWindows skips the rest of a test elsewhere. These live in a file compiled everywhere, with
// a runtime check rather than a _windows suffix, because the generated test main names every test
// in its sources, and on Linux one excluded by its filename is a name that does not exist.
func onlyOnWindows(t *testing.T) {
	t.Helper()
	if runtime.GOOS != "windows" {
		t.Skip("Windows has no #! mechanism; everywhere else the kernel runs the script itself")
	}
}

func scriptConfig() *core.Configuration {
	config := core.DefaultConfiguration()
	config.Build.Shell = busybox
	return config
}

func writeScript(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	require.NoError(t, os.WriteFile(path, []byte(content), 0o755))
	return path
}

func TestWithScriptShellRunsABashScriptThroughBusybox(t *testing.T) {
	onlyOnWindows(t)
	script := writeScript(t, "wc.sh", "#!/bin/bash\n\nwc -w $@\n")
	args := withScriptShell(scriptConfig(), []string{script, "file.txt"})
	assert.Equal(t, []string{busybox, "bash", script, "file.txt"}, args)
}

func TestWithScriptShellRunsAnExtensionlessShScript(t *testing.T) {
	onlyOnWindows(t)
	script := writeScript(t, "tool", "#!/bin/sh\necho hi\n")
	args := withScriptShell(scriptConfig(), []string{script})
	assert.Equal(t, []string{busybox, "sh", script}, args)
}

func TestWithScriptShellUsesARealShellAsItIs(t *testing.T) {
	onlyOnWindows(t)
	script := writeScript(t, "wc.sh", "#!/bin/bash\n")
	config := scriptConfig()
	config.Build.Shell = `C:\Program Files\Git\bin\bash.exe`
	args := withScriptShell(config, []string{script})
	assert.Equal(t, []string{`C:\Program Files\Git\bin\bash.exe`, script}, args)
}

func TestWithScriptShellLeavesOtherInterpretersAlone(t *testing.T) {
	onlyOnWindows(t)
	script := writeScript(t, "tool.py", "#!/usr/bin/env python3\nprint('hi')\n")
	args := withScriptShell(scriptConfig(), []string{script})
	assert.Equal(t, []string{script}, args)
}

func TestWithScriptShellLeavesRunnableNamesAlone(t *testing.T) {
	onlyOnWindows(t)
	script := writeScript(t, "tool.cmd", "#!/bin/sh\n")
	args := withScriptShell(scriptConfig(), []string{script})
	assert.Equal(t, []string{script}, args)
}
