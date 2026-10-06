#!/usr/bin/env bash
# Cross-compiles flinch for every platform a release ships, and packs each binary with the README
# and the license. The archives and a checksums file land in dist/.
set -euo pipefail

cd "$(dirname "$0")/.."
version=$(sed -n 's/^const Version = "\(.*\)"$/\1/p' internal/version/version.go)
if [ -z "$version" ]; then
	echo "no version found in internal/version/version.go" >&2
	exit 1
fi

mkdir -p dist
for target in darwin/amd64 darwin/arm64 linux/amd64 linux/arm64 windows/amd64 windows/arm64; do
	goos=${target%/*}
	goarch=${target#*/}
	name="flinch_${version}_${goos}_${goarch}"
	stage="dist/$name"
	mkdir -p "$stage"
	binary=flinch
	if [ "$goos" = windows ]; then
		binary=flinch.exe
	fi
	CGO_ENABLED=0 GOOS=$goos GOARCH=$goarch go build -trimpath -ldflags "-s -w" -o "$stage/$binary" ./cmd/flinch
	cp README.md LICENSE "$stage/"
	if [ "$goos" = windows ]; then
		(cd dist && zip -qr "$name.zip" "$name")
	else
		tar -C dist -czf "dist/$name.tar.gz" "$name"
	fi
done

(cd dist && shasum -a 256 ./*.tar.gz ./*.zip | sed 's| \./| |' > checksums.txt)
echo "$version"
