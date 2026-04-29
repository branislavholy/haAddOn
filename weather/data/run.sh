#!/usr/bin/with-contenv bashio
echo "Starting weather service..."

LOG_LEVEL=${LOG_LEVEL:-$(bashio::config 'log_level')}

export HA_LANGUAGE=${HA_LANGUAGE:-$(bashio::config 'language')}
export MQTT_SSL=${MQTT_SSL:-$(bashio::config 'ssl')}
export MQTT_SKIP_SSL_VERIFY=${MQTT_SKIP_SSL_VERIFY:-$(bashio::config 'skipSslVerify')}
export MQTT_HOSTNAME=${MQTT_HOSTNAME:-$(bashio::info.hostname)}
export MQTT_PASSWORD=${MQTT_PASSWORD:-$(bashio::config 'password')}
export MQTT_PORT=${MQTT_PORT:-1883}
export MQTT_USERNAME=${MQTT_USERNAME:-$(bashio::config 'username')}

export UNIT_TEMPERATURE=${UNIT_TEMPERATURE:-$(bashio::config 'temperature')}
export UNIT_PRECIPITATION=${UNIT_PRECIPITATION:-$(bashio::config 'precipitation')}
export UNIT_PRESSURE=${UNIT_PRESSURE:-$(bashio::config 'pressure')}
export UNIT_SPEED=${UNIT_SPEED:-$(bashio::config 'speed')}

exec /usr/bin/weather
