#!/bin/sh
# Rendered from gates.sh.tmpl for acme-web: the gate list comes from vars.gates.
set -e
cd "/work/acme-web"
echo "== test"
pnpm test
echo "== types"
pnpm tsc --noEmit
echo "== e2e"
pnpm playwright test
echo "all gates green"
