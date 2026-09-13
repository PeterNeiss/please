//go:build !windows

package run

import "github.com/thought-machine/please/src/core"

// withScriptShell leaves args alone: outside Windows the kernel reads a script's #! line itself.
func withScriptShell(config *core.Configuration, args []string) []string {
	return args
}
