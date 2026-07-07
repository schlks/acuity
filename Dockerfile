FROM golang:1.26-alpine AS build-stage

RUN apk add --no-cache vips-dev gcc musl-dev glycin-loaders-all upx

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
RUN go install go.uber.org/mock/mockgen@latest

COPY . .
RUN go generate ./...
RUN mkdir -p ./bin
RUN go build -ldflags="-s -w" -o ./bin/acuity ./cmd/acuity
RUN upx --best --lzma ./bin/acuity

FROM build-stage AS test-stage
RUN go test -v ./...

FROM alpine:latest AS production

RUN apk add --no-cache vips exiftool

WORKDIR /app
COPY --from=build-stage /app/bin/acuity .

EXPOSE 3000
ENTRYPOINT ["./acuity"]