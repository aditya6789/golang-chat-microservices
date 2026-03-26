FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o /out/message-service ./services/message-service/cmd

FROM alpine:3.20
COPY --from=builder /out/message-service /usr/local/bin/message-service
EXPOSE 8084
CMD ["message-service"]

