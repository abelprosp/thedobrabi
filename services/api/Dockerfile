FROM golang:1.24-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/thedobra-api ./cmd/api

FROM gcr.io/distroless/static-debian12
COPY --from=build /out/thedobra-api /thedobra-api
EXPOSE 8080
USER nonroot
ENTRYPOINT ["/thedobra-api"]
