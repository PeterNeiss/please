package plzinit

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/please-build/buildtools/build"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// serveTags points the tag lookup at a local server for the rest of the test.
func serveTags(t *testing.T, handler http.HandlerFunc) {
	t.Helper()
	srv := httptest.NewServer(handler)
	old := githubAPI
	githubAPI = srv.URL
	t.Cleanup(func() {
		githubAPI = old
		srv.Close()
	})
}

// noNetwork fails the test if anything tries to list tags.
func noNetwork(t *testing.T) {
	serveTags(t, func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected tag lookup: %s", r.URL)
		w.WriteHeader(http.StatusInternalServerError)
	})
}

func readPluginRepo(t *testing.T, location string) *build.Rule {
	t.Helper()
	b, err := os.ReadFile(location)
	require.NoError(t, err)
	f, err := build.Parse(location, b)
	require.NoError(t, err)
	rules := f.Rules("plugin_repo")
	require.Len(t, rules, 1)
	return rules[0]
}

func TestCreatePluginTargetUsesThePinnedFork(t *testing.T) {
	noNetwork(t)
	location := filepath.Join(t.TempDir(), "plugins", "BUILD")
	require.NoError(t, createPluginTarget(location, "go", "", ""))

	rule := readPluginRepo(t, location)
	assert.Equal(t, "go", rule.Name())
	assert.Equal(t, "go-rules", rule.AttrString("plugin"))
	assert.Equal(t, pinnedPlugins["go"].Owner, rule.AttrString("owner"))
	assert.Equal(t, pinnedPlugins["go"].Revision, rule.AttrString("revision"))
}

func TestCreatePluginTargetKeepsTheForkForAnExplicitVersion(t *testing.T) {
	noNetwork(t)
	location := filepath.Join(t.TempDir(), "plugins", "BUILD")
	require.NoError(t, createPluginTarget(location, "python", "abc1234", ""))

	rule := readPluginRepo(t, location)
	assert.Equal(t, "PeterNeiss", rule.AttrString("owner"))
	assert.Equal(t, "abc1234", rule.AttrString("revision"))
}

func TestCreatePluginTargetLooksUpAnExplicitOwner(t *testing.T) {
	serveTags(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/repos/please-build/go-rules/tags", r.URL.Path)
		fmt.Fprint(w, `[{"name":"v1.33.1"}]`)
	})
	location := filepath.Join(t.TempDir(), "plugins", "BUILD")
	require.NoError(t, createPluginTarget(location, "go", "", "please-build"))

	rule := readPluginRepo(t, location)
	assert.Equal(t, "please-build", rule.AttrString("owner"))
	assert.Equal(t, "v1.33.1", rule.AttrString("revision"))
}

func TestCreatePluginTargetLooksUpUnpinnedPlugins(t *testing.T) {
	serveTags(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/repos/please-build/java-rules/tags", r.URL.Path)
		fmt.Fprint(w, `[{"name":"v0.4.1"}]`)
	})
	location := filepath.Join(t.TempDir(), "plugins", "BUILD")
	require.NoError(t, createPluginTarget(location, "java", "", ""))

	rule := readPluginRepo(t, location)
	assert.Equal(t, "please-build", rule.AttrString("owner"))
	assert.Equal(t, "v0.4.1", rule.AttrString("revision"))
}

func TestCreatePluginTargetWritesNothingWhenTheLookupFails(t *testing.T) {
	serveTags(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	})
	location := filepath.Join(t.TempDir(), "plugins", "BUILD")
	assert.Error(t, createPluginTarget(location, "java", "", ""))
	assert.NoFileExists(t, location)
}

// The pins here and in plugins/BUILD are one decision written down twice; this is what stops them
// drifting apart when a plugin is bumped.
func TestPinnedPluginsMatchPluginsBuild(t *testing.T) {
	const location = "plugins/BUILD"
	b, err := os.ReadFile(location)
	require.NoError(t, err)
	f, err := build.Parse(location, b)
	require.NoError(t, err)

	owners := map[string]bool{}
	build.Walk(f, func(x build.Expr, _ []build.Expr) {
		call, ok := x.(*build.CallExpr)
		if !ok {
			return
		}
		if ident, ok := call.X.(*build.Ident); !ok || ident.Name != "plugin_repo" {
			return
		}
		for _, arg := range call.List {
			if assign, ok := arg.(*build.AssignExpr); ok {
				if lhs, ok := assign.LHS.(*build.Ident); ok && lhs.Name == "owner" {
					if str, ok := assign.RHS.(*build.StringExpr); ok {
						owners[str.Value] = true
					}
				}
			}
		}
	})
	require.Len(t, owners, 1, "plugins/BUILD is expected to download every plugin from one owner")

	got := map[string]pluginSource{}
	for _, stmt := range f.Stmt {
		assign, ok := stmt.(*build.AssignExpr)
		if !ok {
			continue
		}
		if lhs, ok := assign.LHS.(*build.Ident); !ok || lhs.Name != "PLUGINS" {
			continue
		}
		list, ok := assign.RHS.(*build.ListExpr)
		require.True(t, ok, "PLUGINS is expected to be a list")
		for _, item := range list.List {
			tuple, ok := item.(*build.TupleExpr)
			require.True(t, ok, "each entry of PLUGINS is expected to be a tuple")
			require.Len(t, tuple.List, 3)
			name := tuple.List[0].(*build.StringExpr).Value
			revision := tuple.List[2].(*build.StringExpr).Value
			for owner := range owners {
				got[name] = pluginSource{Owner: owner, Revision: revision}
			}
		}
	}
	assert.Equal(t, pinnedPlugins, got)
}

func TestGetLatestRevisionPicksTheHighestReleaseTag(t *testing.T) {
	serveTags(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/repos/PeterNeiss/python-rules/tags", r.URL.Path)
		fmt.Fprint(w, `[{"name":"wheel_resolver-v2.1.0"},{"name":"v2.0.2"},{"name":"v2.10.0"},{"name":"v2.9.1"},{"name":"please_pex-v3.0.2"}]`)
	})
	revision, err := getLatestRevision("PeterNeiss", "python")
	require.NoError(t, err)
	assert.Equal(t, "v2.10.0", revision)
}

func TestGetLatestRevisionReadsEveryPage(t *testing.T) {
	serveTags(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("page") == "2" {
			fmt.Fprint(w, `[{"name":"v9.9.9"}]`)
			return
		}
		fmt.Fprint(w, "[")
		for i := 0; i < tagsPerPage; i++ {
			if i > 0 {
				fmt.Fprint(w, ",")
			}
			fmt.Fprintf(w, `{"name":"v0.0.%d"}`, i)
		}
		fmt.Fprint(w, "]")
	})
	revision, err := getLatestRevision("please-build", "go")
	require.NoError(t, err)
	assert.Equal(t, "v9.9.9", revision)
}

func TestGetLatestRevisionWithNoReleaseTags(t *testing.T) {
	serveTags(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `[{"name":"wheel_resolver-v2.1.0"}]`)
	})
	_, err := getLatestRevision("please-build", "python")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no release tags")
}

func TestGetLatestRevisionNamesTheRateLimit(t *testing.T) {
	serveTags(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	})
	_, err := getLatestRevision("please-build", "go")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rate limit")
	assert.Contains(t, err.Error(), "--version")
}

func TestGetLatestRevisionSendsAToken(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "sekrit")
	serveTags(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer sekrit", r.Header.Get("Authorization"))
		fmt.Fprint(w, `[{"name":"v1.0.0"}]`)
	})
	_, err := getLatestRevision("please-build", "go")
	require.NoError(t, err)
}
