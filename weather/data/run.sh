#!/usr/bin/with-contenv bashio

echo "Starting weather service..."

export HA_LANGUAGE=${HA_LANGUAGE:-$(bashio::config 'language')}
export HA_UNITS=${HA_UNITS:-$(bashio::config 'uof')}
export MQTT_HOSTNAME=${MQTT_HOSTNAME:-$(bashio::info.hostname)}
export MQTT_PASSWORD=${MQTT_PASSWORD:-$(bashio::config 'password')}
export MQTT_PORT=${MQTT_PORT:-1883}
export MQTT_USERNAME=${MQTT_USERNAME:-$(bashio::config 'username')}

LOG_LEVEL=${LOG_LEVEL:-$(bashio::config 'log_level')}
export LOG_LEVEL=${LOG_LEVEL^^}

bashio::log.info "Starting weather service with log level: '$LOG_LEVEL'"

if [ "$LOG_LEVEL" = "DEBUG" ]; then
  export __BASHIO_LOG_LEVEL=7
  bashio::log.warning "Loaded hostname: '$MQTT_HOSTNAME'"
  bashio::log.debug "Loaded hostname: '$MQTT_HOSTNAME'"
  bashio::log.debug "Loaded language: '$HA_LANGUAGE'"
  bashio::log.debug "Loaded port:     '$MQTT_PORT'"
  bashio::log.debug "Loaded units:    '$HA_UNITS'"
  bashio::log.debug "Loaded username: '$MQTT_USERNAME'"
fi

exec /usr/bin/weather
