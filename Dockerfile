FROM golang:1.17.7-alpine AS builder

RUN apk add --no-cache \
    bash \
    gcc \
    git \
    libc-dev

WORKDIR /go/src/app

ENV GO111MODULE=on
COPY go.mod go.sum ./
RUN go mod download

ARG GOARCH

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build \
  -ldflags '-extldflags "-static"' \
  -o /usr/bin/tickerd


FROM scratch
COPY --from=builder /usr/bin/tickerd /tickerd

CMD [ "--help" ]
ENTRYPOINT [ "/tickerd" ]
