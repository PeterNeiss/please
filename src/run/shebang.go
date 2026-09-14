package run

import (
	"path"
	"strings"
)

// shebangShell says whether a script's first line names a shell the bundled busybox can stand in
// for, and which of its applets that is. Only sh, ash and bash: busybox provides those, and a
// script for any other interpreter is left to fail as it would have, with the explanation
// fs.ExplainUnrunnable gives.
//
// Kept apart from the Windows wiring so it can be tested anywhere. Only Windows acts on it, since
// everywhere else the kernel reads the #! line itself.
func shebangShell(head []byte) (applet string, ok bool) {
	line := string(head)
	if !strings.HasPrefix(line, "#!") {
		return "", false
	}
	if i := strings.IndexAny(line, "\r\n"); i >= 0 {
		line = line[:i]
	}
	fields := strings.Fields(line[2:])
	if len(fields) == 0 {
		return "", false
	}
	interpreter := fields[0]
	// #!/usr/bin/env bash, and #!/usr/bin/env -S bash -e, name the interpreter after env's flags.
	if baseName(interpreter) == "env" {
		interpreter = ""
		for _, field := range fields[1:] {
			if !strings.HasPrefix(field, "-") {
				interpreter = field
				break
			}
		}
	}
	switch baseName(interpreter) {
	case "sh", "ash":
		return "sh", true
	case "bash":
		return "bash", true
	}
	return "", false
}

// baseName is the last element of a path written with either separator.
func baseName(p string) string {
	if p == "" {
		return ""
	}
	return path.Base(strings.ReplaceAll(p, `\`, "/"))
}
