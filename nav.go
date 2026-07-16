package systheme

import (
	"sort"
	"strings"

	"github.com/gofuego/fuego/core"
)

// BuildNavTree (a BeforeRender hook) assembles the repository file tree the
// sidebar renders on every page: directories nested as in the repo, one leaf
// per artifact file linking to its page (tree-expanded children live inside
// the artifact's own pages, not the file tree). The tree is built once and
// shared by reference on every envelope — JSON-shaped and read-only from the
// templates' side.
func BuildNavTree(pages []*core.Page) ([]*core.Page, error) {
	root := newNavDir("")
	for _, p := range pages {
		if p.Skip || p.TreeSlugPath != "" || p.URL == "" {
			continue
		}
		if strings.HasPrefix(p.RelPath, "_virtual/") || p.Type == "gitignore" ||
			p.Type == "taxonomy-term" || p.Type == "taxonomy-index" {
			continue
		}
		segs := strings.Split(p.RelPath, "/")
		dir := root
		for _, s := range segs[:len(segs)-1] {
			dir = dir.child(s)
		}
		dir.files = append(dir.files, map[string]any{
			"name": segs[len(segs)-1],
			"url":  p.URL,
			"type": p.Type,
		})
	}
	tree := root.toList()

	for _, p := range pages {
		if p.Skip {
			continue
		}
		p.Envelope["nav"] = tree
	}
	return pages, nil
}

type navDir struct {
	name  string
	dirs  map[string]*navDir
	files []map[string]any
}

func newNavDir(name string) *navDir {
	return &navDir{name: name, dirs: map[string]*navDir{}}
}

func (d *navDir) child(name string) *navDir {
	c, ok := d.dirs[name]
	if !ok {
		c = newNavDir(name)
		d.dirs[name] = c
	}
	return c
}

// toList renders a directory's entries in repo-browser order: directories
// first, alphabetical, then files alphabetical.
func (d *navDir) toList() []any {
	names := make([]string, 0, len(d.dirs))
	for n := range d.dirs {
		names = append(names, n)
	}
	sort.Strings(names)
	out := make([]any, 0, len(names)+len(d.files))
	for _, n := range names {
		out = append(out, map[string]any{
			"name":     n,
			"children": d.dirs[n].toList(),
		})
	}
	sort.Slice(d.files, func(i, j int) bool {
		a, _ := d.files[i]["name"].(string)
		b, _ := d.files[j]["name"].(string)
		return a < b
	})
	for _, f := range d.files {
		out = append(out, f)
	}
	return out
}
