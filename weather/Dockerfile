ARG BUILD_FROM
FROM ${BUILD_FROM}

# x-release-please-version
ARG RELEASE_TAG="v1.0.0"
# x-release-please-end

WORKDIR /tmp
RUN curl -L "https://github.com/branislavholy/haAddOn/releases/download/${RELEASE_TAG}/weather_${TARGETOS}_${TARGETARCH}.tar.gz" -o weather_${TARGETOS}_${TARGETARCH}.tar.gz && \
    tar -xzf weather_${TARGETOS}_${TARGETARCH}.tar.gz weather && \
    chmod a+x /tmp/weather && \
    mv /tmp/weather /usr/bin/weather

# Home Assistant add-ons usually use a simple script to start the app
COPY data/run.sh /
RUN chmod a+x /run.sh

CMD [ "/run.sh" ]
