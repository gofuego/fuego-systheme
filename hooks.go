package systheme

import (
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/gofuego/fuego-formats/adr"
	"github.com/gofuego/fuego-formats/dbml"
	"github.com/gofuego/fuego-formats/docker"
	"github.com/gofuego/fuego-formats/kubernetes"
	"github.com/gofuego/fuego-formats/mermaid"
	"github.com/gofuego/fuego-formats/openapi"
	"github.com/gofuego/fuego-formats/playwright"
	"github.com/gofuego/fuego/core"
	"github.com/gofuego/fuego/parsers/markdown"
)

// EnrichPages (an AfterParse hook) defaults the layout for the parsers that
// deliberately emit none (markdown, docker, kubernetes, adr — their schema.md
// contracts leave layout to the consuming pack) and lifts identity out of
// node attributes into the envelope where taxonomies and listings can see it.
// All envelope values written here are JSON-shaped, so pages stay
// cache-eligible.
func EnrichPages(pages []*core.Page) ([]*core.Page, error) {
	for _, p := range pages {
		switch p.Type {
		case markdown.Type:
			if p.Layout == "" {
				p.Layout = "doc"
			}
		case docker.Type:
			if p.Layout == "" {
				p.Layout = "docker"
			}
		case kubernetes.Type:
			if p.Layout == "" {
				p.Layout = "k8s"
			}
			enrichK8s(p)
		case adr.Type:
			if p.Layout == "" {
				p.Layout = "adr"
			}
			if n := adr.ExtractADRNumber(p.RelPath); n > 0 {
				p.Envelope["adr_number"] = n
			}
		}
	}
	return pages, nil
}

// enrichK8s copies the resource identity from the k8s-resource-header node
// into the envelope: resource_kind feeds the by-kind taxonomy (mirroring what
// fuego-devops' scanner writes as frontmatter) and title names the page,
// since raw manifests carry no frontmatter at all.
func enrichK8s(p *core.Page) {
	for _, n := range p.Nodes {
		if n.Type != kubernetes.NodeResourceHeader {
			continue
		}
		kind, _ := n.Attributes["kind"].(string)
		name, _ := n.Attributes["name"].(string)
		namespace, _ := n.Attributes["namespace"].(string)
		if kind != "" {
			p.Envelope["resource_kind"] = kind
		}
		if _, ok := p.Envelope["title"]; !ok && kind != "" && name != "" {
			p.Envelope["title"] = fmt.Sprintf("%s %s", kind, name)
		}
		if namespace != "" {
			p.Envelope["k8s_namespace"] = namespace
		}
		return
	}
}

// BuildOverview (an Index hook) appends the dashboard: one virtual page at /
// summarizing every artifact family with counts and entry links. It runs
// after ROUTE, so entries carry the pages' real URLs.
func BuildOverview(pages []*core.Page) ([]*core.Page, error) {
	families := []any{
		apiFamily(pages),
		dataFamily(pages),
		testFamily(pages),
		family(pages, "Infrastructure", "manifests", func(p *core.Page) bool { return p.Type == kubernetes.Type },
			func(p *core.Page) string { s, _ := p.Envelope["resource_kind"].(string); return s }),
		family(pages, "Containers", "dockerfiles", func(p *core.Page) bool { return p.Type == docker.Type },
			func(p *core.Page) string { return path.Dir(p.RelPath) }),
		adrFamily(pages),
		family(pages, "Diagrams", "diagrams", func(p *core.Page) bool { return p.Type == mermaid.Type },
			func(p *core.Page) string { return "" }),
		family(pages, "Docs", "pages", func(p *core.Page) bool { return p.Type == markdown.Type },
			func(p *core.Page) string { return "" }),
	}

	overview := &core.Page{
		RelPath: "_virtual/overview",
		Type:    "systheme-overview",
		URL:     "/",
		Layout:  "home",
		Envelope: core.Envelope{
			"title":    "Overview",
			"families": families,
		},
	}
	return append(pages, overview), nil
}

