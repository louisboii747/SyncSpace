#!/usr/bin/env bash
set -Eeuo pipefail

die() {
  printf 'verify-package: %s\n' "$*" >&2
  exit 1
}

require_command() {
  command -v "$1" >/dev/null 2>&1 || die "required command '$1' was not found"
}

[[ $# -eq 1 ]] || die "usage: packaging/linux/verify-package.sh PACKAGE.deb|PACKAGE.rpm"
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
    grep -Eq '(^|, )[[:space:]]*systemd([[:space:]]|,|$)' <<<"$package_dependencies" || die "Debian package does not depend on systemd"
    grep -Eq '(^|, )[[:space:]]*xdg-utils([[:space:]]|,|$)' <<<"$package_dependencies" || die "Debian package does not depend on xdg-utils"
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
    rpm --query --package --requires "$package" | grep -Eq '^systemd([[:space:]]|$)' || die "RPM does not require systemd"
    rpm --query --package --requires "$package" | grep -Eq '^xdg-utils([[:space:]]|$)' || die "RPM does not require xdg-utils"
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

launcher="$root/usr/bin/syncspace"
server="$root/usr/libexec/syncspace/syncspace-server"
unit="$root/usr/lib/systemd/user/syncspace.service"
desktop="$root/usr/share/applications/syncspace.desktop"
metainfo="$root/usr/share/metainfo/syncspace.metainfo.xml"
icon="$root/usr/share/icons/hicolor/scalable/apps/syncspace.svg"

[[ -x $launcher ]] || die "launcher is missing or is not executable"
[[ -x $server ]] || die "server is missing or is not executable"
[[ -f $unit ]] || die "systemd user unit is missing"
[[ -f $desktop ]] || die "desktop entry is missing"
[[ -f $metainfo ]] || die "AppStream metadata is missing"
[[ -f $icon ]] || die "desktop icon is missing"
[[ -f $root/usr/share/licenses/syncspace/LICENSE ]] || die "license is missing"
[[ -f $root/usr/share/doc/syncspace/service.env.example ]] || die "environment example is missing"
[[ -f $root/usr/share/man/man1/syncspace.1.gz ]] || die "manual page is missing"

[[ $(stat --format='%a' "$launcher") == 755 ]] || die "launcher mode must be 0755"
[[ $(stat --format='%a' "$server") == 755 ]] || die "server mode must be 0755"
for regular_file in \
  "$unit" \
  "$desktop" \
  "$metainfo" \
  "$icon" \
  "$root/usr/share/licenses/syncspace/LICENSE" \
  "$root/usr/share/doc/syncspace/service.env.example" \
  "$root/usr/share/man/man1/syncspace.1.gz"; do
  [[ $(stat --format='%a' "$regular_file") == 644 ]] || die "$(basename -- "$regular_file") mode must be 0644"
done

sh -n "$launcher"
version_output=$(HOME="$work_dir/home" XDG_CONFIG_HOME="$work_dir/home/.config" "$launcher" --version)
[[ $version_output == SyncSpace\ * ]] || die "launcher version smoke test failed"
launcher_version=${version_output#SyncSpace }
grep -Fq "<release version=\"$launcher_version\"" "$metainfo" || die "launcher and AppStream versions do not match"

actual_machine=$(readelf --file-header "$server" | awk -F: '/^[[:space:]]*Machine:/{ sub(/^[[:space:]]+/, "", $2); print $2; exit }')
[[ $actual_machine == "$expected_machine" ]] || die "server architecture '$actual_machine' does not match package architecture '$package_arch'"

grep -Fq 'ExecStart=/usr/libexec/syncspace/syncspace-server' "$unit" || die "unit has an unexpected executable"
grep -Fq 'Environment=SYNCSPACE_HOST=127.0.0.1' "$unit" || die "unit does not pin management to loopback"
grep -Fq 'EnvironmentFile=-%E/syncspace/service.env' "$unit" || die "unit environment file is missing"
grep -Fq 'WantedBy=default.target' "$unit" || die "unit is not enableable as a user service"
grep -Fq 'ProtectSystem=full' "$unit" || die "unit filesystem hardening does not preserve writable user data"
if grep -Eq 'SYNCSPACE_(DEV_MODE|STATIC_PEERS)' "$unit"; then
  die "production unit enables a development-only setting"
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
if command -v shellcheck >/dev/null 2>&1; then
  shellcheck "$launcher"
fi

printf 'Verified %s (%s)\n' "$package" "$version_output"
