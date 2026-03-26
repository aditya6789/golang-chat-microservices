FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o /out/user-service ./services/user-service/cmd

FROM alpine:3.20
COPY --from=builder /out/user-service /usr/local/bin/user-service
EXPOSE 8082
CMD ["user-service"]

