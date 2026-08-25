FROM golang:1.22 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN CGO_ENABLED=0 go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /server ./cmd/server

FROM gcr.io/distroless/static-debian12
COPY --from=build /server /server
EXPOSE 8080
ENTRYPOINT ["/server"]
