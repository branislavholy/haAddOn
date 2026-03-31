#!/usr/bin/with-contenv bashio

echo "Starting weather service..."

export HA_LANGUAGE=${HA_LANGUAGE:-$(bashio::config 'language')}
export HA_UNITS=${HA_UNITS:-$(bashio::config 'uof')}
export MQTT_USERNAME=${MQTT_USERNAME:-$(bashio::config 'username')}
export MQTT_PASSWORD=${MQTT_PASSWORD:-$(bashio::config 'password')}

echo "Language: '$HA_LANGUAGE'"
echo "Units: '$HA_UNITS'"
echo "Username: '$MQTT_USERNAME'"

exec /usr/bin/weather
