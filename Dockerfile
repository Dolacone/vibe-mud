FROM node:22-alpine AS frontend-build
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM golang:1.22 AS backend-build
WORKDIR /src
COPY go.mod go.sum ./
RUN CGO_ENABLED=0 go mod download
COPY cmd/ ./cmd/
COPY internal/ ./internal/
RUN CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /server ./cmd/server

FROM gcr.io/distroless/static-debian12
COPY --from=backend-build /server /server
COPY --from=frontend-build /src/web/dist /web/dist
EXPOSE 8080
ENTRYPOINT ["/server"]
