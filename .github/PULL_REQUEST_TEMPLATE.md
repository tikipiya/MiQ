## What does this change?

<!-- Describe the outcome and link the issue if there is one. -->

## Checklist

- [ ] `gofmt -w .` produces no further changes
- [ ] `go vet ./...` passes
- [ ] `go test ./...` passes
- [ ] `go test -race ./...` passes when concurrency or caches are affected
- [ ] `CGO_ENABLED=0 go test -tags nodynamic ./...` passes when codecs or builds are affected
- [ ] Tests cover the change
- [ ] README updated if the public API changed
- [ ] Gallery comparison completed if rendering changed

<!-- Attach before/after images for intentional visual changes. -->
