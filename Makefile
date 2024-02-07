immufluent: *.go go.* */*.go
	CGO_ENABLED=0 go build -o $@

.PHONY: docker
docker:
	docker build -t immufluent .
