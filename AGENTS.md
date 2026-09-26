# AGENTS.md

Instructions for coding agents working in this repository.

**Read [`wiki/README.md`](wiki/README.md) first.** It indexes the
documentation:

| Page | Covers |
| --- | --- |
| [wiki/architecture.md](wiki/architecture.md) | The Go server, `embed.FS`, ETags, gzip, CSP, htmx |
| [wiki/development.md](wiki/development.md) | Running locally, layout, editing the site, gotchas |
| [wiki/deployment.md](wiki/deployment.md) | Docker, the ghcr image, Kubernetes manifests |
| [wiki/releases.md](wiki/releases.md) | GoReleaser, GitHub Actions, cutting a release |
| [wiki/design.md](wiki/design.md) | Typography, colour, layout decisions |

The top-level [README.md](README.md) is deliberately three lines. Do not move
documentation back into it.

## Non-negotiables

These are design decisions, not oversights. Do not "fix" them.

- **The server has no flags and no config file.** Only `ADDR` and `PORT`, both
  validated with a fallback so a bad value degrades instead of crash-looping.
  Do not add a `--version` flag; build info is stamped via `-buildvcs` instead.
- **The site must work with JavaScript disabled.** Every `hx-get` has a real
  `href` beside it, and every file in `fragments/` is a complete standalone
  page. htmx is an upgrade, never a requirement.
- **htmx swaps into a container, never over one.** `hx-swap="innerHTML"` with
  `hx-select="article.detail"` fills a pre-existing
  `<div class="entry-more" id="more-<slug>">`. The summary is never replaced,
  which is what makes closing a project a matter of emptying that container.
- **The phone number is not published anywhere.** Not in any page, not in any
  fragment.
- **No test suite.** The user asked for none. Verify with `go vet ./...`,
  `go build ./...`, and the manual checks in
  [wiki/development.md](wiki/development.md#verifying-a-change).
- **No gradients, no cards, no shadows, white paper background.**

## Before you finish

Run these and confirm they pass. Do not report success without running them.

```sh
go vet ./...
go build ./...
goreleaser check                          # if you touched .goreleaser.yaml
kubectl kustomize k8s/ >/dev/null        # if you touched k8s/
```

If you changed the server, also confirm conditional requests still work — a
`200` there means the gzip path has regressed:

```sh
go run . &
ETAG=$(curl -sSI localhost:8080/ | grep -i etag | cut -d' ' -f2 | tr -d '\r')
curl -s -o /dev/null -w '%{http_code}\n' -H "If-None-Match: $ETAG" localhost:8080/
```

If you changed HTML or CSS, confirm the page is well-formed, every internal
link resolves, and no class is used that the stylesheet does not define.

## Verifying a release

Do not assume a workflow succeeded. Watch it:

```sh
gh run watch                             # needs `gh auth login`
```

The unauthenticated API works for a public repo and is enough to poll:

```sh
curl -sS https://api.github.com/repos/vishnukumarkvs/portfolio/actions/runs \
  | python3 -c "import json,sys; [print(r['name'],r['head_branch'],r['status'],r['conclusion']) for r in json.load(sys.stdin)['workflow_runs']]"
```

Then verify the artifacts, not just the run status — download an archive and
check it against the published `checksums.txt`, and pull the image and run it.
See [wiki/releases.md](wiki/releases.md#verifying-a-release).

## Gotchas

- **`.goreleaser.yaml` field names are version-specific.** On v2.18.2,
  `dockers_v2` uses `images` (untagged repository) plus `tags`, and labels as a
  map — not `image_templates` or `build_flag_templates`. The changelog key is
  `filters`, not `filter`. `.IsPrerelease` does not exist; the documented idiom
  is `.IsNightly`, and `.IsSnapshot` is the local equivalent. Validate with
  `goreleaser check` and a real `--snapshot` run rather than trusting the schema.
- **`-buildvcs` is a `go build` flag, not a linker flag.** It belongs under
  `flags:`, not `ldflags:`. Putting it in `ldflags` fails the link step.
- **Two Dockerfiles on purpose.** `Dockerfile` compiles; `Dockerfile.goreleaser`
  only copies a prebuilt binary and uses `$TARGETPLATFORM`. Do not merge them,
  and keep their base image, env, port, user and entrypoint identical.
- **`NetworkPolicy` needs `apiVersion: networking.k8s.io/v1`.** A bare `v1` is
  invalid on every cluster and fails the last resource of `apply -k`.
- **Use `labels` with `includeSelectors: false` in kustomization, not the
  deprecated `commonLabels`.** `commonLabels` stamps labels into immutable
  selectors.
- **Adding a `LICENSE` takes two edits** — the file, plus uncommenting
  `- LICENSE` under `archives.files` in `.goreleaser.yaml`.
- **When adding a project**, the `hx-target` id, the container id, and the
  fragment filename must all agree, and the fragment's content must sit inside
  a single `<article class="detail">`.

## Scope

Keep changes tight. The whole site is one page and about 200 lines of Go; if a
change needs more than a paragraph of justification, it probably does not
belong here.
