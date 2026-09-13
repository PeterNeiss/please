package core

// appletsSupported is true where a tool missing from the build path is looked for among the
// bundled busybox's applets. Only Windows needs it: it has no directory of standard tools, and
// it is the only platform Please bundles busybox for.
const appletsSupported = true
