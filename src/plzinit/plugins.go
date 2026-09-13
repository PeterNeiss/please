package plzinit

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/coreos/go-semver/semver"
	"github.com/please-build/buildtools/build"
	"github.com/please-build/gcfg/ast"

	"github.com/thought-machine/please/src/core"
	"github.com/thought-machine/please/src/fs"
)

const pluginRepoTemplate = `plugin_repo(
  name = "%s",
  revision = "%s",
  plugin = "%s-rules",
  owner = "%s",
)
`

// defaultPluginOwner is where a plugin comes from when nothing says otherwise.
const defaultPluginOwner = "please-build"

// pluginSource is the owner and revision a plugin_repo target downloads.
type pluginSource struct {
	Owner    string
	Revision string
}

// pinnedPlugins are the plugins this Please initialises from forks rather than from please-build,
// at the revision it builds and tests them at. They are the pins in plugins/BUILD, and
// TestPinnedPluginsMatchPluginsBuild keeps the two in step; bump both together.
//
// The forks carry Windows support that upstream has not merged, and upstream publishes no
// windows_amd64 please_go, please_pex or please_cc. A repo initialised against please-build
// therefore cannot build a Go, C++ or Python target on Windows at all, which is where every
// codelab that installs a plugin stopped. A commit rather than a tag, because the forks' tag lists
// are copies of upstream's and their newest tags do not contain the Windows work; and a pin needs
// no call to GitHub's API, which refuses anonymous callers from shared CI addresses.
var pinnedPlugins = map[string]pluginSource{
	"go":     {Owner: "PeterNeiss", Revision: "9c26bfd"},
	"cc":     {Owner: "PeterNeiss", Revision: "90913bb"},
	"shell":  {Owner: "PeterNeiss", Revision: "7ed07be"},
	"python": {Owner: "PeterNeiss", Revision: "eecfd15"},
}

var pluginInitFns = map[string]func() (map[string]string, error){
	"go": initGo,
}

// Mapping of the built in config from Please v16 to the new plugins introduced in v17
type v16Mapping = map[string]string

var pluginVersion16Map = map[string]v16Mapping{
	"go": {
		"gotool":           "GoTool",
		"importpath":       "ImportPath",
		"cgocctool":        "CCTool",
		"cgoenabled":       "CGoEnabled",
		"pleasegotool":     "PleaseGoTool",
		"delvetool":        "DelveTool",
		"defaultstatic":    "DefaultStatic",
		"gotestrootcompat": "TestRootCompat",
		"cflags":           "CFlags",
		"ldflags":          "LdFlags",
	},
	"python": {
		"piptool":             "PipTool",
		"pipflags":            "PipFlags",
		"pextool":             "PexTool",
		"defaultinterpreter":  "DefaultInterpreter",
		"testrunner":          "TestRunner",
		"debugger":            "Debugger",
		"moduledir":           "ModuleDir",
		"defaultpiprepo":      "DefaultPipRepo",
		"wheelrepo":           "WheelRepo",
		"wheelnamescheme":     "WheelNameScheme",
		"disablevendorflags":  "DisableVendorFlags",
		"usepypi":             "UsePypi",
		"testrunnerbootstrap": "TestrunnerDeps",
	},
	"cc": {
		"cctool":             "CCTool",
		"cpptool":            "CPPTool",
		"ldtool":             "LDTool",
		"artool":             "ARTool",
		"defaultoptcflags":   "DefaultOptCFlags",
		"defaultdbgcflags":   "DefaultDbgCFlags",
		"defaultoptcppflags": "DefaultOptCppFlags",
		"defaultdbgcppflags": "DefaultDbgCppFlags",
		"defaultldflags":     "DefaultLdFlags",
		"pkgconfigpath":      "PkgConfigPath",
		"testmain":           "TestMain",
		"dsymtool":           "DsymTool",
	},
	"java": {
		"javactool":          "JavacTool",
		"javacworker":        "JavacWorker",
		"junitrunner":        "JunitRunner",
		"defaulttestpackage": "DefaultTestPackage",
		"releaselevel":       "ReleaseLevel",
		"targetlevel":        "TargetLevel",
		"javacflags":         "JavacFlags",
		"javactestflags":     "JavacTestFlags",
		"defaultmavenrepo":   "MavenRepo",
		"toolchain":          "Toolchain",
	},
}

