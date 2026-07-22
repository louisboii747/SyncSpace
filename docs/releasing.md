# Create a SyncSpace release

This guide is for maintainers publishing Linux DEB/RPM packages and portable
Windows executables through GitHub Releases. The release workflow is the only
supported publication path; local builds are useful for inspection but are not
official releases.

## Release contract

- Stable tags use `vMAJOR.MINOR.PATCH`, for example `v1.4.0`.
- A valid SemVer prerelease suffix, such as `v1.5.0-rc.1`, creates a GitHub
  prerelease.
- A publishing tag must point to a commit contained in `origin/main`; the
  workflow rejects tags from unmerged or unrelated commits.
- Manual workflow dispatch builds and verifies artifacts but never publishes a
  GitHub Release.
- Linux artifacts are built for `amd64` and `arm64` source architectures.
- A release contains two DEBs, two RPMs, Windows x64 and Arm64 portable ZIPs,
  standalone server and CLI EXEs, platform checksum manifests, and GitHub
  artifact attestations.
- Windows artifacts are portable and currently unsigned; code signing and an
  installer remain separate release-hardening work.

The final publication job targets the `linux-release` GitHub environment.
Repository owners can add required reviewers or deployment-branch rules to
that environment when a human approval gate is wanted in addition to the tag
and test gates.

Expected stable-release assets for version `1.4.0` are:

```text
syncspace_1.4.0_amd64.deb
syncspace_1.4.0_arm64.deb
syncspace-1.4.0-1.x86_64.rpm
syncspace-1.4.0-1.aarch64.rpm
syncspace-1.4.0-windows-amd64.exe
syncspace-1.4.0-windows-arm64.exe
```

Prerelease tags keep the SemVer string inside SyncSpace but use each package
manager's native ordering convention. For example, `v1.5.0-rc.1` produces
`syncspace_1.5.0~rc.1_amd64.deb` and
`syncspace-1.5.0-0.rc.1.1.x86_64.rpm`. These sort before the eventual `1.5.0`
stable package and must not be renamed to the stable filename pattern.

The executable, package metadata, filenames, and GitHub tag all derive from the
same validated release version. Do not hand-edit one artifact to repair a
version mismatch.

## Validate before tagging

Run the normal acceptance sequence from the repository root:

```sh
go test ./...
go vet ./...
(
  cd frontend
  npm ci
  npm run check
)
go build ./backend/cmd/server ./backend/cmd/syncspace
go run ./backend/cmd/syncspace dev verify
```

On Linux, build and inspect packages locally with:

```sh
packaging/linux/build-packages.sh \
  --version 1.4.0 \
  --arch amd64 \
  --format all \
  --output dist

packaging/linux/verify-package.sh dist/syncspace_1.4.0_amd64.deb
packaging/linux/verify-package.sh dist/syncspace-1.4.0-1.x86_64.rpm
```

Repeat with `--arch arm64` on an ARM64 Linux host, or let the GitHub matrix use
its native `ubuntu-24.04-arm` runner. Building the RPM on the matching CPU
architecture avoids relabelling an x86 RPM payload as ARM. The verifier
inspects package metadata, permissions, the expected payload and service model,
then smoke-tests the extracted executable without installing the package as
root.

Local package builds require a Linux host with the repository's Go and Node.js
versions, `npm`, standard GNU packaging utilities, `dpkg-deb` for DEB output,
and `rpmbuild` for RPM output. Package verification additionally uses `rpm`,
`rpm2cpio`, `cpio`, and `readelf` from binutils. `desktop-file-validate`,
`appstreamcli`, and `shellcheck` add optional format-specific checks when they
are installed. The build script fails instead of publishing a partial format
when a requested tool is missing.

## Publish

1. Update [CHANGELOG.md](../CHANGELOG.md) with the release version and
   user-visible changes.
2. Confirm the commit intended for release has passed the normal CI workflow.
3. Create an annotated tag and push only that tag:

   ```sh
   git tag -a v1.4.0 -m "SyncSpace v1.4.0"
   git push origin v1.4.0
   ```

4. Watch the desktop release workflow through every build, package verification,
   checksum, attestation, and publication job.
5. Download the published assets into an empty directory and verify them as a
   user would:

   ```sh
   gh attestation verify ./syncspace_1.4.0_amd64.deb \
     --repo louisboii747/syncspace
   gh attestation verify ./syncspace-1.4.0-1.x86_64.rpm \
     --repo louisboii747/syncspace
   ```

6. Install one DEB and one RPM on clean supported distributions. Confirm that
   `syncspace` opens one native window without a browser, a second launch
   focuses it, privacy acceptance is required, closing the window stops its
   embedded backend, and uninstall leaves user data intact.

Do not publish checksums produced before the final asset aggregation. Do not
replace a file attached to an existing tag: create a new patch release so the
tag, provenance, package metadata, and checksum remain an auditable unit.

## What GitHub verifies

The `.github/workflows/release-linux.yml` workflow uses least-privilege
permissions, runs frontend and Go quality gates, builds each package twice to
detect non-deterministic output, verifies every package, exercises each
extracted native app over its real health endpoint, and records signed GitHub
build-provenance attestations before publishing only end-user application assets.

## Windows release hardening

The workflow builds version-stamped x64 and Arm64 PE executables, checks their
machine type, smoke-tests the packaged runtime, and records GitHub provenance.
It does not yet Authenticode-sign the binaries or
create an installer. Add those only with protected signing credentials and a
tested upgrade/uninstall path; never imply that the current portable files are
signed.
