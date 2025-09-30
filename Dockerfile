ARG GO_VERSION=1
FROM golang:${GO_VERSION}-trixie as builder

WORKDIR /usr/src/app
COPY go.mod go.sum ./
RUN go mod download && go mod verify
COPY . .
RUN go build -v -o /app .


FROM debian:trixie

COPY --from=builder /app /usr/local/bin/
CMD ["app"]
