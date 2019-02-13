FROM golang:1.11.5-alpine

RUN apk add --no-cache \
    bash \
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
  -o /usr/bin/tickerd

CMD [ "--help" ]
ENTRYPOINT [ "/usr/bin/tickerd" ]
