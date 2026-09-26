# portfolio wiki

Documentation for [the portfolio site](../README.md) — a single-page personal
site written in Go and htmx, served by a Go binary that embeds the whole thing.

| Page | What is in it |
| --- | --- |
| [architecture.md](architecture.md) | How the pieces fit: the Go server, embedding, caching, compression |
| [development.md](development.md) | Running it locally, project layout, editing the site |
| [deployment.md](deployment.md) | Docker, the container image, Kubernetes manifests |
| [releases.md](releases.md) | GoReleaser, GitHub Actions, cutting a release |
| [design.md](design.md) | Typography, colour, layout decisions |

## The short version

- One page of plain HTML, CSS and htmx. No build step, no `node_modules`, no
  framework.
- A ~200-line Go program embeds every file with `embed.FS` and serves them, so
  deployment is one binary and one small container image.
- Configuration is two environment variables, `ADDR` and `PORT`. There are no
  flags and no config file, by design.
- The site works with JavaScript disabled. htmx only upgrades the project
  "Details" link into an in-place expand.

## Status

Released as `v0.1.0`. Published at
`ghcr.io/vishnukumarkvs/portfolio`, multi-arch for `linux/amd64` and
`linux/arm64`.

There is no `LICENSE` file yet, so the repository is legally
all-rights-reserved. See [releases.md](releases.md#licence).
