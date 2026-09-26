# Local build helpers. The application itself has no flags and no config file,
# so these are just shortcuts around `go` and `docker`.

BINARY := portfolio
IMAGE  := ghcr.io/vishnukumarkvs/portfolio
TAG    := latest

.PHONY: run build test docker push clean

## run: build and serve on :8080
run: build
	./$(BINARY)

## build: compile the static binary into the working directory
build:
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o $(BINARY) .

## test: vet and run any tests
test:
	go vet ./...
	go test ./...

## docker: build the container image
docker:
	docker build -t $(IMAGE):$(TAG) .

## push: publish the image
push: docker
	docker push $(IMAGE):$(TAG)

## clean: remove build output
clean:
	rm -f $(BINARY)
