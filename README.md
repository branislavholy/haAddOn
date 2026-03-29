![version](https://img.shields.io/static/v1?label=version&message=2.1.0&color=blue) <!-- x-release-please-version -->

# Weather Add-On

## Running Go

```bash
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go run main.go
```

## Building Go Binary

```bash
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build
```

## Building Docker Image

```bash
cd ~/workspace/haAddOnTest/weather
podman build --format docker --pull --rm --build-arg BUILD_FROM="ghcr.io/home-assistant/aarch64-base:latest" -t weather:latest -t 'weather:2.0.0' .

```

## Listing Container Images

```bash
podman images
podman images localhost/weather
```

## Running Container

### Interactive Shell
```bash
podman run --rm -it weather:latest bash
```

### With Environment Variables
```bash
podman run --rm --name weather-container -p 8000:8000 \
  -e HA_LANGUAGE=en \
  -e HA_UNITS=metric \
  -e MQTT_USERNAME=$MQTT_USERNAME \
  -e MQTT_PASSWORD=$MQTT_PASSWORD \
  weather:latest
```

### Test Binary Directly
```bash
podman run --rm -it weather:latest /usr/bin/weather
```

### Remove image
```bash
podman rmi weather:2.0.0 weather:latest
```

## Building Alternative Image

```bash
podman build --pull --rm -f 'workspace/haAddOnTest/weather/Dockerfile' -t 'branislav:latest' 'workspace/haAddOnTest/weather'
podman run --rm -v /home/branislav/workspace/haAddOnTest/weather/data:/data -p 8000:80  localhost/branislav:latest

podman run --rm -v /home -p 8000:80 -it localhost/test:latest bash
https://github.com/home-assistant/cli/blob/master/.github/workflows/build.yml

      matrix:
        variant:
          - {"name": "ha_i386", "args": "GOARCH=386"}
          - {"name": "ha_amd64", "args": "GOARCH=amd64"}
          - {"name": "ha_armhf", "args": "GOARM=6 GOARCH=arm"}
          - {"name": "ha_armv7", "args": "GOARM=7 GOARCH=arm"}
          - {"name": "ha_aarch64", "args": "GOARCH=arm64"}

CGO_ENABLED=0 GOARM=7 GOARCH=arm go build -ldflags="-s -w" -o weather
```
