# Two stages: build a static binary, then ship it on a distroless base (~25 MB image).
FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X github.com/navi-crwn/pixelhop/internal/app.Version=${VERSION}" -o /pixelhop ./cmd/pixelhop

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /pixelhop /pixelhop
ENV PIXELHOP_DATA=/data PIXELHOP_ADDR=:8080
VOLUME /data
EXPOSE 8080
USER nonroot
ENTRYPOINT ["/pixelhop"]
