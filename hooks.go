package systheme

import (
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/gofuego/fuego-formats/adr"
	"github.com/gofuego/fuego-formats/docker"
	"github.com/gofuego/fuego-formats/kubernetes"
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
