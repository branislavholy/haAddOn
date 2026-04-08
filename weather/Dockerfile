ARG BUILD_FROM
FROM ${BUILD_FROM}

ARG RELEASE_TAG="1.4.17" # x-release-please-version
ARG TARGETOS
# ARG TARGETARCH

RUN echo "Binnary: https://github.com/branislavholy/haAddOn/releases/download/v${RELEASE_TAG}/weather_${TARGETOS}_${TARGETARCH}.tar.gz"

RUN ARCH=$(uname -m) && \
    wget -qO - https://github.com/branislavholy/haAddOn/releases/download/v${RELEASE_TAG}/weather_${TARGETOS}_${ARCH}.tar.gz | tar -xzvf - -C /usr/bin weather && \
    chmod a+x /usr/bin/weather

# Home Assistant add-ons usually use a simple script to start the app
COPY data/run.sh /
RUN chmod a+x /run.sh

CMD [ "/run.sh" ]
