build:
	docker build --tag tickerd .

test: build
	docker run --name tickerd --rm -it tickerd

dist/tickerd-linux-amd64:
	docker build --output - . | tar -x tickerd
	mkdir -p dist/
	mv tickerd "$@"

release: dist/tickerd-linux-amd64

clean:
	rm -rf dist/

.PHONY: build test release clean
