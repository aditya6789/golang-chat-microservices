FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o /out/api-gateway ./services/api-gateway/cmd

FROM alpine:3.20
WORKDIR /app
COPY --from=builder /out/api-gateway /usr/local/bin/api-gateway
COPY docs/swagger.yaml /app/docs/swagger.yaml
EXPOSE 8080
CMD ["api-gateway"]

