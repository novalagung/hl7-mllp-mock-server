FROM golang:1.22-alpine AS builder
WORKDIR /app
RUN apk add --no-cache upx
COPY go.mod go.sum ./
RUN go mod download
COPY main.go rules.json ./
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o mllpong . && upx --best mllpong

FROM scratch
COPY --from=builder /app/mllpong /mllpong
ENTRYPOINT ["/mllpong"]