// family builds one dashboard card: every matching page becomes an entry
// (title, URL, meta), sorted by URL for determinism.
func family(pages []*core.Page, label, unit string, match func(*core.Page) bool, meta func(*core.Page) string) map[string]any {
	var entries []any
	count := 0
	for _, p := range pages {
		if p.Skip || !match(p) {
			continue
		}
		count++
		entries = append(entries, entry(pageTitle(p), p.URL, meta(p)))
	}
	sortEntries(entries)
	return card(label, unit, count, entries)
}

// apiFamily lists each OpenAPI root and counts its operation pages.
func apiFamily(pages []*core.Page) map[string]any {
	var entries []any
	ops := 0
	for _, p := range pages {
		if p.Skip || p.Type != openapi.Type {
			continue
		}
		if p.Layout == "openapi-operation" {
			ops++
		}
		if p.TreeSlugPath == "" { // the spec's index page
			version, _ := p.Envelope["version"].(string)
			entries = append(entries, entry(pageTitle(p), p.URL, version))
		}
	}
	sortEntries(entries)
	return card("API", "operations", ops, entries)
}

// dataFamily lists each DBML schema root and counts its table pages.
func dataFamily(pages []*core.Page) map[string]any {
	var entries []any
	tables := 0
	for _, p := range pages {
		if p.Skip || p.Type != dbml.Type {
			continue
		}
		if p.Layout == "dbml-table" {
			tables++
		}
		if p.TreeSlugPath == "" {
			dbType, _ := p.Envelope["database_type"].(string)
			entries = append(entries, entry(pageTitle(p), p.URL, dbType))
		}
	}
	sortEntries(entries)
	return card("Data", "tables", tables, entries)
}

// testFamily lists each Playwright spec root and counts its test pages.
func testFamily(pages []*core.Page) map[string]any {
	var entries []any
	tests := 0
	for _, p := range pages {
		if p.Skip || p.Type != playwright.Type {
			continue
		}
		if p.Layout == "playwright-test" {
			tests++
		}
		if p.TreeSlugPath == "" {
			entries = append(entries, entry(pageTitle(p), p.URL, ""))
		}
	}
	sortEntries(entries)
	return card("Tests", "tests", tests, entries)
}

// adrFamily lists decisions ordered by number, with status as the meta.
func adrFamily(pages []*core.Page) map[string]any {
	type numbered struct {
		n    int
		e    any
	}
	var decisions []numbered
	for _, p := range pages {
		if p.Skip || p.Type != adr.Type {
			continue
		}
		status, _ := p.Envelope["status"].(string)
		n, _ := p.Envelope["adr_number"].(int)
		decisions = append(decisions, numbered{n, entry(pageTitle(p), p.URL, status)})
	}
	sort.Slice(decisions, func(i, j int) bool { return decisions[i].n < decisions[j].n })
	entries := make([]any, 0, len(decisions))
	for _, d := range decisions {
		entries = append(entries, d.e)
	}
	return card("Decisions", "ADRs", len(entries), entries)
}

func card(label, unit string, count int, entries []any) map[string]any {
	return map[string]any{
		"label":   label,
		"unit":    unit,
		"count":   count,
		"entries": entries,
	}
}

func entry(title, url, meta string) any {
	return map[string]any{"title": title, "url": url, "meta": meta}
}

func sortEntries(entries []any) {
	sort.Slice(entries, func(i, j int) bool {
		a, _ := entries[i].(map[string]any)
		b, _ := entries[j].(map[string]any)
		au, _ := a["url"].(string)
		bu, _ := b["url"].(string)
		return au < bu
	})
}

// pageTitle prefers the envelope title and falls back to the source path —
// several parsers (playwright roots, untitled diagrams) legitimately emit
// none, because a parser cannot see the filename.
func pageTitle(p *core.Page) string {
	if t, ok := p.Envelope["title"].(string); ok && t != "" {
		return t
	}
	base := path.Base(p.RelPath)
	return strings.TrimSuffix(base, path.Ext(base))
}
