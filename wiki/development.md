# Development

## Running it locally

With Go:

```sh
go run .                       # http://localhost:8080
ADDR=:9000 go run .            # different port
```

With Docker:

```sh
docker build -t portfolio .
docker run --rm -p 8080:8080 portfolio
```

As a plain static site, with no Go involved at all:

```sh
python3 -m http.server 8080
```

`index.html`, `assets/` and `fragments/` are self-contained, so they work on
any static host — GitHub Pages, Cloudflare Pages, an S3 bucket, nginx. The Go
server is a convenience, not a dependency: it exists so there is one binary to
run in Docker and in Kubernetes.

The Makefile has the same commands as shortcuts:

```sh
make run       # go run .
make build     # build ./portfolio
make docker    # build the image
make clean     # remove build output
```

## Layout

```
index.html                    the site
404.html                      served for unknown paths
assets/
  style.css                   one stylesheet, both themes
  app.js                      theme toggle, project collapse
  htmx.min.js                 vendored, v2.0.4
  favicon.svg
  fonts/                      Instrument Serif + Instrument Sans, woff2 subsets
fragments/
  project-prom-viewer.html    long-form page for the one featured project
main.go                       startup, env config, graceful shutdown
site.go                       embedded file server: ETags, gzip, headers, probes
Dockerfile                    multi-stage, distroless nonroot
Dockerfile.goreleaser         runtime-only, copies a prebuilt binary
k8s/                          Deployment, Service, Ingress, NetworkPolicy
```

## Editing the site

Everything is hand-written. There is no bundler, transpiler, or minifier, and
no `package.json`.

- **Content** — edit `index.html`. Sections are `work`, `projects`, `stack`.
- **Styles** — `assets/style.css`. The design tokens are custom properties at
  the top; `--shell` sets page width and `--read` sets line length. See
  [design.md](design.md).
- **Behaviour** — `assets/app.js`, about 40 lines: the theme toggle and
  collapsing a project.
- **A project write-up** — add a file to `fragments/`. It must be a complete
  standalone page, and its body must live in a single `<article class="detail">`
  for htmx to select it. Then add an entry in `index.html` with
  `hx-target="#more-<your-slug>"` and a matching `<div class="entry-more"
  id="more-<your-slug>">`.

That last point is the one that bites. The `hx-target` id and the container id
have to agree, and the container has to already exist on the page, because
`hx-swap="innerHTML"` fills it rather than replacing it.

## Adding a file to the embedded set

`//go:embed` in `site.go` lists directories, not individual files:

```go
//go:embed index.html 404.html assets fragments
```

Anything dropped into `assets/` or `fragments/` is picked up automatically.
A new top-level file needs adding to that line, and the build will fail loudly
if the pattern matches nothing — which is the intended safety net.

## Verifying a change

There is no test suite. What there is:

```sh
go vet ./...                    # must be clean
go build ./...
goreleaser check                # if you touched release config
```

For a change to the server, the thing worth checking is that a conditional
request still returns a real `304` with no body:

```sh
go run . &
ETAG=$(curl -sSI localhost:8080/ | grep -i etag | cut -d' ' -f2 | tr -d '\r')
curl -s -o /dev/null -w '%{http_code}\n' -H "If-None-Match: $ETAG" localhost:8080/
```

A `200` there means the gzip path is broken again, in the way described in
[architecture.md](architecture.md#the-gzip-writer-is-created-lazily).

If you change the HTML, check that the page is still well-formed and that every
internal link resolves. A quick loop:

```sh
grep -oE '(href|src|hx-get)="[^"]+"' index.html fragments/*.html 404.html
```

## Things that will bite you

- **The project directory is not a git-ignored dumping ground.** `dist/` and
  the `portfolio` binary are ignored; anything else you leave behind gets
  committed.
- **Hard-refresh after a server fix.** The browser will have cached a bad
  response, and a normal reload may revalidate against it.
- **The phone number is deliberately not published.** Do not add it back to any
  page.
- **Adding a `LICENSE` requires two edits** — create the file, then uncomment
  the `- LICENSE` line in the `archives.files` list in `.goreleaser.yaml`.
  Otherwise it will not appear in release archives.
