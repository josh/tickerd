FROM golang:1.11.5-alpine AS builder

RUN apk add --no-cache \
    bash \
    ca-certificates \
    gcc \
    git \
    libc-dev

WORKDIR /go/src/app

ENV GO111MODULE=on
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
  -ldflags '-extldflags "-static"' \
  -o /go/bin/tickerd


FROM alpine

RUN apk add --no-cache bash

COPY --from=builder /go/bin/tickerd /usr/bin/tickerd

CMD ["echo", "Hello, World!"]
ENTRYPOINT ["/usr/bin/tickerd"]

ENV TICKERD_HEALTHCHECK_FILE "/healthcheck"
HEALTHCHECK --interval=30s --timeout=3s --start-period=3s --retries=1 \
  CMD ["/usr/bin/tickerd", "-healthcheck"]
