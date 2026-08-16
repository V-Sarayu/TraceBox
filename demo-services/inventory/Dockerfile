FROM golang:1.25-alpine AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o /inventory ./demo-services/inventory

FROM alpine:latest
COPY --from=build /inventory /inventory
EXPOSE 9002
CMD ["/inventory"]