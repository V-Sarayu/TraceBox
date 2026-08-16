FROM golang:1.25-alpine AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o /tracebox-server ./cmd/server

FROM alpine:latest
COPY --from=build /tracebox-server /tracebox-server
EXPOSE 8080
CMD ["/tracebox-server"]