# CLAUDE.md — fuego-systheme Contributor Guide

## What is fuego-systheme?

The Fuego **system theme**: a format pack (`systheme.Pack()`) plus a thin CLI
that renders a repository's engineering artifacts — OpenAPI, DBML,
Playwright, Dockerfiles, Kubernetes manifests, ADRs, Mermaid, Markdown — as
one site. It registers every fuego-formats parser and contains **no parser
code of its own**; each parser's `schema.md` in
[fuego-formats](https://github.com/gofuego/fuego-formats) is the contract the
renderers are written against.

Unlike fuego-devops there is no scanner stage: discovery runs directly over
the target repository (`ContentDir` = repo root) because the parsers claim
artifacts by filename. The non-invasive promise is absolute — no config,
theme, cache, or content is ever written into the scanned repo.

## Layout

```
fuego-systheme/
  systheme.go            Pack() + Options + Run(repoPath, Options)
  hooks.go               EnrichPages (AfterParse) + BuildOverview (Index)
  links.go               ResolveRefLinks + RewriteContentLinks (BeforeRender)
  config-defaults.yaml   ignore + routes + taxonomies (Pack.ConfigDefaults)
  cmd/fuego-systheme/    CLI binary (flags → systheme.Run)
  theme/
    base.html            shell; loads mermaid.js only on mermaid pages
    partials/topbar.html
    layouts/             home (dashboard) + one per page layout the parsers emit
    renderers/           one per attribute-carrying node type (see the test)
    static/style.css     the whole look; no build step, no Tailwind
  testdata/fixture/      one artifact per format; built end-to-end in tests
```

## The hooks (the only logic here)

- **EnrichPages** (AfterParse): defaults `Layout` for the parsers whose
  contracts deliberately omit it (markdown → `doc`, docker → `docker`,
  kubernetes → `k8s`, adr → `adr`); lifts k8s identity out of the
  `k8s-resource-header` node into the envelope (`resource_kind` feeds the
  by-kind taxonomy, `title` names the page); extracts `adr_number`.
- **BuildOverview** (Index): appends the landing page at `/` (root modules
  with unit counts, the current-decision board — accepted + open only) and
  the `/decisions/` log (every status, lifecycle order). Runs after ROUTE,
  so entries carry real URLs.
- **AggregateArtifacts** (BeforeRender): folds tree-wide data back onto tree
  roots — the OpenAPI root gains the yaml-document envelope (`paths` grouped
  by path, `api_tags`, `schemas`, `servers`, `description`); the DBML root
  gains `erd`, generated mermaid erDiagram source (columns, PK/FK, refs
  deduped across endpoint pages).
- **ComposeHome** (BeforeRender, after RewriteContentLinks): lifts the
  rendered README (links already rewritten; README sits one level deep, so
  its page-relative hrefs resolve identically from `/`) and the `.gitignore`
  text onto the landing envelope, then skips the gitignore page. The
  `.gitignore` is read by a one-node parser in `gitignore.go` — landing-page
  furniture, the deliberate exception to "no parser code here".
- **BuildNavTree** (BeforeRender, last): builds the repo file tree once
  (dirs nested, one leaf per artifact file) and shares it on every page's
  envelope for the sidebar partial, which renders it recursively.
- **ResolveRefLinks** (BeforeRender): renderer templates see only the node,
  never `.Site` — this hook converts every cross-page pointer into a
  page-relative `href` attribute (tree-ref `slug`s by climbing
  `TreeSlugPath` depth; taxonomy `page-ref`/`term-ref` absolute `url`s by
  URL diffing; `dbml-ref` endpoints via the documented
  `tables/ + formatkit.Slugify` slug rule).
- **RewriteContentLinks** (BeforeRender): repo Markdown is written for the
  GitHub view — relative links to `schema.dbml` or `001-x.adr.md` are
  resolved against the page's source path and rewritten to the target's
  rendered page. This is what makes `-strict-links` viable over an
  unmodified repo.

BeforeRender mutation is deliberate: the build cache stores post-PARSE
state, so nothing these hooks write can leak into or out of the cache.

## Conventions

- **Branch workflow:** `develop` is the default branch; `main` is protected
  (PR-only). Tag releases from `main` after the user merges.
- **Renderer coverage is enforced:** `TestRendererCoverage` fails if an
  attribute-carrying node type lacks `theme/renderers/<type>.html`, if a
  renderer matches no known type, or if a raw-HTML type grows a renderer.
  Adding a format = pack registration + `attributeTypes` entries + renderers
  + layouts + a README table row.
- **Raw types stay renderer-less:** `html`, `mermaid-diagram`, and the
  `adr-*` sections are already HTML and pass through the engine's default
  raw rendering.
- **Envelope writes must stay JSON-shaped** (cache eligibility).
- **No `<base href>`:** links are either page-relative (computed by the
  hooks) or explicitly `{{.Site.BaseURL}}`-prefixed in layouts/partials.
- **Commit trailer:** end commit messages with
  `Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>`.
- Race-clean: `go test ./... -race` before merging.
- Permissive dependencies only (no GPL) — the open-core line.

## Testing

- `go test ./...` — renderer coverage, the end-to-end fixture build
  (with `-strict-links`), and link-math unit tests.
- Manual: `go run ./cmd/fuego-systheme -site-name X ../some-repo serve`.
  The reference target is the
  [demo-fuego-systheme](https://github.com/gofuego/demo-fuego-systheme) repo,
  cloned as a sibling in the fuego workspace.

## What NOT to Do

- **Don't parse artifacts here.** If a format needs another node or
  attribute, that's a fuego-formats change (each schema.md is a versioned
  contract); this repo only renders what the modules emit.
- **Don't write into the scanned repo or cwd** — temp dirs only (see
  `Run`'s serve cache handling).
- **Don't reach into node internals from layouts.** Layouts see
  `.Page.Content` (all nodes rendered in order); per-node structure belongs
  in renderers.
- **Don't add a CSS build step.** `style.css` is hand-written on purpose.
