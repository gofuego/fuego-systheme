// Package systheme is the Fuego system theme: it renders the engineering
// artifacts a repository already contains — OpenAPI specs, DBML schemas,
// Playwright suites, Dockerfiles, Kubernetes manifests, ADRs, Mermaid
// diagrams, and Markdown docs — as one navigable system site. Register it on
// any Fuego engine with eng.Use(systheme.Pack()), or point the fuego-systheme
// CLI at a repository; nothing is written into the repo either way.
package systheme

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"os"

	"github.com/gofuego/fuego-formats/adr"
	"github.com/gofuego/fuego-formats/dbml"
	"github.com/gofuego/fuego-formats/docker"
	"github.com/gofuego/fuego-formats/kubernetes"
	"github.com/gofuego/fuego-formats/mermaid"
	"github.com/gofuego/fuego-formats/openapi"
	"github.com/gofuego/fuego-formats/playwright"
	"github.com/gofuego/fuego/core"
	"github.com/gofuego/fuego/engine"
	"github.com/gofuego/fuego/parsers/markdown"
)

//go:embed all:theme
var themeFS embed.FS

//go:embed config-defaults.yaml
var configDefaults []byte

// Pack returns the systheme format pack: every fuego-formats parser plus the
// engine's markdown parser (this repo contains no parser code), the system
// theme, route/taxonomy/ignore defaults, and the hooks that default layouts
// and build the overview dashboard.
func Pack() core.Pack {
	theme, _ := fs.Sub(themeFS, "theme")
	return core.Pack{
		Name: "systheme",
		Parsers: []core.Parser{
			markdown.Parser(),
			mermaid.Parser(),
			openapi.Parser(),
			dbml.Parser(),
			playwright.Parser(),
			docker.Parser(),
			kubernetes.Parser(),
			adr.Parser(),
			gitignoreParser(),
		},
		Theme:          theme,
		ConfigDefaults: configDefaults,
		Hooks: core.Hooks{
			AfterParse: []core.AfterParseHook{EnrichPages},
			Index:      []core.IndexHook{BuildOverview},
			// Order matters: link resolution first (AggregateArtifacts and
			// ComposeHome consume resolved hrefs / rewritten content), the
			// nav tree last (it spans every page, including skips decided
			// by ComposeHome).
			BeforeRender: []core.BeforeRenderHook{
				ResolveRefLinks, RewriteContentLinks, AggregateArtifacts, ComposeHome, BuildNavTree,
			},
		},
	}
}

// Options configures a fuego-systheme site build.
type Options struct {
	SiteName    string // site title (default: "System Docs")
	BaseURL     string // base URL for the site (default: "")
	Output      string // output directory (default: "build")
	Command     string // "build", "serve", or "validate" (default: "serve")
	StrictLinks bool   // fail the build on a broken internal link
}

// Run builds (or serves) the system site for a repository. Discovery runs
// over the repo itself — the parsers claim their artifacts by filename — and
// the pack supplies theme, config, and hooks, so nothing is written into the
// repository.
func Run(repoPath string, opts Options) error {
	applyOptionDefaults(&opts)

	eng := engine.New()
	eng.Use(Pack())

	bo := engine.BuildOptions{
		ContentDir:  repoPath,
		OutputDir:   opts.Output,
		SiteName:    opts.SiteName,
		BaseURL:     opts.BaseURL,
		StrictLinks: opts.StrictLinks,
	}

	ctx := context.Background()
	switch opts.Command {
	case "serve":
		// The dev server's incremental cache defaults to .fuego in the cwd;
		// keep the non-invasive promise by pointing it at a temp dir.
		cacheDir, err := os.MkdirTemp("", "fuego-systheme-cache-*")
		if err != nil {
			return fmt.Errorf("creating cache dir: %w", err)
		}
		defer os.RemoveAll(cacheDir)
		bo.CacheDir = cacheDir
		return eng.Serve(ctx, bo)
	case "validate":
		n, err := eng.Validate(ctx, bo)
		if err != nil {
			return err
		}
		fmt.Printf("valid: %d pages\n", n)
		return nil
	case "build":
		return eng.Build(ctx, bo)
	default:
		return fmt.Errorf("unknown command %q (want build, serve, or validate)", opts.Command)
	}
}

func applyOptionDefaults(opts *Options) {
	if opts.SiteName == "" {
		opts.SiteName = "System Docs"
	}
	if opts.Output == "" {
		opts.Output = "build"
	}
	if opts.Command == "" {
		opts.Command = "serve"
	}
}
