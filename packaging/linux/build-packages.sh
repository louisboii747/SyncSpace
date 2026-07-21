#!/usr/bin/env bash
set -Eeuo pipefail

script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
repo_root=$(cd -- "$script_dir/../.." && pwd)

version=""
architecture=""
format="all"
output_dir="$repo_root/dist/packages"
skip_dependency_install=0
skip_frontend_build=0

usage() {
  cat <<'EOF'
Build SyncSpace Linux packages.

Usage:
  packaging/linux/build-packages.sh --version VERSION --arch ARCH [options]

Required:
  --version VERSION     SemVer without a leading v, for example 1.4.0 or 1.4.0-rc.1
  --arch ARCH           amd64, arm64, x86_64, or aarch64

Options:
  --format FORMAT       deb, rpm, or all (default: all)
  --output DIRECTORY    Artifact directory (default: dist/packages)
  --skip-deps           Do not run npm ci before the frontend build
  --skip-frontend       Reuse the already generated embedded frontend
  --help                Show this help

SOURCE_DATE_EPOCH is honored. If it is unset, the current Git commit timestamp
is used so repeated builds from the same commit remain reproducible.
EOF
}

die() {
  printf 'build-packages: %s\n' "$*" >&2
  exit 1
}

require_command() {
  command -v "$1" >/dev/null 2>&1 || die "required command '$1' was not found"
}

while (($#)); do
  case "$1" in
    --version)
      (($# >= 2)) || die "--version needs a value"
      version=$2
      shift 2
      ;;
    --arch)
      (($# >= 2)) || die "--arch needs a value"
      architecture=$2
      shift 2
      ;;
    --format)
      (($# >= 2)) || die "--format needs a value"
      format=$2
      shift 2
      ;;
    --output)
      (($# >= 2)) || die "--output needs a value"
      output_dir=$2
      shift 2
      ;;
    --skip-deps)
      skip_dependency_install=1
      shift
      ;;
    --skip-frontend)
      skip_frontend_build=1
      shift
      ;;
    --help|-h)
      usage
      exit 0
      ;;
    *)
      die "unknown argument '$1'"
      ;;
  esac
done

[[ $version =~ ^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z]+([.-][0-9A-Za-z]+)*)?$ ]] || die "version must be SemVer such as 1.4.0 or 1.4.0-rc.1"

case "$architecture" in
  amd64|x86_64)
    go_arch=amd64
    deb_arch=amd64
    rpm_arch=x86_64
    ;;
  arm64|aarch64)
    go_arch=arm64
    deb_arch=arm64
    rpm_arch=aarch64
    ;;
  *)
    die "architecture must be amd64, arm64, x86_64, or aarch64"
    ;;
esac

case "$format" in
  deb|rpm|all) ;;
  *) die "format must be deb, rpm, or all" ;;
esac

if [[ $format == rpm || $format == all ]]; then
  require_command uname
  case "$(uname -m)" in
    x86_64|amd64) rpm_host_arch=amd64 ;;
    aarch64|arm64) rpm_host_arch=arm64 ;;
    *) die "RPM builds are unsupported on host architecture '$(uname -m)'" ;;
  esac
  if [[ $rpm_host_arch != "$go_arch" ]]; then
    die "RPM builds require a native $go_arch Linux host; use --format deb for a cross-built DEB"
  fi
fi

