package core

// setPlatformTmpEnv sets any platform-specific environment variables pointing at a build
// action's temporary directory. Windows-native tools look at USERPROFILE rather than HOME,
// and at TEMP/TMP rather than TMPDIR, so they need the same redirection for the build
// environment to stay hermetic.
//
// LOCALAPPDATA and APPDATA are the Windows counterparts of ~/.cache and ~/.config, and they
// are not optional: Go on Windows finds its build cache under LOCALAPPDATA, and without it
// every go command in a build action failed with "GOCACHE is not defined and %LocalAppData%
// is not defined". On Unix the same lookups fall back to HOME, which is already redirected.
func setPlatformTmpEnv(env BuildEnv, dir string) {
	env["USERPROFILE"] = dir
	env["TEMP"] = dir
	env["TMP"] = dir
	env["LOCALAPPDATA"] = dir
	env["APPDATA"] = dir
}
