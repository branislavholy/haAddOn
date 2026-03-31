#!/usr/bin/with-contenv bashio

echo "Starting weather service..."

export HA_LANGUAGE=${HA_LANGUAGE:-$(bashio::config 'language')}
export HA_UNITS=${HA_UNITS:-$(bashio::config 'uof')}

echo "Language: '$HA_LANGUAGE'"
echo "Units: '$HA_UNITS'"

exec /usr/bin/weather
