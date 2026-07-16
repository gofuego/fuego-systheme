package systheme

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/gofuego/fuego-formats/dbml"
	"github.com/gofuego/fuego-formats/openapi"
	"github.com/gofuego/fuego/core"
)

// AggregateArtifacts (a BeforeRender hook) builds the whole-artifact views
// that no single page's nodes carry — the data lives spread across a tree's
// child pages, and this hook folds it back onto the root:
//
//   - OpenAPI roots gain a "paths" index (every operation, sorted by path,
//     deduplicated across tags) and the info/servers/tags/schemas lists, so
//     the root layout can render the yaml-shaped document devs already know
//     how to scan;
//   - DBML roots gain an "erd" — mermaid erDiagram source generated from the
//     table pages' columns and refs, rendered client-side.
//
// Everything written is JSON-shaped; BeforeRender mutations never touch the
// build cache.
func AggregateArtifacts(pages []*core.Page) ([]*core.Page, error) {
	bySource := map[string][]*core.Page{}
	for _, p := range pages {
		if p.Skip {
			continue
		}
		if p.Type == openapi.Type || p.Type == dbml.Type {
			bySource[p.SourcePath] = append(bySource[p.SourcePath], p)
		}
	}
	for _, group := range bySource {
		var root *core.Page
		for _, p := range group {
			if p.TreeSlugPath == "" {
				root = p
				break
			}
		}
		if root == nil {
			continue
		}
		switch root.Type {
		case openapi.Type:
			aggregateOpenAPI(root, group)
		case dbml.Type:
			root.Envelope["erd"] = buildERD(group)
		}
	}
	return pages, nil
}

// aggregateOpenAPI folds the spec back into yaml-document order on the root
// envelope: info (description from the info node), servers, tags (with
// operation counts), paths, and schemas. Hrefs are root-relative — the root
// page is where they render.
func aggregateOpenAPI(root *core.Page, group []*core.Page) {
	type op struct {
		method, path, href, summary, id string
		deprecated                      bool
	}
	seen := map[string]op{}
	for _, p := range group {
		if p.Layout != "openapi-operation" {
			continue
		}
		method, _ := p.Envelope["method"].(string)
		pth, _ := p.Envelope["path"].(string)
		key := method + " " + pth
		href := strings.TrimPrefix(p.URL, root.URL)
		if prev, ok := seen[key]; ok && prev.href <= href {
			continue // an operation appears once per tag; keep one page
		}
		summary, _ := p.Envelope["title"].(string)
		id, _ := p.Envelope["operation_id"].(string)
		deprecated, _ := p.Envelope["deprecated"].(bool)
		seen[key] = op{method, pth, href, summary, id, deprecated}
	}
	ops := make([]op, 0, len(seen))
	for _, o := range seen {
		ops = append(ops, o)
	}
	sort.Slice(ops, func(i, j int) bool {
		if ops[i].path != ops[j].path {
			return ops[i].path < ops[j].path
		}
		return ops[i].method < ops[j].method
	})
	// Grouped by path, methods nested — the shape of the yaml paths: block.
	var paths []any
	var current map[string]any
	for _, o := range ops {
		if current == nil || current["path"] != o.path {
			current = map[string]any{"path": o.path, "ops": []any{}}
			paths = append(paths, current)
		}
		current["ops"] = append(current["ops"].([]any), map[string]any{
			"method": o.method, "href": o.href, "summary": o.summary,
			"operation_id": o.id, "deprecated": o.deprecated,
		})
	}
	root.Envelope["paths"] = paths

	// The root's own nodes carry description, servers, and the tag/schema
	// refs (with slugs already resolved to hrefs by ResolveRefLinks).
	var servers, tags, schemas []any
	for _, n := range root.Nodes {
		switch n.Type {
		case openapi.NodeInfo:
			if n.Content != "" {
				root.Envelope["description"] = n.Content
			}
		case openapi.NodeServer:
			servers = append(servers, map[string]any{
				"url":         n.Attributes["url"],
				"description": n.Attributes["description"],
			})
		case openapi.NodeTagRef:
			slug, _ := n.Attributes["slug"].(string)
			count := 0
			for _, p := range group {
				if p.Layout == "openapi-operation" && strings.HasPrefix(p.TreeSlugPath, strings.Trim(slug, "/")+"/") {
					count++
				}
			}
			tags = append(tags, map[string]any{
				"name":        n.Attributes["name"],
				"href":        n.Attributes["href"],
				"description": n.Attributes["description"],
				"count":       count,
			})
		case openapi.NodeSchemaRef:
			schemas = append(schemas, map[string]any{
				"name": n.Attributes["name"],
				"href": n.Attributes["href"],
			})
		}
	}
	root.Envelope["servers"] = servers
	root.Envelope["api_tags"] = tags
	root.Envelope["schemas"] = schemas
}