base_version=${version%%-*}
if [[ $version == *-* ]]; then
  prerelease=${version#*-}
  deb_version="${base_version}~${prerelease}"
  rpm_release="0.${prerelease//-/_}.1"
else
  deb_version=$base_version
  rpm_release=1
fi
rpm_version=$base_version

if [[ -z ${SOURCE_DATE_EPOCH:-} ]]; then
  require_command git
  SOURCE_DATE_EPOCH=$(git -C "$repo_root" log -1 --format=%ct)
fi
[[ $SOURCE_DATE_EPOCH =~ ^[0-9]+$ ]] || die "SOURCE_DATE_EPOCH must be a non-negative integer"
export SOURCE_DATE_EPOCH

require_command go
require_command install
require_command sed
require_command tar
require_command gzip
require_command find
require_command touch
require_command date
require_command cp
require_command du
require_command awk
require_command sort
require_command xargs
require_command md5sum
require_command mktemp
require_command chmod
require_command mkdir
if ((skip_frontend_build == 0)); then
  require_command npm
fi
if [[ $format == deb || $format == all ]]; then
  require_command dpkg-deb
fi
if [[ $format == rpm || $format == all ]]; then
  require_command rpmbuild
fi

mkdir -p -- "$output_dir"
output_dir=$(cd -- "$output_dir" && pwd)
work_dir=$(mktemp -d "${TMPDIR:-/tmp}/syncspace-package.XXXXXXXX")
cleanup() {
  rm -rf -- "$work_dir"
}
trap cleanup EXIT

if ((skip_frontend_build == 0)); then
  if ((skip_dependency_install == 0)); then
    (
      cd -- "$repo_root/frontend"
      npm_config_audit=false npm_config_fund=false npm ci --ignore-scripts
    )
  fi
  (
    cd -- "$repo_root/frontend"
    npm run build
  )
fi

[[ -f "$repo_root/backend/internal/frontend/dist/index.html" ]] || die "embedded frontend is missing; run without --skip-frontend"

binary="$work_dir/syncspace-server"
(
  cd -- "$repo_root"
  CGO_ENABLED=0 GOOS=linux GOARCH=$go_arch go build \
    -trimpath \
    -buildvcs=false \
    -ldflags "-s -w -X main.buildVersion=$version" \
    -o "$binary" \
    ./backend/cmd/server
)

stage="$work_dir/stage"
install -D -m 0755 "$binary" "$stage/usr/libexec/syncspace/syncspace-server"
sed "s/@VERSION@/$version/g" "$script_dir/assets/syncspace-launcher" > "$work_dir/syncspace-launcher"
install -D -m 0755 "$work_dir/syncspace-launcher" "$stage/usr/bin/syncspace"
install -D -m 0644 "$script_dir/assets/syncspace.service" "$stage/usr/lib/systemd/user/syncspace.service"
install -D -m 0644 "$script_dir/assets/syncspace.desktop" "$stage/usr/share/applications/syncspace.desktop"
install -D -m 0644 "$script_dir/assets/syncspace.svg" "$stage/usr/share/icons/hicolor/scalable/apps/syncspace.svg"
install -D -m 0644 "$script_dir/assets/service.env.example" "$stage/usr/share/doc/syncspace/service.env.example"
install -D -m 0644 "$repo_root/LICENSE" "$stage/usr/share/licenses/syncspace/LICENSE"

release_date=$(date --utc --date="@$SOURCE_DATE_EPOCH" +%F)
rpm_changelog_date=$(LC_ALL=C date --utc --date="@$SOURCE_DATE_EPOCH" '+%a %b %d %Y')
sed -e "s/@VERSION@/$version/g" -e "s/@RELEASE_DATE@/$release_date/g" \
  "$script_dir/assets/syncspace.metainfo.xml" > "$work_dir/syncspace.metainfo.xml"
install -D -m 0644 "$work_dir/syncspace.metainfo.xml" "$stage/usr/share/metainfo/syncspace.metainfo.xml"
sed -e "s/@VERSION@/$version/g" -e "s/@RELEASE_DATE@/$release_date/g" \
  "$script_dir/assets/syncspace.1" > "$work_dir/syncspace.1"
gzip --no-name --best --stdout "$work_dir/syncspace.1" > "$work_dir/syncspace.1.gz"
install -D -m 0644 "$work_dir/syncspace.1.gz" "$stage/usr/share/man/man1/syncspace.1.gz"

while IFS= read -r -d '' path; do
  touch -h --date="@$SOURCE_DATE_EPOCH" "$path"
done < <(find "$stage" -print0)

build_deb() {
  local package_root="$work_dir/deb-root"
  local artifact="$output_dir/syncspace_${deb_version}_${deb_arch}.deb"
  local installed_size
  cp -a -- "$stage" "$package_root"
  install -d -m 0755 "$package_root/DEBIAN"
  installed_size=$(du -sk "$package_root/usr" | awk '{print $1}')
  sed \
    -e "s/@DEB_VERSION@/$deb_version/g" \
    -e "s/@DEB_ARCH@/$deb_arch/g" \
    -e "s/@INSTALLED_SIZE@/$installed_size/g" \
    "$script_dir/deb/control.in" > "$package_root/DEBIAN/control"
  chmod 0644 "$package_root/DEBIAN/control"
  (
    cd -- "$package_root"
    find usr -type f -print0 | sort -z | xargs -0 md5sum > DEBIAN/md5sums
  )
  chmod 0644 "$package_root/DEBIAN/md5sums"
  while IFS= read -r -d '' path; do
    touch -h --date="@$SOURCE_DATE_EPOCH" "$path"
  done < <(find "$package_root" -print0)
  dpkg-deb --root-owner-group --build "$package_root" "$artifact"
  printf '%s\n' "$artifact"
}

build_rpm() {
  local top_dir="$work_dir/rpmbuild"
  local spec_file="$top_dir/SPECS/syncspace.spec"
  local built_rpm
  install -d -m 0755 "$top_dir/BUILD" "$top_dir/BUILDROOT" "$top_dir/RPMS" "$top_dir/SOURCES" "$top_dir/SPECS" "$top_dir/SRPMS"
  tar --sort=name --mtime="@$SOURCE_DATE_EPOCH" --owner=0 --group=0 --numeric-owner \
    -C "$stage" -cf "$top_dir/SOURCES/syncspace-payload.tar" .
  sed \
    -e "s/@RPM_VERSION@/$rpm_version/g" \
    -e "s/@RPM_RELEASE@/$rpm_release/g" \
    -e "s/@RPM_ARCH@/$rpm_arch/g" \
    -e "s/@RPM_CHANGELOG_DATE@/$rpm_changelog_date/g" \
    "$script_dir/rpm/syncspace.spec.in" > "$spec_file"
  rpmbuild -bb "$spec_file" \
    --target "$rpm_arch" \
    --define "_topdir $top_dir" \
    --define "_buildhost reproducible.syncspace.invalid" \
    --define "use_source_date_epoch_as_buildtime 1" \
    --define "clamp_mtime_to_source_date_epoch 1" \
    --define "source_date_epoch_from_changelog 0"
  built_rpm=$(find "$top_dir/RPMS" -type f -name '*.rpm' -print -quit)
  [[ -n $built_rpm ]] || die "rpmbuild did not produce an RPM"
  install -m 0644 "$built_rpm" "$output_dir/$(basename -- "$built_rpm")"
  printf '%s\n' "$output_dir/$(basename -- "$built_rpm")"
}

case "$format" in
  deb) build_deb ;;
  rpm) build_rpm ;;
  all)
    build_deb
    build_rpm
    ;;
esac
