//go:build !windows

package core

// appletsSupported is true where a tool missing from the build path is looked for among the
// bundled busybox's applets. Everywhere but Windows the build path has the real tools on it,
// and a busybox found there is the user's own, not one Please bundles.
const appletsSupported = false
