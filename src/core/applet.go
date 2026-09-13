package core

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/thought-machine/please/src/fs"
)

// busyboxName is the shell Please bundles on Windows, and the program the applet fallback runs.
const busyboxName = "busybox"

// appletLists caches the applets each busybox binary provides, keyed by its path. Asking means
// running it, and a build resolves the same tools over and over.
var appletLists = struct {
	sync.Mutex
	m map[string]map[string]bool
}{m: map[string]map[string]bool{}}

// appletFallback finds a tool that is not on the path among the applets of the busybox that is.
//
// A rule written for Unix names tools like wc or sed and expects them on the build path. Windows
// has no such directory, and its default build path holds only Please's own install - but that
// is where busybox sits, and busybox has those tools built in. It picks which one to be from the
// name it was started under, so a copy called wc.exe is wc. This makes one, under plz-out, and
// returns it.
//
// Its content is busybox's, so the tool hash follows a busybox upgrade like any other tool.
// Only bare names are considered: a path or a name with an extension was asked for as a file.
func appletFallback(name string, paths []string, outDir string) (string, bool) {
	if name == busyboxName || strings.ContainsAny(name, fs.PathSeparators) || filepath.Ext(name) != "" || outDir == "" {
		return "", false
	}
	busybox, err := lookPath(busyboxName, paths)
	if err != nil {
		return "", false
	}
	if !busyboxApplets(busybox)[name] {
		return "", false
	}
	applet, err := materialiseApplet(busybox, name, outDir)
	if err != nil {
		log.Warning("Found %s as a busybox applet, but could not make it runnable: %s", name, err)
		return "", false
	}
	return applet, true
}

// busyboxApplets returns the set of applets the given busybox provides.
func busyboxApplets(busybox string) map[string]bool {
	appletLists.Lock()
	defer appletLists.Unlock()
	if applets, present := appletLists.m[busybox]; present {
		return applets
	}
	out, err := exec.Command(busybox, "--list").Output()
	if err != nil {
		log.Debug("Can't list the applets of %s: %s", busybox, err)
	}
	applets := parseAppletList(out)
	appletLists.m[busybox] = applets
	return applets
}

// parseAppletList parses the output of busybox --list: one applet name per line.
func parseAppletList(out []byte) map[string]bool {
	applets := map[string]bool{}
	for _, line := range bytes.Split(out, []byte{'\n'}) {
		if name := strings.TrimSpace(string(line)); name != "" {
			applets[name] = true
		}
	}
	return applets
}

// materialiseApplet puts a copy of busybox named for the applet into a directory under outDir and
// returns its absolute path. The directory is keyed by the busybox binary, so a different or
// upgraded one never reuses a stale copy. It is a hard link where the filesystem allows.
func materialiseApplet(busybox, name, outDir string) (string, error) {
	info, err := os.Stat(busybox)
	if err != nil {
		return "", err
	}
	h := sha1.Sum([]byte(fmt.Sprintf("%s\x00%d\x00%d", busybox, info.Size(), info.ModTime().UnixNano())))
	dir := filepath.Join(outDir, "busybox", hex.EncodeToString(h[:6]))
	dest, err := filepath.Abs(filepath.Join(dir, name+fs.ExeSuffix))
	if err != nil {
		return "", err
	}
	if fs.FileExists(dest) {
		return dest, nil
	}
	if err := os.MkdirAll(dir, fs.DirPermissions); err != nil {
		return "", err
	}
	// Made under a name of its own and renamed into place, since any number of targets may be
	// resolving the same tool at once. Whoever loses the rename finds the winner's copy there.
	tmp := fmt.Sprintf("%s.%d.tmp", dest, os.Getpid())
	if err := fs.CopyOrLinkFile(busybox, tmp, info.Mode(), info.Mode(), true, true); err != nil {
		return "", err
	}
	if err := os.Rename(tmp, dest); err != nil {
		os.Remove(tmp)
		if !fs.FileExists(dest) {
			return "", err
		}
	}
	return dest, nil
}
