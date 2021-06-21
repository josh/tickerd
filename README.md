# tickerd

A Docker process scheduler.

## Usage

```
$ tickerd -interval=1h -- run-backup.sh
```

**Dockerfile**

```dockerfile
FROM alpine:latest

RUN apk add --no-cache bash

RUN wget -O /usr/bin/tickerd https://github.com/josh/tickerd/releases/latest/download/tickerd-linux-amd64 && chmod +x /usr/bin/tickerd

CMD ["echo", "Hello, World!"]
ENTRYPOINT ["/usr/bin/tickerd"]

ENV TICKERD_HEALTHCHECK_PORT 9000
HEALTHCHECK --interval=30s --timeout=3s --start-period=3s --retries=1 \
  CMD ["/usr/bin/tickerd", "-healthcheck"]
```
