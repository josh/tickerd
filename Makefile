build:
	docker build --tag tickerd .

test: build
	docker run --name tickerd --rm -it tickerd

release: build
	docker create -it --name tickerd-build tickerd echo
	docker cp tickerd-build:/usr/bin/tickerd ./tickerd-linux-amd64
	docker rm tickerd-build

.PHONY: build test release
