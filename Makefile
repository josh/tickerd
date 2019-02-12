build:
	docker build --tag tickerd .

test: build
	docker run --name tickerd --rm -it tickerd

.PHONY: build test
