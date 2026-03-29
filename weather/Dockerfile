ARG BUILD_FROM

FROM golang:1.26.1-alpine AS builder
WORKDIR /app
COPY data/ .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o /tmp/weather

# ARG BUILD_FROM
FROM ${BUILD_FROM}

# x-release-please-version
# ARG RELEASE_TAG="v2.0.0"
# x-release-please-end

# ARG REPO="your-username/your-repo"

# Download the correct binary for the current architecture
# (Note: Assumes your artifacts follow a naming convention like 'myprog-amd64')
# RUN ARCH=$(uname -m) && \
#     if [ "${ARCH}" = "x86_64" ]; then TARGET="amd64"; \
#     elif [ "${ARCH}" = "aarch64" ]; then TARGET="arm64"; \
#     else TARGET="armv7"; fi && \
#     # curl -L -s "https://github.com{REPO}/releases/download/${RELEASE_TAG}/myprog-${TARGET}" -o /usr/bin/my-go-app && \
#     # chmod a+x /usr/bin/my-go-app
# COPY data/weather /usr/bin/weather
COPY --from=builder /tmp/weather /usr/bin/weather
RUN chmod a+x /usr/bin/weather

# Home Assistant add-ons usually use a simple script to start the app
COPY data/run.sh /
RUN chmod a+x /run.sh

CMD [ "/run.sh" ]
