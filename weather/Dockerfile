ARG BUILD_FROM
FROM ${BUILD_FROM}

ARG RELEASE_TAG="1.0.3" # x-release-please-version
ARG TARGETOS
ARG TARGETARCH

WORKDIR /tmp
RUN echo curl -L "https://github.com/branislavholy/haAddOn/releases/download/v${RELEASE_TAG}/weather_${TARGETOS}_${TARGETARCH}.tar.gz"
RUN curl -L "https://github.com/branislavholy/haAddOn/releases/download/v${RELEASE_TAG}/weather_${TARGETOS}_${TARGETARCH}.tar.gz" -o weather_${TARGETOS}_${TARGETARCH}.tar.gz && \
    tar -xzf weather_${TARGETOS}_${TARGETARCH}.tar.gz weather && \
    chmod a+x /tmp/weather && \
    mv /tmp/weather /usr/bin/weather

# Home Assistant add-ons usually use a simple script to start the app
COPY data/run.sh /
RUN chmod a+x /run.sh

CMD [ "/run.sh" ]
