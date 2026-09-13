package core

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/thought-machine/please/src/fs"
)

func TestParseAppletList(t *testing.T) {
	applets := parseAppletList([]byte("[\nsh\r\nwc\n\n  sed \n"))
	assert.Equal(t, map[string]bool{"[": true, "sh": true, "wc": true, "sed": true}, applets)
}

// writeFakeBusybox writes a busybox that lists the given applets, and nothing else.
func writeFakeBusybox(t *testing.T, applets string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the fake busybox is a shell script")
	}
	dir := t.TempDir()
	script := "#!/bin/sh\nprintf '" + applets + "'\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, busyboxName), []byte(script), 0o755))
	return dir
}

func TestAppletFallback(t *testing.T) {
	dir := writeFakeBusybox(t, "sh\\nwc\\n")
	outDir := t.TempDir()
	found, ok := appletFallback("wc", []string{dir}, outDir)
	require.True(t, ok)
	assert.True(t, filepath.IsAbs(found))
	assert.Equal(t, "wc"+fs.ExeSuffix, filepath.Base(found))
	want, err := os.ReadFile(filepath.Join(dir, busyboxName))
	require.NoError(t, err)
	got, err := os.ReadFile(found)
	require.NoError(t, err)
	assert.Equal(t, want, got, "the applet is busybox under another name")

	again, ok := appletFallback("wc", []string{dir}, outDir)
	require.True(t, ok)
	assert.Equal(t, found, again)
}

func TestAppletFallbackNotAnApplet(t *testing.T) {
	dir := writeFakeBusybox(t, "sh\\n")
	_, ok := appletFallback("wibblewobbleflibble", []string{dir}, t.TempDir())
	assert.False(t, ok)
}

func TestAppletFallbackOnlyBareNames(t *testing.T) {
	dir := writeFakeBusybox(t, "wc\\nwc.exe\\n")
	for _, name := range []string{"wc.exe", "tools/wc", busyboxName} {
		_, ok := appletFallback(name, []string{dir}, t.TempDir())
		assert.False(t, ok, name)
	}
}

func TestAppletFallbackNoBusybox(t *testing.T) {
	_, ok := appletFallback("wc", []string{t.TempDir()}, t.TempDir())
	assert.False(t, ok)
}
