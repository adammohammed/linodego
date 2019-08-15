#! /bin/bash -efu

new=${1:-}
if [ ! -n "$new" ]; then
	echo "Usage: $0 <new-version> [<old-version>]" >&2
	exit 1
fi

old=${2:-bleeding}

cp -r "$old" "$new"

find "$new" -name \*.yaml |
	xargs sed -i "" -e "s,\(lke.linode.com/caplke-version\): \"\($old\)\",\1: \"$new\","
