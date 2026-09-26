# Releases

## Cutting one

```sh
git tag -a v0.1.0 -m "release notes"
git push origin v0.1.0
```

Pushing a `v*` tag is the only trigger. The workflow builds everything and
publishes it. Do not move a published tag afterwards — the release is already
attached to it, and force-pushing leaves the two out of step.

## What a tag produces

**A GitHub Release** with six binary archives and `checksums.txt`:

```
portfolio_0.1.0_linux_amd64.tar.gz     portfolio_0.1.0_darwin_amd64.tar.gz
portfolio_0.1.0_linux_arm64.tar.gz     portfolio_0.1.0_darwin_arm64.tar.gz
portfolio_0.1.0_windows_amd64.tar.gz   portfolio_0.1.0_windows_arm64.tar.gz
```

**A container image** at `ghcr.io/vishnukumarkvs/portfolio`, tagged at the
release version and at `latest`, as one manifest covering `linux/amd64` and
`linux/arm64`. A prerelease never claims `latest`, so the floating tag cannot
end up pointing at a release candidate.

| File | What it does |
| --- | --- |
| `.goreleaser.yaml` | Targets, archive naming, checksums, release notes, image |
| `.github/workflows/release.yml` | Runs on a `v*` tag. Publishes the release and image |
| `.github/workflows/ci.yml` | Runs on every push and PR. Publishes nothing |

## The workflows

`release.yml` needs two things beyond checkout and Go setup, and both are
load-bearing:

- **`docker/setup-buildx-action`** — buildx cannot assemble a multi-arch
  manifest without a multi-platform builder, and a GitHub runner's default
  builder is host-platform only.
- **`docker/login-action`** against `ghcr.io`, authenticating with the automatic
  `GITHUB_TOKEN`. The username is the repository owner; the password is the
  token.

Permissions are `contents: write` and `packages: write`, granted to that one
job and nothing else. There is no PAT to create and no secret to store.

`ci.yml` runs `--skip=docker`. A snapshot build of both platforms would need a
multi-platform builder and a way to load an arm64 image onto an amd64 runner,
which is the release job's concern. CI still catches a broken
`.goreleaser.yaml`, a bad `Dockerfile.goreleaser` path, and any build or
archive regression.

## Locally

```sh
goreleaser check                            # validate the config
goreleaser release --snapshot --clean       # build everything, publish nothing
```

`--snapshot` writes archives, checksums and both platform images to `dist/`
without creating a release. It adds a platform suffix to each image tag, so
you get `:1.2.3-amd64` and `:1.2.3-arm64` rather than a manifest, because
buildx will not build a multi-arch manifest without pushing it.

Requires a clean tree and a tag; GoReleaser will not build from a dirty
checkout.

## Two deliberate choices

**No `-version` flag.** The binary has no flags at all, by design. Instead
`-buildvcs` stamps Go's own build info, so a binary can be traced to its source
without adding a CLI surface:

```sh
go version -m ./portfolio | grep vcs
```

```
build   vcs.revision=365fee6d76a7ef99830a1af2db83c53c42e6b3c1
build   vcs.time=2026-09-26T08:44:37Z
build   vcs.modified=false
```

**Builds are reproducible.** `-trimpath` plus `mod_timestamp` pinned to the
commit timestamp means the same tag produces the same bytes.

## Cost

Nothing. GitHub Actions and GitHub Releases are free for a public repository,
and the included monthly minute allowance does not apply to public repos.
GoReleaser's open-source version is Apache-2.0. GoReleaser Pro is a paid
product and is not used here.

## Licence

There is no `LICENSE` file yet, so the repository and its releases are legally
all-rights-reserved. Add one before treating a release as something anyone else
may build on.

Two edits are needed, not one: create the file, then uncomment the
`- LICENSE` line under `archives.files` in `.goreleaser.yaml`. Without the
second, it will not appear in release archives.

## Verifying a release

```sh
gh release download v0.1.0 --pattern 'checksums.txt' --dir /tmp/sums
gh release download v0.1.0 --pattern '*darwin_arm64*' --dir /tmp/sums
cd /tmp/sums && shasum -a 256 -c checksums.txt
```

And the image, which is the artefact a deploy actually depends on:

```sh
docker run --rm -d --name pf -p 8080:8080 \
  --read-only --user 65532:65532 --cap-drop ALL \
  ghcr.io/vishnukumarkvs/portfolio:latest
curl -sI localhost:8080/ | head -1
docker rm -f pf
```

## Related

- [deployment.md](deployment.md) — running the image
