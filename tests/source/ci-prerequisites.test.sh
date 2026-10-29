#!/usr/bin/env bash
set -euo pipefail
root=$(cd "$(dirname "$0")/../.." && pwd)
fail() {
	printf 'CI prerequisite test failed: %s\n' "$*" >&2
	exit 1
}
work=$(mktemp -d)
complete=0
cleanup() {
	local result=$?
	trap - EXIT
	rm -rf "$work"
	[[ $complete == 1 || $result != 0 ]] || result=1
	exit "$result"
}
trap cleanup EXIT
yq -o=json '.' "$root/.github/workflows/ci.yaml" >"$work/workflow.json"
jq -e '[.jobs["source-integration"].steps[] | select(.name == "Install verified Helm 3")] | length == 1' \
	"$work/workflow.json" >/dev/null || fail 'exactly one verified Helm installation is required'
jq -er '.jobs["source-integration"].steps[] | select(.name == "Install verified Helm 3") | .run | select(type == "string" and length > 0)' \
	"$work/workflow.json" >"$work/install.sh"
version=$(jq -er '.jobs["source-integration"].steps[] | select(.name == "Install verified Helm 3") | .env.HELM_VERSION' "$work/workflow.json")
checksum=$(jq -er '.jobs["source-integration"].steps[] | select(.name == "Install verified Helm 3") | .env.HELM_SHA256' "$work/workflow.json")
[[ $version =~ ^v3\.(1[3-9]|[2-9][0-9])\.[0-9]+$ && $checksum =~ ^[a-f0-9]{64}$ ]] || fail 'Helm must have an explicit compatible version and checksum'
jq -e '.jobs["source-integration"].steps | map(.name) |
 index("Install verified Helm 3") < index("Exercise the source-to-registry lifecycle")' \
	"$work/workflow.json" >/dev/null || fail 'installation must precede lifecycle execution'
mkdir -p "$work/bin" "$work/source-archive/linux-amd64"
cat >"$work/bin/curl" <<'MOCK'
#!/usr/bin/env bash
set -euo pipefail
[[ $CI_INSTALL_CASE != download ]] || exit 7
destination='' url=''
while (($#)); do
 case $1 in
 --output) destination=$2; shift 2 ;;
 https://*) url=$1; shift ;;
 *) shift ;;
 esac
done
[[ $url == "https://get.helm.sh/helm-$HELM_VERSION-linux-amd64.tar.gz" && -n $destination ]]
cp "$CI_INSTALL_ARCHIVE" "$destination"
MOCK
chmod +x "$work/bin/curl"
for scenario in healthy download checksum archive version version-status; do
	case_dir="$work/$scenario"
	mkdir "$case_dir"
	printf '#!/usr/bin/env bash\nprintf "%%s\\n" "%s+fixture"\n' "$version" >"$work/source-archive/linux-amd64/helm"
	[[ $scenario != version ]] || printf '#!/usr/bin/env bash\nprintf "v4.0.0+fixture\\n"\n' >"$work/source-archive/linux-amd64/helm"
	[[ $scenario != version-status ]] || printf 'exit 7\n' >>"$work/source-archive/linux-amd64/helm"
	chmod +x "$work/source-archive/linux-amd64/helm"
	tar -czf "$case_dir/fixture.tar.gz" -C "$work/source-archive" linux-amd64/helm
	[[ $scenario != archive ]] || printf 'not a tar archive\n' >"$case_dir/fixture.tar.gz"
	fixture_hash=$(sha256sum "$case_dir/fixture.tar.gz")
	fixture_hash=${fixture_hash%% *}
	[[ $scenario != checksum ]] || fixture_hash=0000000000000000000000000000000000000000000000000000000000000000
	: >"$case_dir/path"
	status=0
	HELM_VERSION="$version" HELM_SHA256="$fixture_hash" RUNNER_TEMP="$case_dir" GITHUB_PATH="$case_dir/path" \
		CI_INSTALL_CASE="$scenario" CI_INSTALL_ARCHIVE="$case_dir/fixture.tar.gz" PATH="$work/bin:$PATH" \
		bash "$work/install.sh" >"$case_dir/output" 2>&1 || status=$?
	if [[ $scenario == healthy ]]; then
		[[ $status == 0 && $(cat "$case_dir/path") == "$case_dir/helm-bin" ]] || {
			cat "$case_dir/output" >&2
			fail 'complete verified installation must publish its path'
		}
		[[ $("$case_dir/helm-bin/helm" version --short) == "$version+fixture" ]] || fail 'installed binary is wrong'
	else
		[[ $status != 0 && ! -s $case_dir/path ]] || fail "$scenario advertised an incomplete installation"
		case $scenario in
		version-status) [[ $status == 7 ]] ;;
		download) [[ $status == 7 ]] ;;
		checksum) grep -F 'FAILED' "$case_dir/output" >/dev/null ;;
		archive) grep -Ei 'Unrecognized archive format|not in gzip format|does not look like a tar archive' "$case_dir/output" >/dev/null ;;
		version) grep -F 'installed Helm version does not match the pinned release' "$case_dir/output" >/dev/null ;;
		esac || fail "$scenario did not reach the intended refusal"
	fi
done
complete=1
printf 'PASS: lifecycle installs verified Helm 3 before use; download, checksum, archive, version and executable failures never publish PATH\n'
