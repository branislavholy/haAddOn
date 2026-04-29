# Weather Station MQTT Gateway

A Go-based MQTT gateway that integrates weather station data with Home Assistant via MQTT protocol.

## Table of Contents

- [Overview](#overview)
- [Features](#features)
- [Security](#security)
- [Configuration](#configuration)
- [Installation](#installation)
- [Usage](#usage)
- [MQTT Topics](#mqtt-topics)
- [Environment Variables](#environment-variables)
- [API Endpoints](#api-endpoints)
- [Logging](#logging)
- [Troubleshooting](#troubleshooting)

---

## Overview

This application acts as a bridge between weather station hardware (via HTTP POST) and Home Assistant (via MQTT). It:

1. Accepts weather data via HTTP POST from weather stations
2. Converts imperial units (configurable)
3. Publishes sensor data to MQTT for Home Assistant with auto-discovery
4. Supports multiple languages for sensor names

---

## Features

| Feature              | Description                                      |
| -------------------- | ------------------------------------------------ |
| **MQTT Integration** | Publishes to Home Assistant via MQTT             |
| **Auto-Discovery**   | Automatic sensor registration via MQTT discovery |
| **Unit Conversion**  | Converts °F→°C, mph→m/s, inHg→hPa, etc.          |
| **Multi-language**   | Supports EN, SK, CZ localization                 |
| **Rate Limiting**    | Prevents DoS attacks (1 req/sec)                 |
| **Input Validation** | Sanitizes all user input                         |
| **Security Headers** | X-Content-Type-Options, X-Frame-Options, etc.    |

---


## Configuration

### Configuration Structure (config.yaml)

```yaml
mqtt:
  host: "homeassistant"  # MQTT broker hostname
  port: 1883             # MQTT broker port
  username: ""           # MQTT username
  password: ""           # MQTT password
  ssl: false             # Use SSL/TLS
  skipSslVerify: false   # Use SSL certificate validation

# Unit preferences
unit_temperature: "°C"    # °C or °F
unit_precipitation: "mm"  # mm or in
unit_pressure: "hPa"      # hPa, inHg, mbar, mmHg
unit_speed: "m/s"         # m/s, mph, km/h, kn

# Language for sensor names
language: "en"           # en, sk, cz
```

---

## Installation

### Prerequisites

- Go 1.21+
- MQTT broker (Mosquitto, Home Assistant MQTT, etc.)

### Manual Go run

```bash
cd ~/workspace/haAddOn/weather/data
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go run main.go
```

### Manual build

```bash
cd ~/workspace/haAddOn/weather/data
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build
```

### Basic Usage

```bash
./weather
```

### With Environment Variables

```bash
export MQTT_USERNAME=$MQTT_USERNAME
export MQTT_PASSWORD=$MQTT_PASSWORD

export LOG_LEVEL='DEBUG'
export HA_LANGUAGE='en'

export MQTT_SSL='false'
export MQTT_SKIP_SSL_VERIFY='true'
export MQTT_HOSTNAME='192.168.1.100'
export MQTT_PORT='1883'
export HTTP_PORT='8080'
export UNIT_TEMPERATURE='°C'
export UNIT_PRECIPITATION='mm'
export UNIT_PRESSURE='hPa'

./weather
```

## Building Local Docker Image

<!-- x-release-please-start-version -->
```bash
cd ~/workspace/haAddOn/weather
podman build -f debug/Dockerfile --format docker --pull --rm --build-arg BUILD_FROM="ghcr.io/home-assistant/aarch64-base:latest" -t weather:latest -t 'weather:1.9.0' .

```
<!-- x-release-please-end -->

## Building GitHub Docker Image
<!-- x-release-please-start-version -->
```bash
cd ~/workspace/haAddOn/weather
podman build --format docker --pull --rm --build-arg BUILD_FROM="ghcr.io/home-assistant/aarch64-base:latest" -t weather:latest -t 'weather:1.9.0' .

```
<!-- x-release-please-end -->

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
  -e LOG_LEVEL=DEBUG \
  -e HA_LANGUAGE=en \
  -e MQTT_USERNAME=$MQTT_USERNAME \
  -e MQTT_PASSWORD=$MQTT_PASSWORD \
  -e MQTT_SSL=false \
  -e MQTT_SKIP_SSL_VERIFY=true \
  -e MQTT_HOSTNAME=192.168.1.100 \
  -e MQTT_PORT=1883 \
  -e HTTP_PORT=8000 \
  -e UNIT_TEMPERATURE='°C' \
  -e UNIT_PRECIPITATION=mm \
  -e UNIT_PRESSURE=hPa \
  -e UNIT_SPEED='m/s' \
  weather:latest
```

### Remove image
<!-- x-release-please-start-version -->
```bash
podman rmi weather:1.9.0 weather:latest
// remove all images
podman images --filter reference='weather:*' -q | xargs podman rmi
```
<!-- x-release-please-end -->

---

## Usage

### Weather Station Configuration

Configure your weather station to send data to own server.

Connect to the PWS Access Point, then go to http://192.168.1.1. On the Configuration page, navigate to Settings > Setup to enter the URL, Station ID, and Station Key.
More info in official meteo docs.

---

## MQTT Topics

### Discovery Topic

```
homeassistant/sensor/weather/<sensor_id>/config
```

### State Topic

```
homeassistant/sensor/weather/state
```
---

## Environment Variables

| Variable               | Required | Default         | Description                |
| ---------------------- | -------- | --------------- | -------------------------- |
| `MQTT_HOSTNAME`        | No       | `homeassistant` | MQTT broker host           |
| `MQTT_PORT`            | No       | `1883`          | MQTT broker port           |
| `MQTT_USERNAME`        | **Yes**  | -               | MQTT username              |
| `MQTT_PASSWORD`        | **Yes**  | -               | MQTT password              |
| `MQTT_SSL`             | No       | `false`         | Enable SSL/TLS             |
| `MQTT_SKIP_SSL_VERIFY` | No       | `false`         | Enable SSL/TLS             |
| `LOG_LEVEL`            | No       | `INFO`          | DEBUG, INFO, WARN, ERROR   |
| `HA_LANGUAGE`          | No       | `en`            | Sensor language (en/sk/cz) |
| `UNIT_TEMPERATURE`     | No       | `°C`            | Temperature unit           |
| `UNIT_PRECIPITATION`   | No       | `mm`            | Precipitation unit         |
| `UNIT_PRESSURE`        | No       | `hPa`           | Pressure unit              |
| `UNIT_SPEED`           | No       | `m/s`           | Speed unit                 |

---

## API Endpoints

### POST /weatherstation/updateweatherstation.php

Receives weather station data via form POST.

**Request:**

```bash
curl -X POST 'http://localhost:8080/weatherstation/updateweatherstation.php?ID=garni2055&PASSWORD=garni2055&action=updateraww&realtime=1&rtfreq=5&dateutc=now&baromin=29.32&tempf=57.9&dewptf=35.7&humidity=44&windspeedmph=0.0&windgustmph=0.0&winddir=114&rainin=0.0&dailyrainin=0.0&solarradiation=0.0&UV=0.0&indoortempf=67.8&indoorhumidity=60'
```

```http
POST /weatherstation/updateweatherstation.php HTTP/1.1
Content-Type: application/x-www-form-urlencoded

ID=garni2055&PASSWORD=garni2055&action=updateraww&realtime=1&rtfreq=5&dateutc=now&baromin=29.32&tempf=57.9&dewptf=35.7&humidity=44&windspeedmph=0.0&windgustmph=0.0&winddir=114&rainin=0.0&dailyrainin=0.0&solarradiation=0.0&UV=0.0&indoortempf=67.8&indoorhumidity=60
```

**Response:**

```
success
```

### GET /

Health check endpoint (with security headers).

**Response:**

```http
HTTP/1.1 200 OK
X-Content-Type-Options: nosniff
X-Frame-Options: DENY
X-XSS-Protection: 1; mode=block
Server: GoWeatherStation/1.8.12
```

---

## Logging

### Log Levels

| Level | Priority | Color  |
| ----- | -------- | ------ |
| DEBUG | 1        | Cyan   |
| INFO  | 2        | Reset  |
| WARN  | 3        | Yellow |
| ERROR | 4        | Red    |

### Set Log Level

```bash
export LOG_LEVEL=DEBUG
```

### Log Output Format

```
[2026-04-29 10:30:45] INFO: Connected to MQTT Broker
[2026-04-29 10:30:45] DEBUG: Converted json payload: '{"tempf":22.5,"humidity":65}'
```
