#!/bin/sh
set -eu
repo_root=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)
chart="$repo_root/charts/data-product-controller"
base=$(helm template controller "$chart" --namespace data-product-system --set route.enabled=true --set route.host=catalog.example)
for origin in 'https://kit.example:8443' 'http://kit.example' 'https://*.example' 'https://kit.example/' 'https://127.1'; do
	rendered=$(helm template controller "$chart" --namespace data-product-system --set route.enabled=true --set route.host=catalog.example --set uiContract.enabled=true --set uiAppearance.enabled=true --set-string "uiContract.additionalHostOrigins[0]=$origin")
	[ "$rendered" = "$base" ] || {
		echo 'Retired host approvals altered public resources' >&2
		exit 1
	}
done
printf '%s\n' 'Legacy UI host approvals are inert; API publication remains independent'
