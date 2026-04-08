#!/usr/bin/with-contenv bashio

echo "Starting weather service..."

export HA_LANGUAGE=${HA_LANGUAGE:-$(bashio::config 'language')}
export HA_UNITS=${HA_UNITS:-$(bashio::config 'uof')}
# export MQTT_HOSTNAME=${MQTT_HOSTNAME:-$(bashio::config 'hostname')}
export MQTT_HOSTNAME=${MQTT_HOSTNAME:-$(bashio::services mqtt "host")}
export MQTT_PASSWORD=${MQTT_PASSWORD:-$(bashio::config 'password')}
# export MQTT_PORT=${MQTT_PORT:-$(bashio::config 'port')}
export MQTT_PORT=${MQTT_PORT:-$(bashio::services mqtt "port")}
export MQTT_USERNAME=${MQTT_USERNAME:-$(bashio::config 'username')}

echo "Loaded hostname: '$MQTT_HOSTNAME'"
echo "Loaded language: '$HA_LANGUAGE'"
echo "Loaded port:     '$MQTT_PORT'"
echo "Loaded units:    '$HA_UNITS'"
echo "Loaded username: '$MQTT_USERNAME'"

exec /usr/bin/weather
