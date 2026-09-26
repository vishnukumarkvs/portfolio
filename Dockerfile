# syntax=docker/dockerfile:1

# Build stage. The site is plain HTML, CSS and a vendored htmx, so the only
# thing to compile is the server that wraps it in embed.FS.
FROM golang:1.25-alpine AS build

WORKDIR /src

# Dependencies first so the module layer stays cached. There are none today, but
# this keeps a future import from invalidating the whole build.
COPY go.mod ./
RUN go mod download

# The embedded file set has to be present at compile time, not just at runtime.
COPY *.go ./
COPY index.html 404.html ./
COPY assets ./assets
COPY fragments ./fragments

# Static binary: no libc, so it runs on a distroless or scratch base.
# -trimpath keeps build paths out of the binary, -s -w drops the symbol table.
RUN CGO_ENABLED=0 GOOS=linux go build \
      -trimpath -ldflags="-s -w" \
      -o /out/portfolio .

# Runtime stage. distroless/static has no shell and no package manager, which
# means no shell to exec into if someone does get a foothold, and nothing to
# patch. The nonroot variant already runs as uid 65532.
FROM gcr.io/distroless/static-debian12:nonroot

# A distroless base is about 2 MB and the stripped binary about 6 MB, so the
# whole image lands in the low teens of megabytes.
COPY --from=build /out/portfolio /portfolio

# The server reads its bind address from the environment. Nothing else is
# configurable, and there are no flags.
ENV ADDR=":8080"

EXPOSE 8080

USER nonroot:nonroot

# No HEALTHCHECK here on purpose: distroless has no shell and no curl, so a
# healthcheck would need a binary in the image. Kubernetes probes the /healthz
# endpoint over HTTP instead, which is the right place for it.
ENTRYPOINT ["/portfolio"]
