FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o /out/chat-service ./services/chat-service/cmd

FROM alpine:3.20
COPY --from=builder /out/chat-service /usr/local/bin/chat-service
EXPOSE 8083
CMD ["chat-service"]

