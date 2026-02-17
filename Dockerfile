FROM golang:1.21-alpine AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/nanobot-go cmd/nanobot/main.go

FROM alpine:3.20
RUN adduser -D -h /home/nanobot nanobot && \
    mkdir -p /home/nanobot/.nanobot && \
    chown -R nanobot:nanobot /home/nanobot

USER nanobot
WORKDIR /home/nanobot

COPY --from=builder /out/nanobot-go /usr/local/bin/nanobot-go

EXPOSE 18890 18791

ENTRYPOINT ["nanobot-go"]
CMD ["status"]