func info(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
}

// InitPlugins initialises one or more plugins by inserting plugin config values into
// the host repo config file, and creating a build target in //plugins.
//
// owner and version may each be empty. A plugin in pinnedPlugins then comes from its fork at the
// pinned revision; any other comes from please-build at its newest release tag.
func InitPlugins(plugins []string, version, owner string) error {
	log.Debug("Initialising plugin(s): %v", plugins)

	// Check that we're in a plz repo
	configPath := filepath.Join(core.RepoRoot, ".plzconfig")
	if !fs.FileExists(configPath) {
		return fmt.Errorf("You don't appear to be in a plz repo.")
	}

	configFile, err := os.Open(configPath)
	if err != nil {
		return fmt.Errorf("Failed to open plz config file")
	}
	defer configFile.Close()

	// Read config file into AST
	file := ast.Read(configFile)

	for _, p := range plugins {
		config, err := initPlugin(p, version, owner)
		if err != nil {
			return fmt.Errorf("Could not initialise plugin %s. Got error: %s", p, err)
		}
		file = writeFieldsToConfig(p, file, config)
	}

	return ast.Write(file, configPath)
}

// initPlugin initialises the plugin, performing any plugin specific operations, returning the plugin config
func initPlugin(plugin, version, owner string) (map[string]string, error) {
	if err := createPluginTarget("plugins/BUILD", plugin, version, owner); err != nil {
		return nil, err
	}

	if fn, ok := pluginInitFns[plugin]; ok {
		return fn()
	}
	return nil, nil
}

func writeFieldsToConfig(plugin string, plzConfig ast.File, pluginConfig map[string]string) ast.File {
	section := "Plugin"

	pluginName := strings.ReplaceAll(plugin, "-", "_")

	// Check for existing plugin section
	if s := plzConfig.MaybeGetSection(section, pluginName); s != nil {
		info("Plugin config section already exists, so init did nothing.")
		return plzConfig
	}

	// Inject the preloadsubincludes
	// TODO(sam): We can get the actual name of the package containing the build_defs
	// if we build the plugin target, which we do below. Refactor this to build the target
	// earlier and use the build_defs dir specified in the plugin config
	subincludeStr := "///" + pluginName + "//build_defs:" + pluginName
	plzConfig = ast.InjectField(plzConfig, "preloadsubincludes", subincludeStr, "parse", "", true)

	// Write plugin target value
	plzConfig = ast.InjectField(plzConfig, "Target", "//plugins:"+pluginName, section, pluginName, false)

	for k, v := range pluginConfig {
		plzConfig = ast.InjectField(plzConfig, k, v, section, pluginName, false)
	}

	// Migrate any existing language fields to their plugin equivalents
	if configMap, ok := pluginVersion16Map[plugin]; ok {
		for _, s := range plzConfig.Sections {
			if s.Key == pluginName {
				for _, field := range s.Fields {
					if plugVal, ok := configMap[strings.ToLower(field.Name)]; ok {
						plzConfig = ast.InjectField(plzConfig, plugVal, field.Value, section, pluginName, true)
					}
				}
			}
		}
	}

	return plzConfig
}

// targetExistsInFile checks to see if the plugin target already exists
// in plugins/BUILD
func targetExistsInFile(location, target string) (bool, error) {
	if !fs.FileExists(location) {
		return false, nil
	}

	b, err := os.ReadFile(location)
	if err != nil {
		return false, err
	}

	f, err := build.Parse(location, b)
	if err != nil {
		return false, err
	}

	for _, rule := range f.Rules("plugin_repo") {
		if rule.Name() == target {
			return true, nil
		}
	}
	return false, nil
}

