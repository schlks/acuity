FROM golang:1.26-alpine AS builder

RUN apk add --no-cache vips-dev gcc musl-dev glycin-loaders-all

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -ldflags="-s -w" -o acuity ./cmd/acuity

FROM alpine:latest

RUN apk add --no-cache vips

WORKDIR /app
COPY --from=builder /app/acuity .

EXPOSE 3000
ENTRYPOINT ["./acuity"]
