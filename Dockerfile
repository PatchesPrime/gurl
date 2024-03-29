# syntax=docker/dockerfile:1
FROM golang:1.19.0-alpine

WORKDIR /gurl
COPY ./ ./
RUN go mod download

RUN go build -o gurl

CMD ["./gurl"]
