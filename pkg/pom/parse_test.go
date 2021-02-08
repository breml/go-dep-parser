package pom

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aquasecurity/go-dep-parser/pkg/types"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name      string
		inputFile string
		local     bool
		want      []types.Library
		wantErr   string
	}{
		{
			name:      "local repository",
			inputFile: filepath.Join("testdata", "happy", "pom.xml"),
			local:     true,
			want: []types.Library{
				{
					Name:    "com.example:happy",
					Version: "1.0.0",
				},
				{
					Name:    "org.example:example-api",
					Version: "1.7.30",
				},
			},
		},
		{
			name:      "remote repository",
			inputFile: filepath.Join("testdata", "happy", "pom.xml"),
			local:     false,
			want: []types.Library{
				{
					Name:    "com.example:happy",
					Version: "1.0.0",
				},
				{
					Name:    "org.example:example-api",
					Version: "1.7.30",
				},
			},
		},
		{
			name:      "inherit parent properties",
			inputFile: filepath.Join("testdata", "parent-properties", "child", "pom.xml"),
			local:     true,
			want: []types.Library{
				{
					Name:    "com.example:child",
					Version: "1.0.0",
				},
				{
					Name:    "org.example:example-api",
					Version: "1.7.30",
				},
			},
		},
		{
			name:      "inherit parent dependencies",
			inputFile: filepath.Join("testdata", "parent-dependencies", "child", "pom.xml"),
			local:     false,
			want: []types.Library{
				{
					Name:    "com.example:child",
					Version: "1.0.0-SNAPSHOT",
				},
				{
					Name:    "org.example:example-api",
					Version: "1.7.30",
				},
			},
		},
		{
			name:      "inherit parent dependencyManagement",
			inputFile: filepath.Join("testdata", "parent-dependency-management", "child", "pom.xml"),
			local:     true,
			want: []types.Library{
				{
					Name:    "com.example:child",
					Version: "3.0.0",
				},
				{
					Name:    "org.example:example-api",
					Version: "1.7.30",
				},
			},
		},
		{
			name:      "parent relativePath",
			inputFile: filepath.Join("testdata", "parent-relative-path", "pom.xml"),
			local:     true,
			want: []types.Library{
				{
					Name:    "com.example:child",
					Version: "1.0.0",
				},
				{
					Name:    "org.example:example-api",
					Version: "1.7.30",
				},
			},
		},
		{
			name:      "parent in a remote repository",
			inputFile: filepath.Join("testdata", "parent-remote-repository", "pom.xml"),
			local:     true,
			want: []types.Library{
				{
					Name:    "org.example:child",
					Version: "1.0.0",
				},
				{
					Name:    "org.example:example-api",
					Version: "1.7.30",
				},
			},
		},
		{
			name:      "soft requirement",
			inputFile: filepath.Join("testdata", "soft-requirement", "pom.xml"),
			local:     true,
			want: []types.Library{
				{
					Name:    "com.example:soft",
					Version: "1.0.0",
				},
				{
					Name:    "org.example:example-api",
					Version: "1.7.30",
				},
				{
					Name:    "org.example:example-dependency",
					Version: "1.2.3",
				},
			},
		},
		{
			name:      "hard requirement",
			inputFile: filepath.Join("testdata", "hard-requirement", "pom.xml"),
			local:     true,
			want: []types.Library{
				{
					Name:    "com.example:hard",
					Version: "1.0.0",
				},
				{
					Name:    "org.example:example-api",
					Version: "2.0.0",
				},
				{
					Name:    "org.example:example-dependency",
					Version: "1.2.4",
				},
			},
		},
		{
			name:      "import dependencyManagement",
			inputFile: filepath.Join("testdata", "import-dependency-management", "pom.xml"),
			local:     true,
			want: []types.Library{
				{
					Name:    "com.example:import",
					Version: "2.0.0",
				},
				{
					Name:    "org.example:example-api",
					Version: "1.7.30",
				},
			},
		},
		{
			name:      "multi module",
			inputFile: filepath.Join("testdata", "multi-module", "pom.xml"),
			local:     true,
			want: []types.Library{
				{
					Name:    "com.example:aggregation",
					Version: "1.0.0",
				},
				{
					Name:    "com.example:module",
					Version: "1.1.1",
				},
				{
					Name:    "org.example:example-api",
					Version: "1.7.30",
				},
			},
		},
		{
			name:      "multi module soft requirement",
			inputFile: filepath.Join("testdata", "multi-module-soft-requirement", "pom.xml"),
			local:     true,
			want: []types.Library{
				{
					Name:    "com.example:aggregation",
					Version: "1.0.0",
				},
				{
					Name:    "com.example:module1",
					Version: "1.1.1",
				},
				{
					Name:    "com.example:module2",
					Version: "1.1.1",
				},
				{
					Name:    "org.example:example-api",
					Version: "1.7.30",
				},
				{
					Name:    "org.example:example-api",
					Version: "2.0.0",
				},
			},
		},
		{
			name:      "parent not found",
			inputFile: filepath.Join("testdata", "not-found-parent", "pom.xml"),
			local:     true,
			wantErr:   "com.example:parent:1.0.0 was not found in local/remote repositories",
		},
		{
			name:      "dependency not found",
			inputFile: filepath.Join("testdata", "not-found-dependency", "pom.xml"),
			local:     true,
			wantErr:   "org.example:example-not-found:999 was not found in local/remote repositories",
		},
		{
			name:      "module not found",
			inputFile: filepath.Join("testdata", "not-found-module", "pom.xml"),
			local:     true,
			wantErr:   "stat testdata/not-found-module/module: no such file or directory",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, err := os.Open(tt.inputFile)
			require.NoError(t, err)
			defer f.Close()

			var remoteRepos []string
			if tt.local {
				// for local repository
				os.Setenv("MAVEN_HOME", "testdata")
				defer os.Unsetenv("MAVEN_HOME")
			} else {
				// for remote repository
				ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					paths := strings.Split(r.URL.Path, "/")
					filePath := filepath.Join(paths...)
					filePath = filepath.Join("testdata", "repository", filePath)

					f, err = os.Open(filePath)
					if err != nil {
						http.NotFound(w, r)
						return
					}
					defer f.Close()

					_, err = io.Copy(w, f)
					require.NoError(t, err)
				}))
				remoteRepos = []string{ts.URL}
			}

			p := newParser(tt.inputFile)
			p.remoteRepositories = remoteRepos

			got, err := p.Parse(f)
			if tt.wantErr != "" {
				require.NotNil(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			assert.NoError(t, err)

			sort.Slice(got, func(i, j int) bool {
				return got[i].Name < got[j].Name
			})

			assert.Equal(t, tt.want, got)
		})
	}
}
