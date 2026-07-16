package systheme

import (
	"fmt"
	"path"
	"regexp"
	"strings"

	"github.com/gofuego/fuego-formats/dbml"
	"github.com/gofuego/fuego-formats/formatkit"
	"github.com/gofuego/fuego/core"
	"github.com/gofuego/fuego/parsers/markdown"
)

// ResolveRefLinks (a BeforeRender hook) turns the cross-page pointers the
// parsers emit into browser-resolvable relative hrefs, so renderer templates
// — which see only the node, never .Site — can link without knowing the base
// URL:
//
//   - tree ref nodes (openapi/dbml/playwright *-ref, dbml-ref) carry a "slug"
//     relative to the tree root; the href climbs out of the current page's
//     slug path first
//   - taxonomy nodes (page-ref, term-ref) carry a site-absolute "url"
//
// Both gain an "href" attribute relative to the page they render on. It runs
// BeforeRender deliberately: URLs exist only after ROUTE, and mutations here
// never reach the build cache (which stores post-PARSE state).
func ResolveRefLinks(pages []*core.Page) ([]*core.Page, error) {
	for _, p := range pages {
		if p.Skip {
			continue
		}
		depth := 0
		if p.TreeSlugPath != "" {
			depth = strings.Count(p.TreeSlugPath, "/") + 1
		}
		for i := range p.Nodes {
			resolveNode(&p.Nodes[i], p.URL, depth)
		}
	}
	return pages, nil
}

func resolveNode(n *core.Node, pageURL string, depth int) {
	if n.Attributes != nil {
		if slug, ok := n.Attributes["slug"].(string); ok && slug != "" {
			n.Attributes["href"] = strings.Repeat("../", depth) + strings.Trim(slug, "/") + "/"
		}
		if url, ok := n.Attributes["url"].(string); ok && strings.HasPrefix(url, "/") {
			n.Attributes["href"] = relativeURL(pageURL, url)
		}
		// dbml-ref carries endpoint table names, not a slug; both endpoints
		// link via the documented slug rule (tables/ + formatkit.Slugify —
		// a stability promise of the dbml module).
		if n.Type == dbml.NodeRef {
			for _, side := range []string{"from", "to"} {
				if table, ok := n.Attributes[side+"_table"].(string); ok && table != "" {
					n.Attributes[side+"_href"] = strings.Repeat("../", depth) + "tables/" + formatkit.Slugify(table) + "/"
				}
			}
		}
	}
	for i := range n.Children {
		resolveNode(&n.Children[i], pageURL, depth)
	}
}

// relativeURL computes the relative path from one site-absolute page URL to
// another (both directory-style, trailing slash).
func relativeURL(from, to string) string {
	fromSegs := splitURL(from)
	toSegs := splitURL(to)
	common := 0
	for common < len(fromSegs) && common < len(toSegs) && fromSegs[common] == toSegs[common] {
		common++
	}
	rel := strings.Repeat("../", len(fromSegs)-common) + strings.Join(toSegs[common:], "/")
	if rel == "" {
		return "./"
	}
	if !strings.HasSuffix(rel, "/") {
		rel += "/"
	}
	return rel
}

func splitURL(u string) []string {
	u = strings.Trim(u, "/")
	if u == "" {
		return nil
	}
	return strings.Split(u, "/")
}

var hrefRe = regexp.MustCompile(`href="([^"]+)"`)

// RewriteContentLinks (a BeforeRender hook) keeps a repository's own relative
// Markdown links working on the rendered site. Content in a scanned repo is
// written for the repo view — "see [the schema](../backend/db/schema.dbml)" —
// so this hook resolves each relative href against the page's source
// location, and when the target file rendered as a page (including as a tree
// root), replaces the href with the relative URL of that page. Links to
// files that aren't pages, absolute URLs, and anchors pass through untouched.
func RewriteContentLinks(pages []*core.Page) ([]*core.Page, error) {
	// Source path → routed URL. Tree children share the root's source path;
	// keep the root (its TreeSlugPath is empty).
	byRel := make(map[string]string, len(pages))
	for _, p := range pages {
		if p.Skip || p.RelPath == "" || p.TreeSlugPath != "" {
			continue
		}
		byRel[p.RelPath] = p.URL
	}

	for _, p := range pages {
		if p.Skip || p.Type != markdown.Type {
			continue
		}
		dir := path.Dir(p.RelPath)
		for i := range p.Nodes {
			n := &p.Nodes[i]
			if !n.Raw || n.Content == "" {
				continue
			}
			n.Content = hrefRe.ReplaceAllStringFunc(n.Content, func(m string) string {
				href := m[len(`href="`) : len(m)-1]
				if strings.Contains(href, "://") || strings.HasPrefix(href, "/") ||
					strings.HasPrefix(href, "#") || strings.HasPrefix(href, "mailto:") {
					return m
				}
				target, fragment, _ := strings.Cut(href, "#")
				rel := path.Clean(path.Join(dir, target))
				url, ok := byRel[rel]
				if !ok {
					return m
				}
				newHref := relativeURL(p.URL, url)
				if fragment != "" {
					newHref += "#" + fragment
				}
				return fmt.Sprintf("href=%q", newHref)
			})
		}
	}
	return pages, nil
}
