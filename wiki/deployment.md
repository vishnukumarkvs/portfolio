# Deployment

## The container image

```
ghcr.io/vishnukumarkvs/portfolio
```

| Tag | Meaning |
| --- | --- |
| `:latest` | Most recent stable release. Currently `0.1.0`. |
| `:0.1.0` | Pinned to that release. Prefer this. |

One multi-arch manifest covering `linux/amd64` and `linux/arm64`, so a node of
either architecture pulls `:latest` and gets the right image. The index also
lists two `unknown/unknown` entries; those are buildx's SBOM and provenance
attestations, and container runtimes ignore them.

Run it:

```sh
docker run --rm -p 8080:8080 ghcr.io/vishnukumarkvs/portfolio:latest
```

The image is distroless static on `nonroot`. There is no shell to exec into,
no package manager, and nothing to patch. It runs correctly under
`--read-only --user 65532:65532 --cap-drop ALL`.

## Building locally

```sh
docker build -t portfolio .
docker run --rm -p 8080:8080 portfolio
```

`Dockerfile` is multi-stage: it compiles in `golang:1.25-alpine` and copies the
static binary into `gcr.io/distroless/static-debian12:nonroot`.

There is a second Dockerfile, `Dockerfile.goreleaser`, which only copies in a
binary GoReleaser has already built. The split is deliberate — GoReleaser
cross-compiles all six targets before it builds an image, so compiling again
inside the container would be wasted work. The local Dockerfile exists for
`docker build -t portfolio .`, where there is no prebuilt binary to reuse.

Both files share the same base image, environment, port, user and entrypoint,
so the two paths cannot drift.

## Kubernetes

```sh
kubectl apply -k k8s/
kubectl rollout status deployment/portfolio
```

You do not need to build or push an image first. The manifests reference
`ghcr.io/vishnukumarkvs/portfolio:latest`, which the release workflow publishes.

### Before you apply

Two things in `k8s/` are environment-specific and will stop a deploy:

- **`ingressClassName`** in `ingress.yaml` is set to `traefik`, the k3s default.
  Change it if the cluster runs nginx or something else.
- **The `cert-manager.io/cluster-issuer` annotation** and the `portfolio-tls`
  secret. Without the secret the Ingress stays pending and serves nothing. Drop
  the annotation if cert-manager is not managing the cluster.

### What the manifests set, and why

- **One replica**, `maxSurge: 1` / `maxUnavailable: 0`. There is no state and no
  database, so a second pod would only be a second thing to patch. A rollout
  still briefly runs two pods so it never drops a request.
- **No `topologySpreadConstraints` and no pod anti-affinity.** Both exist to keep
  replicas off the same node, which is meaningless at a replica count of one.
- **`readOnlyRootFilesystem`**, all capabilities dropped, `runAsNonRoot` at
  65532, no service account token, `enableServiceLinks: false`.
- **`startupProbe`** with a generous budget, so a slow node does not get the
  liveness probe killed during boot.
- **Probes on `/healthz` and `/readyz`** rather than a Docker `HEALTHCHECK`,
  because distroless has no shell and no curl to run one with.
- **`NetworkPolicy`** allowing ingress on 8080 and DNS egress. The server makes
  no outbound requests of its own, but the page will not render correctly if
  DNS is blocked.
- **`terminationGracePeriodSeconds: 30`**, comfortably above the server's own
  10 second drain timeout, so requests finish and kube-proxy withdraws the
  endpoint before the process exits.

No HPA either. A static site serving kilobytes does not benefit from
autoscaling, and one replica is the entire availability story.

### The NetworkPolicy apiVersion

`k8s/networkpolicy.yaml` declares `apiVersion: networking.k8s.io/v1`. It must.
A bare `apiVersion: v1` is not a valid version for this kind on any cluster,
and `kubectl` rejects it with `no matches for kind NetworkPolicy in version
"v1"` — which fails the last resource of `apply -k` every time.

### Verifying without deploying

A server-side dry run validates against the real API server and writes
nothing:

```sh
kubectl apply -k k8s/ --dry-run=server
```

## As a plain static site

`index.html`, `assets/` and `fragments/` need nothing but an HTTP server, so
GitHub Pages, Cloudflare Pages, an S3 bucket or nginx all work. This is the
simplest way to host it, and the Go server is optional.

The one requirement is that `fragments/` is served alongside the root, since
the "Details" links point into it.

## Related

- [releases.md](releases.md) — how the image gets published
- [architecture.md](architecture.md) — the server that runs in the container
