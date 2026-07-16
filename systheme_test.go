package systheme

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gofuego/fuego-formats/adr"
	"github.com/gofuego/fuego-formats/dbml"
	"github.com/gofuego/fuego-formats/docker"
	"github.com/gofuego/fuego-formats/kubernetes"
	"github.com/gofuego/fuego-formats/openapi"
	"github.com/gofuego/fuego-formats/playwright"
)

// rawTypes are node types deliberately left to the engine's default renderer:
// their content is already HTML (Raw: true), so a template would only get in
// the way. Everything else a registered parser can emit must have a renderer.
var rawTypes = map[string]bool{
	"html":            true, // markdown.NodeHTML
	"mermaid-diagram": true, // pre-wrapped for client-side mermaid.js
	adr.NodePreamble:  true, // adr sections are rendered HTML; the set is
	// open-ended (adr-<heading-slug>), so none get renderers
	adr.NodeContext:      true,
	adr.NodeDecision:     true,
	adr.NodeConsequences: true,
}

// attributeTypes is every attribute-carrying node type the registered
// parsers emit — each is a schema.md contract — plus the engine's taxonomy
// nodes. Adding a format to the pack means extending this list AND adding
// the renderers; the test fails in both directions otherwise.
var attributeTypes = []string{
	openapi.NodeInfo, openapi.NodeServer, openapi.NodeTagRef,
	openapi.NodeSchemaRef, openapi.NodeOperationRef, openapi.NodeOperation,
	openapi.NodeParameter, openapi.NodeRequestBody, openapi.NodeResponse,
	openapi.NodeSchema, openapi.NodeProperty,

	dbml.NodeProject, dbml.NodeTableRef, dbml.NodeEnum, dbml.NodeTableGroup,
	dbml.NodeTable, dbml.NodeColumn, dbml.NodeIndex, dbml.NodeRef,

	playwright.NodeSuiteRef, playwright.NodeTestRef,
	playwright.NodeSuite, playwright.NodeTest,

	docker.NodeStage, docker.NodeInstruction, docker.NodeComment,

	kubernetes.NodeResourceHeader, kubernetes.NodeMetadata,
	kubernetes.NodeReplicas, kubernetes.NodePodTemplateLabels,
	kubernetes.NodeContainerSpec, kubernetes.NodeEnvRef,
	kubernetes.NodeServiceAccountRef, kubernetes.NodeVolume,
	kubernetes.NodeServiceSpec, kubernetes.NodePortMapping,
	kubernetes.NodeConfigData, kubernetes.NodeSecretData,
	kubernetes.NodeIngressRule, kubernetes.NodeSpec,

	"page-ref", "term-ref", // engine taxonomy virtual-page nodes
}

func TestRendererCoverage(t *testing.T) {
	shipped := map[string]bool{}
	entries, err := fs.ReadDir(themeFS, "theme/renderers")
	if err != nil {
		t.Fatalf("reading embedded renderers: %v", err)
	}
	for _, e := range entries {
		shipped[strings.TrimSuffix(e.Name(), ".html")] = true
	}

	for _, typ := range attributeTypes {
		if !shipped[typ] {
			t.Errorf("node type %q has no theme/renderers/%s.html", typ, typ)
		}
	}
	expected := map[string]bool{}
	for _, typ := range attributeTypes {
		expected[typ] = true
	}
	for typ := range shipped {
		if !expected[typ] {
			t.Errorf("renderer %s.html matches no known node type — stale, or missing from attributeTypes", typ)
		}
	}
	for typ := range rawTypes {
		if shipped[typ] {
			t.Errorf("node type %q is raw HTML; its renderer %s.html would shadow the default raw pass-through", typ, typ)
		}
	}
}

// TestBuildFixture builds the one-artifact-per-format fixture repo end to end
// and asserts every family produced its pages.
func TestBuildFixture(t *testing.T) {
	out := t.TempDir()
	err := Run("testdata/fixture", Options{
		SiteName:    "Fixture",
		Output:      out,
		Command:     "build",
		StrictLinks: true,
	})
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	wantPages := []string{
		"index.html",                          // dashboard
		"docs/guide/index.html",               // markdown
		"docs/flow/index.html",                // mermaid
		"api/petstore.openapi/index.html",     // openapi root
		"api/petstore.openapi/schemas/pet/index.html",
		"db/inventory/index.html",             // dbml root
		"db/inventory/tables/items/index.html",
		"e2e/checkout.spec/index.html",        // playwright root
		"dockerfiles/Dockerfile/index.html",   // docker ({dir} collapses for a root-level Dockerfile)
		"deploy/app.k8s/index.html",           // kubernetes
		"decisions/001-use-postgres.adr/index.html",
		"tags/index.html",
		"by-kind/deployment/index.html",
		"style.css", // pack static assets reach the output root
	}
	for _, p := range wantPages {
		if _, err := os.Stat(filepath.Join(out, filepath.FromSlash(p))); err != nil {
			t.Errorf("expected output page %s: %v", p, err)
		}
	}

	home, err := os.ReadFile(filepath.Join(out, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	for _, family := range []string{"API", "Data", "Tests", "Infrastructure", "Containers", "Decisions", "Diagrams", "Docs"} {
		if !strings.Contains(string(home), ">"+family+"<") {
			t.Errorf("dashboard is missing the %s family card", family)
		}
	}
}

func TestRelativeURL(t *testing.T) {
	cases := []struct{ from, to, want string }{
		{"/", "/decisions/001.adr/", "decisions/001.adr/"},
		{"/docs/overview/", "/decisions/001.adr/", "../../decisions/001.adr/"},
		{"/docs/overview/", "/docs/runbook/", "../runbook/"},
		{"/docs/overview/", "/", "../../"},
		{"/a/b/", "/a/b/", "./"},
	}
	for _, c := range cases {
		if got := relativeURL(c.from, c.to); got != c.want {
			t.Errorf("relativeURL(%q, %q) = %q, want %q", c.from, c.to, got, c.want)
		}
	}
}
