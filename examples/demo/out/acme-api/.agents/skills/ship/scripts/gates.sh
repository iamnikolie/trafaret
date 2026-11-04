#!/bin/sh
# Rendered from gates.sh.tmpl for acme-api: the gate list comes from vars.gates.
set -e
cd "/work/acme-api"
echo "== test"
go test ./...
echo "== lint"
golangci-lint run
echo "all gates green"
