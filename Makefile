.PHONY: test test-race

test:
	go test ./... -cover

test-race:
	go test -race ./...
