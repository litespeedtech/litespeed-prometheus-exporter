#!/bin/bash
echo Create distribution tar
tar czfv lsws-prometheus-exporter.$VERSION.tgz dist --transform s/dist/lsws-prometheus-exporter/
