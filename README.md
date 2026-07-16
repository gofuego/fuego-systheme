# fuego-systheme

The **system theme** for the [Fuego](https://github.com/gofuego/fuego)
meta-engine: point it at a repository and it renders the engineering
artifacts the repo already contains as one navigable site that reads like
the repo itself. Nothing is written into the repository.

- a **repo-structure sidebar** on every page — the file tree, artifacts as
  links, current file highlighted;
- a **landing page that orients**: the root modules and what each contains,
  the decisions currently in force (superseded history one click away at
  `/decisions/`), the rendered README, and the `.gitignore` in a
  collapsible;
- the **OpenAPI index shaped like the yaml** devs already scan — `info`,
  `servers`, `tags`, `paths` with method-colored links, `components.schemas`;
- the **DBML schema as an ERD** — a mermaid entity-relationship diagram
  generated from the tables, keys, and refs, above the linked table pages.

**Demo:** [demo-fuego-systheme](https://github.com/gofuego/demo-fuego-systheme)
— a fictional AI service rendered at
[gofuego.github.io/demo-fuego-systheme](https://gofuego.github.io/demo-fuego-systheme/).

## What it renders

Every [fuego-formats](https://github.com/gofuego/fuego-formats) parser, plus
the engine's markdown parser:

| Artifact | Claimed as | Becomes |
|---|---|---|
| OpenAPI 3 specs | `*.openapi.yaml`, `openapi.yaml`, `swagger.json`, … | an API section: index, a page per tag, operation, and schema |
| DBML schemas | `*.dbml` | a schema index plus a page per table (columns, indexes, linked refs) |
| Playwright suites | `*.spec.ts`, `*.test.ts`, `.js` variants | a page per spec, suite, and test, with tags and annotations |
| Dockerfiles | `Dockerfile`, `Dockerfile.*`, `*.dockerfile` | a page per build under `/dockerfiles/`, stage by stage |
| Kubernetes manifests | `*.k8s.yaml`, `*.k8s.yml`, `*.k8s` | a page per resource, plus the `/by-kind/` taxonomy |
| ADRs | `*.adr.md` | a page per decision under `/decisions/`, with status and supersession |
| Mermaid diagrams | `*.mmd` | a page per diagram (rendered client-side) |
| Markdown docs | `*.md` | prose pages; repo-relative links to other artifacts are rewritten to their rendered pages |

The dashboard at `/` counts and links every family; `tags` from ADR
frontmatter, OpenAPI operations, and Playwright suites share one `/tags/`
taxonomy.

## Use the CLI

```bash
go run github.com/gofuego/fuego-systheme/cmd/fuego-systheme@latest \
  -site-name "My System" /path/to/repo serve

# CI / deploy:
go run github.com/gofuego/fuego-systheme/cmd/fuego-systheme@latest \
  -site-name "My System" -base-url /my-repo -strict-links -output build . build
```

Commands: `build`, `serve`, `validate`. Flags: `-site-name`, `-base-url`,
`-output`, `-strict-links`.

## Use as a pack

```go
import systheme "github.com/gofuego/fuego-systheme"

eng := engine.New()
eng.Use(systheme.Pack())
```

The pack carries the parsers, the theme, the hooks, and config defaults
(routes, the `tags`/`resource_kind` taxonomies, and an `ignore` list of
non-artifact trees — `.git`, `node_modules`, `build`, …). Your site's own
config and `theme/` files override anything the pack supplies; a file in
your `theme/renderers/` or `theme/layouts/` wins over the pack's.

To re-claim differently named files, register a parser yourself before the
pack — user-registered parsers take precedence:

```go
eng.Register(kubernetes.Parser(formatkit.WithPatterns("*.yaml")))
eng.Use(systheme.Pack())
```

## Contributing

Contributions require signing the [Contributor License Agreement](CLA.md) —
the CLA-assistant bot will prompt you on your first pull request. Work on
`develop` (the default branch); `main` is protected and updated by PR.

## License

Apache-2.0. See [LICENSE](LICENSE) and [NOTICE](NOTICE).
