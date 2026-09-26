# Architecture

## The shape of it

```
index.html  assets/  fragments/  404.html
        │
        │  go:embed
        ▼
   site.go ──────────►  http.ResponseWriter
        │
        ▼
    main.go ──────────►  listener, signal handling
```

Everything the site needs is compiled into the binary at build time. There is
no filesystem to ship alongside it, no asset directory to copy into a
container, and nothing to go wrong at startup because a file is missing in the
wrong place.

## `main.go` — startup and shutdown

Reads `ADDR` and `PORT` from the environment, starts the listener, and waits
for `SIGINT` or `SIGTERM`. On a signal it calls `srv.Shutdown` with a 10 second
timeout, so in-flight requests finish before the process exits.

`shutdownTimeout` is deliberately well under the Kubernetes
`terminationGracePeriodSeconds: 30`. If it were longer, the kubelet would
`SIGKILL` the process mid-drain.

Configuration resolution, in order:

1. `ADDR` set and parses as host:port → use it as-is
2. `ADDR` set but no port → append `PORT`
3. `ADDR` unset → wildcard address with `PORT`
4. `PORT` missing, non-numeric, or `0` → fall back to `8080`

Both variables are validated rather than trusted, so a malformed value produces
a working server on the default port instead of a crash loop. That matters
because a crash-looping pod in a Deployment is harder to diagnose than a pod
that came up on the wrong port.

## `site.go` — the file server

Resolves a URL path to an embedded file, then serves it. The interesting parts
are caching and compression.

### ETags, and why nothing is cached unconditionally

Every file gets a strong ETag derived from its content at startup. Responses
are served with `no-cache` or `public, max-age=0, must-revalidate`, so the
browser revalidates and gets a cheap `304` when nothing changed.

The asset filenames are stable rather than content-hashed, so an immutable
`max-age` would keep serving a stale stylesheet after a deploy. A `304` per
reload is the price of not doing that.

### Compression is decided from the path, before the handler runs

Whether to gzip is worked out from the resolved file extension, before the
handler executes. That ordering matters: it means the response headers are
final before the first byte is written, so the body never has to be buffered
just to discover its own length.

Resolving first is what makes `/` work. The site root has no extension of its
own; it resolves to `index.html`, and it is that name whose extension decides
whether the response is compressible.

Fonts are excluded. woff2 is already compressed, so gzipping it would burn CPU
to make the file marginally larger.

### The gzip writer is created lazily

A `gzip.Writer` is opened on the first body write, not up front. A `304` or a
`204` has no body, and flushing an unused gzip writer emits 18 bytes of
empty-gzip framing downstream — which commits the response as `200 OK` before
the real status is ever written. The browser then receives a bodyless `200`,
discards it, and paints a blank page.

This was a real bug. The symptom was a `superfluous response.WriteHeader` warning
in the log and a page that loaded as nothing at all on any reload, because
browsers revalidate.

### Probes

`/healthz` and `/readyz` return `200` for the Kubernetes liveness and readiness
probes. They are not gzipped, since they carry no extension and resolve to
nothing in the embedded set.

## Security headers

`securityHeaders` sets, on every response:

- `Content-Security-Policy` — same-origin only, `'unsafe-inline'` for scripts
- `X-Content-Type-Options: nosniff`
- `Referrer-Policy: strict-origin-when-cross-origin`
- `X-Frame-Options: DENY`

The `'unsafe-inline'` for scripts is the one concession. The theme bootstrap in
`index.html` has to run before first paint, or the page flashes the wrong theme
on load. It reads one key from `localStorage` and sets an attribute; there is no
way to do that from an external file in time.

`img-src` is `'self' data:`. There is no image on the page at all, so the
directive exists only to state that nothing external is expected.

## htmx, and why it is optional

The project's "Details" link is a plain `href` to a real page in `fragments/`,
plus an `hx-get` pointing at the same file. With JavaScript the detail expands
in place; without it you get an ordinary navigation to a standalone page.

Each file in `fragments/` is therefore a complete, valid HTML document. That is
what makes the no-JavaScript path work, and it is why the swap targets an inner
`article.detail` rather than the whole document.

`hx-select="article.detail"` extracts just the article from the fetched
response, and `hx-swap="innerHTML"` puts it into a container that already exists
on the page. Nothing is replaced, so closing the project is just emptying that
container — no DOM snapshot, no second request, no scroll jump, and no
`htmx.process()` call to re-bind handlers.

The alternative, swapping `outerHTML` over the whole entry, needs a snapshot of
the original markup before the swap and a `htmx.process()` after it to make the
close control work again. The container approach avoids all of that.

## Graceful degradation summary

| Without | The site still |
| --- | --- |
| JavaScript | Navigates to the real fragment page |
| The Go server | Runs on any static host; see [deployment.md](deployment.md) |
| A network | Degrades to a styled 404 |

## Related

- [development.md](development.md) — running it locally
- [design.md](design.md) — why it looks the way it does
