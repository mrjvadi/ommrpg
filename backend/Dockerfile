# Builds any backend service: docker build --build-arg SERVICE=gateway .
FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG SERVICE
RUN test -n "$SERVICE" && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/service ./services/${SERVICE}

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/service /service
USER nonroot
ENTRYPOINT ["/service"]
