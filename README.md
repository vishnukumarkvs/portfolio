# portfolio

Single-page personal site for **Kavali Vignesh Sai Vishnu Kumar**, Site Reliability
Engineer at F5.

The site is plain HTML, CSS and [htmx](https://htmx.org). There is no frontend
build step, no `node_modules`, and no framework. A ~200-line Go program compiles
the whole thing into a single binary using `embed.FS` so it can be shipped as a
small container image.

## Design

One column, white paper, hairline rules, no cards and no shadows. Two
typefaces, both self-hosted in `assets/fonts/`:

- **Instrument Serif** for names and titles &mdash; the editorial voice
- **Instrument Sans** for everything else

Light and dark are the same layout with a different set of custom properties on
`[data-theme]`. The page is 10 KB of HTML; gzipped with the CSS and htmx it is
about 25 KB over the wire, and there is no image on the page at all.

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
k8s/                          Deployment, Service, Ingress, NetworkPolicy
```

## Running it

### Locally, with Go

```sh
go run .                       # http://localhost:8080
ADDR=:9000 go run .            # different port
```

### Locally, with Docker

```sh
docker build -t portfolio .
docker run --rm -p 8080:8080 portfolio
```

### As a plain static site

`index.html`, `assets/` and `fragments/` are self-contained and work on any
static host — GitHub Pages, Cloudflare Pages, an S3 bucket, nginx. Just serve
the directory over HTTP:

```sh
python3 -m http.server 8080
```

The Go server is a convenience, not a dependency. It exists so there is one
binary to run in Docker and in Kubernetes.

## Configuration

There are no flags and no config file. Two environment variables, both optional:

| Variable | Default  | Meaning                                        |
| -------- | -------- | ---------------------------------------------- |
| `ADDR`   | `:8080`  | Listen address, e.g. `127.0.0.1:9000`          |
| `PORT`   | `8080`   | Port, applied when `ADDR` carries no port      |

The server validates both and falls back to the default on a bad value, so a
malformed variable degrades to a working server rather than a crash loop.

### Endpoints

| Path       | Purpose                                                        |
| ---------- | -------------------------------------------------------------- |
| `/`        | The site                                                      |
| `/assets/*`| Stylesheet, scripts, favicon                                   |
| `/fragments/*` | Long-form project pages, also used as htmx fragments        |
| `/healthz` | Liveness. `200` with `ok`                                     |
| `/readyz`  | Readiness. `200` and the embedded file count                  |
| anything else | `404` with the styled `404.html` body                       |

## Behaviour worth knowing about

**htmx is optional.** The project's "Details" link is a plain `href` to a real
page in `fragments/`, plus an `hx-get` pointing at the same file. With JavaScript
the detail expands in place underneath the summary; without it you get an
ordinary navigation to a standalone page. Nothing on the site is
JavaScript-only, and expanding the project replaces nothing, so closing it is
just emptying a container &mdash; no snapshot, no second request, no scroll jump.

**Nothing is cached unconditionally.** Asset filenames are stable rather than
content-hashed, so an immutable `max-age` would keep serving a stale stylesheet
after a deploy. Every response carries a strong ETag and revalidates, which
costs one `304` per reload.

**Compression is decided from the request path** before the handler runs, so
response headers are known before anything is written and the body never needs
buffering. Fonts are excluded, since woff2 is already compressed.

**The CSP allows `'unsafe-inline'` for scripts** because the theme bootstrap has
to run before first paint to avoid a flash of the wrong theme. Everything else is
restricted to same-origin.

## Kubernetes

```sh
docker build -t ghcr.io/vishnukumarkvs/portfolio:latest .
docker push ghcr.io/vishnukumarkvs/portfolio:latest

kubectl apply -k k8s/
kubectl rollout status deployment/portfolio
```

Before applying, check two things in `k8s/`:

- `ingressClassName` in `ingress.yaml` — set to `traefik`, which is the default
  on k3s. Change it if the cluster runs nginx or something else.
- The `cert-manager.io/cluster-issuer` annotation and the `portfolio-tls`
  secret. Without the secret the Ingress stays pending and serves nothing. Drop
  the annotation if cert-manager is not managing the cluster.

What the manifests set, and why:

- Two replicas, `maxUnavailable: 0`. There is no state and no database, so
  availability is entirely about not having both replicas on one node.
- `readOnlyRootFilesystem`, all capabilities dropped, `runAsNonRoot` at 65532,
  no service account token. The distroless base has no shell to exec into.
- `startupProbe` with a generous budget so a slow node does not get the
  liveness probe killed during boot.
- `NetworkPolicy` allowing ingress on 8080 and DNS egress. The server makes no
  outbound requests of its own, but the page will not render correctly if DNS
  is blocked.
- `terminationGracePeriodSeconds: 30` so in-flight requests drain and
  kube-proxy withdraws the endpoint before the process exits.

There is no HPA. A static site serving kilobytes does not need autoscaling, and
two replicas plus a rolling update is the entire availability story.

## Notes

The site works without JavaScript and prints cleanly to PDF. There is no
hamburger menu because there are only three nav links. The page carries no
contact form and no phone number; the Articles, GitHub and LinkedIn links in
the header are the way in.
