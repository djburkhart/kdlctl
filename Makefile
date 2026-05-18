.PHONY: test test-race docs-install docs-dev docs-build

test:
	go test ./... -cover

test-race:
	go test -race ./...

docs-install:
	npm --prefix docs ci

docs-dev:
	npm --prefix docs run dev

docs-build:
	npm --prefix docs run build
