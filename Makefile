VERSION := 1.3.3
DOCKER_TAG := haproxy-docker-wrapper:$(VERSION)_1.8.3
PACKAGE := github.com/tuenti/haproxy-docker-wrapper
ROOT_DIR := $(shell dirname $(realpath $(lastword $(MAKEFILE_LIST))))

all:
	docker build -f Dockerfile.build -t haproxy-docker-wrapper-builder .
	docker run -v $(ROOT_DIR):/go/src/$(PACKAGE) -w /go/src/$(PACKAGE) -it --rm haproxy-docker-wrapper-builder go build -ldflags "-X main.version=$(VERSION)"

test:
	docker build -f Dockerfile.build -t haproxy-docker-wrapper-builder .
	docker run -v $(ROOT_DIR):/go/src/$(PACKAGE) -w /go/src/$(PACKAGE) -it --rm --cap-add=NET_ADMIN haproxy-docker-wrapper-builder go test -cover $(TEST_ARGS)

release:
	@if echo $(VERSION) | grep -q "dev$$" ; then echo Set VERSION variable to release; exit 1; fi
	@if git show v$(VERSION) > /dev/null 2>&1; then echo Version $(VERSION) already exists; exit 1; fi
	sed -i "s/^VERSION :=.*/VERSION := $(VERSION)/" Makefile
	git ci Makefile -m "Version $(VERSION)"
	git tag v$(VERSION) -a -m "Version $(VERSION)"

docker: all
	docker build -t $(DOCKER_TAG) .
