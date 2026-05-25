#!/usr/bin/env node
'use strict';

// Cross-platform launcher for the kratos binary.
// Used by hooks.json and command files instead of hardcoded "bin/kratos || ~/.kratos/bin/kratos" chains.
// Resolves the correct platform binary, inherits stdio so hook payloads and JSON output pass through,
// and propagates the child exit code (non-zero exits block check --init/--verify gates).
// Missing binary → silent exit 0 so the pipeline never hard-fails on an optional binary.

const { spawnSync } = require('child_process');
const { resolveBinary } = require('./kratos-bin.cjs');

const bin = resolveBinary();
if (!bin) process.exit(0);

const res = spawnSync(bin, process.argv.slice(2), { stdio: 'inherit' });
process.exit(res.status === null ? 0 : res.status);
