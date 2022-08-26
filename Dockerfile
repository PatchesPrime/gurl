# syntax=docker/dockerfile:1
FROM golang:1.19.0-alpine

WORKDIR /gurl
COPY ./ ./

CMD ["go", "run", "main.go"]
