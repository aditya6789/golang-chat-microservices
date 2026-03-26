FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o /out/auth-service ./services/auth-service/cmd

FROM alpine:3.20
WORKDIR /app
COPY --from=builder /out/auth-service /usr/local/bin/auth-service
EXPOSE 8081
CMD ["auth-service"]

