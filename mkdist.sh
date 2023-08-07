#!/bin/bash
set -x
echo Create distribution tar
git rm lsws-prometheus-exporter.*
tar czfv lsws-prometheus-exporter.$VERSION.tgz dist --transform s/dist/lsws-prometheus-exporter/
git add lsws-prometheus-exporter.*
git commit -m "Version $VERSION"
git push litespeed-prometheus-exporter
