# tickerd

A Docker process scheduler.

## Usage

```
$ tickerd -interval=1h -- run-backup.sh
```

**Dockerfile**

```dockerfile
FROM alpine

RUN apk add --no-cache bash

COPY --from=joshpeek/tickerd /usr/bin/tickerd /usr/bin/tickerd

CMD ["echo", "Hello, World!"]
ENTRYPOINT ["/usr/bin/tickerd"]

ENV TICKERD_HEALTHCHECK_FILE "/var/log/healthcheck"
HEALTHCHECK --interval=30s --timeout=3s --start-period=3s --retries=1 \
  CMD ["/usr/bin/tickerd", "-healthcheck"]
```
