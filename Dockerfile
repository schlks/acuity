FROM oven/bun:latest AS ui-builder
WORKDIR /ui
COPY ui/package.json ui/bun.lock ./
RUN bun install
COPY ui/ .
RUN bun run build

FROM golang:1.26-alpine AS go-builder
RUN apk add --no-cache vips-dev gcc musl-dev glycin-loaders-all upx vips-heif

WORKDIR /app
COPY go.mod go.sum .
RUN go mod download

COPY . .
RUN mkdir -p ./bin
RUN go build -ldflags="-s -w" -o ./bin/acuity ./cmd/acuity
RUN upx --best --lzma ./bin/acuity

FROM alpine:latest AS production

RUN apk add --no-cache vips exiftool vips-heif

WORKDIR /app
COPY --from=go-builder /app/bin/acuity .
COPY --from=ui-builder /ui/build ./web/build

ENTRYPOINT ["./acuity"]