// resolvePluginSource works out where a plugin comes from. An explicit owner or version wins; a
// pinned plugin otherwise keeps its fork, and only an unpinned revision is looked up.
func resolvePluginSource(plugin, version, owner string) (pluginSource, error) {
	pinned, isPinned := pinnedPlugins[plugin]
	if owner == "" {
		owner = defaultPluginOwner
		if isPinned {
			owner = pinned.Owner
		}
	}
	if version == "" {
		if isPinned && owner == pinned.Owner {
			return pinned, nil
		}
		revision, err := getLatestRevision(owner, plugin)
		if err != nil {
			return pluginSource{}, err
		}
		version = revision
	}
	return pluginSource{Owner: owner, Revision: version}, nil
}

// createPluginTarget writes the plugin target to plugins/BUILD
func createPluginTarget(location, plugin, version, owner string) error {
	pluginTarget := strings.ReplaceAll(plugin, "-", "_")
	exists, err := targetExistsInFile(location, pluginTarget)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}

	// Resolved before anything is written, so a failed lookup leaves no half-made plugins/BUILD.
	source, err := resolvePluginSource(plugin, version, owner)
	if err != nil {
		return err
	}

	pkg := filepath.Dir(location)
	if err := os.MkdirAll(pkg, core.DirPermissions); err != nil {
		return err
	}

	f, err := os.OpenFile(location, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = fmt.Fprintf(f, pluginRepoTemplate, pluginTarget, source.Revision, plugin, source.Owner)

	return err
}

type Response []struct {
	Name       string `json:"name"`
	ZipballURL string `json:"zipball_url"`
	TarballURL string `json:"tarball_url"`
	Commit     struct {
		Sha string `json:"sha"`
		URL string `json:"url"`
	} `json:"commit"`
	NodeID string `json:"node_id"`
}

// githubAPI is where tags are listed. A variable so that tests can serve them locally.
var githubAPI = "https://api.github.com"

// releaseTagRe matches a plugin release tag. Plugin repos also tag their tools' releases -
// wheel_resolver-v2.1.0, please_pex-v3.0.2 - and GitHub lists tags in no order that puts the
// plugin's own newest release first, so taking the first tag listed picked a tool's tag instead.
var releaseTagRe = regexp.MustCompile(`^v\d+\.\d+\.\d+$`)

// tagsPerPage is the most tags GitHub returns in one page.
const tagsPerPage = 100

// getLatestRevision returns the highest vX.Y.Z tag of owner's <plugin>-rules repo.
func getLatestRevision(owner, plugin string) (string, error) {
	var best *semver.Version
	bestName := ""
	for page := 1; ; page++ {
		tags, err := fetchTags(fmt.Sprintf("%s/repos/%s/%s-rules/tags?per_page=%d&page=%d", githubAPI, owner, plugin, tagsPerPage, page))
		if err != nil {
			return "", err
		}
		for _, tag := range tags {
			if !releaseTagRe.MatchString(tag.Name) {
				continue
			}
			v, err := semver.NewVersion(strings.TrimPrefix(tag.Name, "v"))
			if err != nil {
				continue
			}
			if best == nil || best.LessThan(*v) {
				best, bestName = v, tag.Name
			}
		}
		if len(tags) < tagsPerPage {
			break
		}
	}
	if best == nil {
		return "", fmt.Errorf("%s/%s-rules has no release tags of the form vX.Y.Z; pass --version", owner, plugin)
	}
	return bestName, nil
}

// fetchTags lists one page of tags.
func fetchTags(url string) (Response, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("accept", "application/vnd.github.v3+json")
	// Anonymous requests share a small hourly allowance per address, which a CI runner's address
	// has usually spent.
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	switch {
	case resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests:
		return nil, fmt.Errorf("GitHub refused to list the plugin's tags (%s), most likely its rate limit for anonymous requests; set GITHUB_TOKEN or pass --version", resp.Status)
	case resp.StatusCode != http.StatusOK:
		return nil, fmt.Errorf("Failed to list the plugin's tags: %s %s", resp.Status, string(body))
	}
	var result Response
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	return result, nil
}
