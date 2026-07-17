# Required Linux arm64 Obscura binaries

Both files below are required. The Docker build will fail if either file is missing.

- `obscura`
- `obscura-worker`

The CI workflow expects these exact paths:

- `deps/obscura/linux/amd64/obscura`
- `deps/obscura/linux/amd64/obscura-worker`
- `deps/obscura/linux/arm64/obscura`
- `deps/obscura/linux/arm64/obscura-worker`
