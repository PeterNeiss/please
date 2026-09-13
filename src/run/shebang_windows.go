package run

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/thought-machine/please/src/core"
	"github.com/thought-machine/please/src/fs"
)

// withScriptShell runs an sh or bash script through the shell build actions use, when that is the
// only way Windows will run it at all.
//
// Windows decides what is runnable by extension and has no #! mechanism, so a script built the
// Unix way - a filegroup or genrule output marked binary, holding #!/bin/bash - cannot be started
// by name however it is written. The genrule codelab builds its word-count tool exactly like that,
// and plz run failed on it with "%1 is not a valid Win32 application". busybox, which Please
// bundles and runs build actions with, provides both applets, so the script runs as its author
// meant it to.
//
// A file with an extension Windows runs is left alone, and so is a script for any other
// interpreter.
func withScriptShell(config *core.Configuration, args []string) []string {
	script := args[0]
	lower := strings.ToLower(script)
	for _, name := range fs.ExecutableNames("") {
		if name != "" && strings.HasSuffix(lower, name) {
			return args
		}
	}
	f, err := os.Open(script)
	if err != nil {
		return args
	}
	defer f.Close()
	head := make([]byte, 256)
	n, _ := f.Read(head)
	applet, ok := shebangShell(head[:n])
	if !ok {
		return args
	}
	shell := config.Shell()
	argv := []string{shell}
	// busybox has to be told which applet to be. A real sh or bash is already the interpreter.
	if strings.TrimSuffix(strings.ToLower(filepath.Base(shell)), ".exe") == "busybox" {
		argv = append(argv, applet)
	}
	return append(argv, args...)
}
