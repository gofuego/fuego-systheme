package systheme

import (
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

// BuildOverview (an Index hook) appends the two virtual pages of the
// process view:
//
//   - the landing page at /, composed to orient like landing on the repo:
//     the root modules with what each contains, the decisions currently in
//     force, and (filled in by ComposeHome later) the rendered README and
//     the .gitignore collapsible;
//   - /decisions/ — the full decision log, every status, for the history
//     the landing page deliberately leaves out.
//
// It runs after ROUTE, so every entry carries the page's real URL.
func BuildOverview(pages []*core.Page) ([]*core.Page, error) {
	overview := &core.Page{
		RelPath: "_virtual/overview",
		Type:    "systheme-overview",
		URL:     "/",
		Layout:  "home",
		Envelope: core.Envelope{
			"title":     "Overview",
			"modules":   buildModules(pages),
			"decisions": buildDecisionBoard(pages),
		},
	}
	log := &core.Page{
		RelPath: "_virtual/decisions",
		Type:    "systheme-decisions",
		URL:     "/decisions/",
		Layout:  "decisions",
		Envelope: core.Envelope{
			"title":  "Decision log",
			"groups": buildDecisionLog(pages),
		},
	}
	return append(pages, overview, log), nil
}

// familyOf classifies a page for the module summaries. The unit is what a
// reader counts: operations, tables, tests — not files.
func familyOf(p *core.Page) (label, unit string, counts bool) {
	switch p.Type {
	case openapi.Type:
		return "API", "operations", p.Layout == "openapi-operation"
	case dbml.Type:
		return "Data", "tables", p.Layout == "dbml-table"
	case playwright.Type:
		return "Tests", "tests", p.Layout == "playwright-test"
	case kubernetes.Type:
		return "Infrastructure", "manifests", true
	case docker.Type:
		return "Containers", "dockerfiles", true
	case adr.Type:
		return "Decisions", "ADRs", true
	case mermaid.Type:
		return "Diagrams", "diagrams", true
	case markdown.Type:
		return "Docs", "pages", true
	}
	return "", "", false
}

// buildModules groups the artifacts by their top-level directory — the
// repo's own structure — with per-module unit counts and the artifact list.
// Root-level artifact files (a root Dockerfile, say) group under "./".
// README.md is excluded: it renders on the landing page itself.
func buildModules(pages []*core.Page) []any {
	type module struct {
		counts  map[string]int // unit → count
		entries []any
	}
	mods := map[string]*module{}
	get := func(name string) *module {
		m, ok := mods[name]
		if !ok {
			m = &module{counts: map[string]int{}}
			mods[name] = m
		}
		return m
	}

	for _, p := range pages {
		if p.Skip || strings.HasPrefix(p.RelPath, "_virtual/") || p.Type == gitignoreType {
			continue
		}
		if isReadme(p.RelPath) {
			continue
		}
		_, unit, counted := familyOf(p)
		if unit == "" {
			continue
		}
		seg, rest, cut := strings.Cut(p.RelPath, "/")
		if !cut || rest == "" {
			seg = "./"
		} else {
			seg += "/"
		}
		m := get(seg)
		if counted {
			m.counts[unit]++
		}
		if p.TreeSlugPath == "" { // one row per artifact file
			m.entries = append(m.entries, map[string]any{
				"title": pageTitle(p),
				"url":   p.URL,
				"type":  p.Type,
			})
		}
	}

	names := make([]string, 0, len(mods))
	for n := range mods {
		names = append(names, n)
	}
	sort.Strings(names)
	out := make([]any, 0, len(names))
	for _, n := range names {
		m := mods[n]
		units := make([]string, 0, len(m.counts))
		for u := range m.counts {
			units = append(units, u)
		}
		sort.Strings(units)
		chips := make([]any, 0, len(units))
		for _, u := range units {
			chips = append(chips, map[string]any{"unit": u, "count": m.counts[u]})
		}
		sortEntries(m.entries)
		out = append(out, map[string]any{
			"name":    n,
			"chips":   chips,
			"entries": m.entries,
		})
	}
	return out
}

// buildDecisionBoard selects the decisions currently in force: accepted
// (newest first) and open ones still being decided (proposed/tbd, with
// their deadlines). Superseded and deprecated appear only as a count — the
// landing page shows the present; /decisions/ keeps the history.
func buildDecisionBoard(pages []*core.Page) map[string]any {
	var accepted, open []adrEntry
	retired := 0
	for _, p := range pages {
		if p.Skip || p.Type != adr.Type {
			continue
		}
		e := newADREntry(p)
		switch e.status {
		case "accepted":
			accepted = append(accepted, e)
		case "proposed", "tbd":
			open = append(open, e)
		default: // superseded, deprecated
			retired++
		}
	}
	sort.Slice(accepted, func(i, j int) bool { return accepted[i].n > accepted[j].n })
	sort.Slice(open, func(i, j int) bool { return open[i].n > open[j].n })
	return map[string]any{
		"accepted": adrEntryList(accepted),
		"open":     adrEntryList(open),
		"retired":  retired,
	}
}

// buildDecisionLog groups every decision by status, in lifecycle order.
func buildDecisionLog(pages []*core.Page) []any {
	byStatus := map[string][]adrEntry{}
	for _, p := range pages {
		if p.Skip || p.Type != adr.Type {
			continue
		}
		e := newADREntry(p)
		byStatus[e.status] = append(byStatus[e.status], e)
	}
	var out []any
	for _, status := range []string{"accepted", "proposed", "tbd", "deprecated", "superseded"} {
		entries := byStatus[status]
		if len(entries) == 0 {
			continue
		}
		sort.Slice(entries, func(i, j int) bool { return entries[i].n < entries[j].n })
		out = append(out, map[string]any{
			"status":  status,
			"entries": adrEntryList(entries),
		})
	}
	return out
}

type adrEntry struct {
	n      int
	status string
	m      map[string]any
}

func newADREntry(p *core.Page) adrEntry {
	status, _ := p.Envelope["status"].(string)
	n, _ := p.Envelope["adr_number"].(int)
	m := map[string]any{
		"number": n,
		"title":  pageTitle(p),
		"url":    p.URL,
		"status": status,
	}
	for _, k := range []string{"date_accepted", "date_proposed", "deadline"} {
		if v, ok := p.Envelope[k].(string); ok && v != "" {
			m[k] = v
		}
	}
	if tags, ok := p.Envelope["tags"].([]string); ok && len(tags) > 0 {
		vals := make([]any, len(tags))
		for i, t := range tags {
			vals[i] = t
		}
		m["tags"] = vals
	}
	return adrEntry{n: n, status: status, m: m}
}

func adrEntryList(entries []adrEntry) []any {
	out := make([]any, len(entries))
	for i, e := range entries {
		out[i] = e.m
	}
	return out
}

func isReadme(relPath string) bool {
	return strings.EqualFold(relPath, "README.md")
}

// ComposeHome (a BeforeRender hook, registered after RewriteContentLinks)
// finishes the landing page with the repo's own voice: the rendered README
// (links already rewritten — README sits one level deep, so its
// page-relative hrefs resolve identically from /) and the .gitignore text
// for the collapsible. The .gitignore page itself is skipped — it was only
// ever parsed to end up here.
func ComposeHome(pages []*core.Page) ([]*core.Page, error) {
	var home *core.Page
	readme := ""
	gitignore := ""
	for _, p := range pages {
		switch {
		case p.Type == "systheme-overview":
			home = p
		case p.Type == markdown.Type && isReadme(p.RelPath):
			for _, n := range p.Nodes {
				if n.Raw {
					readme = n.Content
					break
				}
			}
		case p.Type == gitignoreType:
			if len(p.Nodes) > 0 {
				gitignore = p.Nodes[0].Content
			}
			p.Skip = true
		}
	}
	if home != nil {
		if readme != "" {
			home.Envelope["readme_html"] = readme
		}
		if gitignore != "" {
			home.Envelope["gitignore"] = strings.TrimSpace(gitignore)
		}
	}
	return pages, nil
}