var erdWord = regexp.MustCompile(`[^A-Za-z0-9_]+`)

// buildERD generates mermaid erDiagram source from a DBML tree's table
// pages: each table with its columns (PK/FK marked), then one relationship
// line per ref, deduplicated across the two endpoint pages that both carry
// it. Identifiers are sanitized to mermaid's word-shaped attribute grammar.
func buildERD(group []*core.Page) string {
	type table struct {
		name string
		rows []string
	}
	var tables []table
	fkCols := map[string]bool{} // "table.column"
	refLines := map[string]string{}

	// Refs first: FK marking needs them before columns render.
	for _, p := range group {
		for _, n := range p.Nodes {
			if n.Type != dbml.NodeRef {
				continue
			}
			from, _ := n.Attributes["from_table"].(string)
			fromCol, _ := n.Attributes["from_column"].(string)
			to, _ := n.Attributes["to_table"].(string)
			toCol, _ := n.Attributes["to_column"].(string)
			relation, _ := n.Attributes["relation"].(string)
			fkCols[from+"."+fromCol] = true
			key := from + "." + fromCol + ">" + to + "." + toCol
			var line string
			switch relation {
			case "one-to-many":
				line = fmt.Sprintf("  %s ||--o{ %s : %q", erdName(from), erdName(to), fromCol)
			case "one-to-one":
				line = fmt.Sprintf("  %s ||--|| %s : %q", erdName(from), erdName(to), fromCol)
			case "many-to-many":
				line = fmt.Sprintf("  %s }o--o{ %s : %q", erdName(from), erdName(to), fromCol)
			default: // many-to-one
				line = fmt.Sprintf("  %s ||--o{ %s : %q", erdName(to), erdName(from), fromCol)
			}
			refLines[key] = line
		}
	}

	for _, p := range group {
		if p.Layout != "dbml-table" {
			continue
		}
		name, _ := p.Envelope["title"].(string)
		t := table{name: name}
		for _, n := range p.Nodes {
			if n.Type != dbml.NodeColumn {
				continue
			}
			col, _ := n.Attributes["name"].(string)
			typ, _ := n.Attributes["type"].(string)
			pk, _ := n.Attributes["pk"].(bool)
			marker := ""
			if pk {
				marker = " PK"
			} else if fkCols[name+"."+col] {
				marker = " FK"
			}
			t.rows = append(t.rows, fmt.Sprintf("    %s %s%s", erdName(typ), erdName(col), marker))
		}
		tables = append(tables, t)
	}
	if len(tables) == 0 {
		return ""
	}
	sort.Slice(tables, func(i, j int) bool { return tables[i].name < tables[j].name })

	var b strings.Builder
	b.WriteString("erDiagram\n")
	for _, t := range tables {
		fmt.Fprintf(&b, "  %s {\n%s\n  }\n", erdName(t.name), strings.Join(t.rows, "\n"))
	}
	keys := make([]string, 0, len(refLines))
	for k := range refLines {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		b.WriteString(refLines[k] + "\n")
	}
	return b.String()
}

func erdName(s string) string {
	s = strings.Trim(erdWord.ReplaceAllString(s, "_"), "_")
	if s == "" {
		return "x"
	}
	return s
}
