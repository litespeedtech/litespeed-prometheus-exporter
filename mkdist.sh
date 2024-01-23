#!/bin/bash
set -x
echo Create distribution tar
git rm lsws-prometheus-exporter.*
tar czfv lsws-prometheus-exporter.$VERSION.tgz dist --transform s/dist/lsws-prometheus-exporter/
git add lsws-prometheus-exporter.*
git add . -u
git commit -m "Version $VERSION"
git push litespeed-prometheus-exporter
git tag -d "v$VERSION" && git push origin --delete "v$VERSION"
git tag -a "v$VERSION" -m "v$VERSION" && git push --tags
if [ $? -ne 0 ]; then
    echo "[ERROR] Unable to perform git tag actions on the package (version)"
    cd `dirname "$0"`
    exit 1
fi
