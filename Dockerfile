FROM golang:1.25-alpine AS builder

WORKDIR /app
COPY go.mod go.sum* ./
RUN go mod download 2>/dev/null || true
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o hookrelay ./cmd/hookrelay

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=builder /app/hookrelay .
EXPOSE 8080
VOLUME ["/app/data"]
ENV HOOKRELAY_DB=/app/data/hookrelay.json
CMD ["./hookrelay"]
