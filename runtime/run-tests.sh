#!/bin/sh
set -eu
export HOME=/tmp/home
mkdir -p "$HOME"
node scripts/apply-dependency-compatibility-patches.mjs
node --test scripts/dependency-compatibility.test.mjs
npm --ignore-scripts run test --workspace @example/frontend -- --reporter=junit --reporter=json --outputFile.junit=/out/vitest.xml --outputFile.json=/out/vitest.json
