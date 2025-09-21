FROM golang:1.25.1-alpine3.22 AS builder

ENV CGO_ENABLED=0 \
    GO111MODULE=on

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN GO111MODULE=on CGO_ENABLED=0 GOOS=linux GOARCH=amd64 ENV=production go build -ldflags="-s -w" -trimpath -o ./app .

FROM alpine:3.22 AS runner

WORKDIR /app

RUN apk add --no-cache \
    ffmpeg \
    ca-certificates \
    libssl3

COPY --from=builder /app/app .

CMD ["./app"]
