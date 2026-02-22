FROM golang:1.21-alpine AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/maxclaw cmd/maxclaw/main.go

FROM alpine:3.20
RUN adduser -D -h /home/maxclaw maxclaw && \
    mkdir -p /home/maxclaw/.maxclaw && \
    chown -R maxclaw:maxclaw /home/maxclaw

USER maxclaw
WORKDIR /home/maxclaw

COPY --from=builder /out/maxclaw /usr/local/bin/maxclaw

EXPOSE 18890 18791

ENTRYPOINT ["maxclaw"]
CMD ["status"]
