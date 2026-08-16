FROM golang:1.25-alpine AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o /orders ./demo-services/orders

FROM alpine:latest
COPY --from=build /orders /orders
EXPOSE 9001
CMD ["/orders"]