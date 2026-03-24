FROM golang:1.26-alpine AS builder

WORKDIR /src

# Cache dependencies
COPY go.mod go.sum .
RUN go mod download

# Build the binary
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags='-w -s' -o /out/gidoco .

FROM alpine:3.23

RUN apk add --no-cache ca-certificates

COPY --from=builder /out/gidoco /usr/local/bin/gidoco

ENTRYPOINT ["gidoco"]
