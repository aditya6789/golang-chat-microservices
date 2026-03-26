FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o /out/notification-service ./services/notification-service/cmd

FROM alpine:3.20
COPY --from=builder /out/notification-service /usr/local/bin/notification-service
EXPOSE 8085
CMD ["notification-service"]

