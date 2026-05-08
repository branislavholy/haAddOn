ARG BUILD_FROM

FROM golang:1.26.3-alpine AS builder
WORKDIR /app
COPY data/ .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o /tmp/weather

FROM ${BUILD_FROM}

COPY --from=builder /tmp/weather /usr/bin/weather
RUN chmod a+x /usr/bin/weather

COPY data/run.sh /
RUN chmod a+x /run.sh

CMD [ "/run.sh" ]
