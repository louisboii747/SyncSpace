#!/usr/bin/env bash
set -Eeuo pipefail

die() {
  printf 'verify-package: %s\n' "$*" >&2
  exit 1
}

require_command() {
  command -v "$1" >/dev/null 2>&1 || die "required command '$1' was not found"
}

skip_runtime=0
if [[ ${1:-} == --skip-runtime ]]; then
  skip_runtime=1
  shift
fi
[[ $# -eq 1 ]] || die "usage: packaging/linux/verify-package.sh [--skip-runtime] PACKAGE.deb|PACKAGE.rpm"
package=$1
[[ -f $package ]] || die "package '$package' does not exist"
require_command awk
require_command basename
require_command find
require_command grep
require_command readelf
require_command stat
require_command mktemp
require_command mkdir
require_command sh

work_dir=$(mktemp -d "${TMPDIR:-/tmp}/syncspace-package-check.XXXXXXXX")
cleanup() {
  rm -rf -- "$work_dir"
}
trap cleanup EXIT
root="$work_dir/root"
mkdir -p -- "$root"

case "$package" in
  *.deb)
    require_command dpkg-deb
    [[ $(dpkg-deb --field "$package" Package) == syncspace ]] || die "unexpected Debian package name"
    package_version=$(dpkg-deb --field "$package" Version)
    package_arch=$(dpkg-deb --field "$package" Architecture)
    package_dependencies=$(dpkg-deb --field "$package" Depends)
    case "$package_arch" in
      amd64) expected_machine='Advanced Micro Devices X86-64' ;;
      arm64) expected_machine='AArch64' ;;
      *) die "unsupported Debian architecture '$package_arch'" ;;
    esac
    expected_name="syncspace_${package_version}_${package_arch}.deb"
    [[ $(basename -- "$package") == "$expected_name" ]] || die "Debian artifact name does not match package metadata (expected $expected_name)"
    grep -Eq '(^|, )[[:space:]]*libwebkit2gtk-4\.1-0([[:space:]]|,|$)' <<<"$package_dependencies" || die "Debian package does not depend on WebKitGTK 4.1"
    if dpkg-deb --contents "$package" | awk '$2 != "root/root" { print; bad=1 } END { exit bad }'; then
      :
    else
      die "Debian payload contains a non-root owner or group"
    fi
    printf '%s %s\n' "$package_version" "$package_arch" >&2
    dpkg-deb --extract "$package" "$root"
    ;;
  *.rpm)
    require_command rpm
    require_command rpm2cpio
    require_command cpio
    [[ $(rpm --query --package --queryformat '%{NAME}' "$package") == syncspace ]] || die "unexpected RPM package name"
    package_version=$(rpm --query --package --queryformat '%{VERSION}' "$package")
    package_release=$(rpm --query --package --queryformat '%{RELEASE}' "$package")
    package_arch=$(rpm --query --package --queryformat '%{ARCH}' "$package")
    case "$package_arch" in
      x86_64) expected_machine='Advanced Micro Devices X86-64' ;;
      aarch64) expected_machine='AArch64' ;;
      *) die "unsupported RPM architecture '$package_arch'" ;;
    esac
    expected_name="syncspace-${package_version}-${package_release}.${package_arch}.rpm"
    [[ $(basename -- "$package") == "$expected_name" ]] || die "RPM artifact name does not match package metadata (expected $expected_name)"
    rpm --query --package --requires "$package" | grep -Eq '^gtk3([[:space:]]|$)' || die "RPM does not require GTK 3"
    rpm --query --package --requires "$package" | grep -Eq '^webkit2gtk4\.1([[:space:]]|$)' || die "RPM does not require WebKitGTK 4.1"
    if rpm --query --package --queryformat '[%{FILEUSERNAME} %{FILEGROUPNAME}\n]' "$package" | awk '$1 != "root" || $2 != "root" { print; bad=1 } END { exit bad }'; then
      :
    else
      die "RPM payload contains a non-root owner or group"
    fi
    printf '%s-%s %s\n' "$package_version" "$package_release" "$package_arch" >&2
    rpm2cpio "$package" | (cd -- "$root" && cpio --extract --make-directories --quiet)
    ;;
  *)
    die "package must end in .deb or .rpm"
    ;;
esac

application="$root/usr/bin/syncspace"
desktop="$root/usr/share/applications/syncspace.desktop"
metainfo="$root/usr/share/metainfo/syncspace.metainfo.xml"
icon="$root/usr/share/icons/hicolor/scalable/apps/syncspace.svg"

[[ -x $application ]] || die "desktop application is missing or is not executable"
[[ -f $desktop ]] || die "desktop entry is missing"
[[ -f $metainfo ]] || die "AppStream metadata is missing"
[[ -f $icon ]] || die "desktop icon is missing"
[[ -f $root/usr/share/licenses/syncspace/LICENSE ]] || die "license is missing"
[[ -f $root/usr/share/man/man1/syncspace.1.gz ]] || die "manual page is missing"

[[ $(stat --format='%a' "$application") == 755 ]] || die "desktop application mode must be 0755"
for regular_file in \
  "$desktop" \
  "$metainfo" \
  "$icon" \
  "$root/usr/share/licenses/syncspace/LICENSE" \
  "$root/usr/share/man/man1/syncspace.1.gz"; do
  [[ $(stat --format='%a' "$regular_file") == 644 ]] || die "$(basename -- "$regular_file") mode must be 0644"
done

if ((skip_runtime == 0)); then
  version_output=$(HOME="$work_dir/home" XDG_CONFIG_HOME="$work_dir/home/.config" "$application" --version)
  [[ $version_output == SyncSpace\ * ]] || die "desktop version smoke test failed"
  application_version=${version_output#SyncSpace }
  grep -Fq "<release version=\"$application_version\"" "$metainfo" || die "desktop executable and AppStream versions do not match"
else
  version_output='runtime check skipped for foreign architecture'
fi

desktop_machine=$(readelf --file-header "$application" | awk -F: '/^[[:space:]]*Machine:/{ sub(/^[[:space:]]+/, "", $2); print $2; exit }')
[[ $desktop_machine == "$expected_machine" ]] || die "desktop architecture '$desktop_machine' does not match package architecture '$package_arch'"

grep -Fq 'Exec=syncspace' "$desktop" || die "desktop entry does not launch the native application"
if grep -Eq 'Exec=.*(open|https?://|xdg-open)' "$desktop"; then
  die "desktop entry still launches a browser workflow"
fi
if find "$root" -path '*/etc/systemd/system/*' -o -path '*/usr/lib/systemd/system/*' | grep -q .; then
  die "package contains a system service; SyncSpace must remain per-user"
fi
if find "$root" -path '*/home/*' | grep -q .; then
  die "package writes into a user home directory"
fi

if command -v desktop-file-validate >/dev/null 2>&1; then
  desktop-file-validate "$desktop"
fi
if command -v appstreamcli >/dev/null 2>&1; then
  appstreamcli validate --no-net "$metainfo"
fi
printf 'Verified %s (%s)\n' "$package" "$version_output"
