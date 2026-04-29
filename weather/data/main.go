package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	u "github.com/bcicen/go-units"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

const (
	// clientID    = "go-mqtt-subscriber"
	topic       = "homeassistant/sensor/weather/state"
	ColorReset  = "\033[0m"
	ColorCyan   = "\033[36m"       // Debug
	ColorGreen  = "\033[32m"       // Info
	ColorYellow = "\033[33m"       // Warning
	ColorRed    = "\033[38;5;210m" // Error
)

// Global map for safely tracking registered topics
var registeredTopics sync.Map

var currentLogLevelPriority int

// define default variables
var (
	version = "1.9.2" // x-release-please-version
	// Define by GoReleaser
	date   = "unknown"
	commit = "none"
	binary = "none"

	githubUrl = "https://github.com/branislavholy"

	// Do not change this variable.
	// It is define device in a HomeAssistant
	// echo -n "weatherG" | hexdump -ve '1/1 "%02x"' | sed 's/^/0x/'
	hexaDumpName = "0x7765617468657247"

	Id           = "garni2055"
	topicConfig  = "homeassistant/sensor/weather/%s/config"
	UniqIdPrefix = "sensor_"
	undefined    = "undefined"
)

// HomeAssistant Device structure
type HomeAssistantDevice struct {
	Identifiers      []string `json:"identifiers"`
	HwVersion        string   `json:"hw_version,omitempty"`
	Manufacturer     string   `json:"manufacturer,omitempty"`
	Model            string   `json:"model,omitempty"`
	ModeId           string   `json:"model_id,omitempty"`
	Name             string   `json:"name"`
	SwVersion        string   `json:"sw_version,omitempty"`
	SerialNumber     string   `json:"serial_numner,omitempty"`
	ConfigurationUrl string   `json:"configuration_url,omitempty"`
}

// HomeAssistant Origin structure
type HomeAssistantOrigin struct {
	Name string `json:"name"`
	Sw   string `json:"sw,omitempty"`
	Url  string `json:"url,omitempty"`
	// PayloadOn  string `json:"payload_on" default:"ON"`
}

// HomeAssistant Config structure
type HomeAssistantConfig struct {
	DefaultEntityId            string              `json:"default_entity_id"`
	DeviceClass                string              `json:"device_class,omitempty"`
	EnabledByDefault           bool                `json:"enabled_by_default"`
	StateClass                 string              `json:"state_class,omitempty"`
	StateTopic                 string              `json:"state_topic"`
	UniqueId                   string              `json:"unique_id"`
	UnitOfMeasurement          string              `json:"unit_of_measurement,omitempty"`
	SuggestedUnitOfMeasurement string              `json:"suggested_unit_of_measurement,omitempty"`
	ValueTemplate              string              `json:"value_template"`
	Name                       string              `json:"name"`
	ObjectId                   string              `json:"object_id,omitempty"`
	Device                     HomeAssistantDevice `json:"device"`
	Origin                     HomeAssistantOrigin `json:"origin"`
}

// Set default for Origin
func FillDefaultHomeAssistantOrigin() HomeAssistantOrigin {
	return HomeAssistantOrigin{
		// Uppercase first letter
		Name: cases.Title(language.English, cases.NoLower).String(binary) + " GoLang Loader",
		Sw:   version,
		Url:  githubUrl,
	}
}

// Set default for Device
func FillDefaultHomeAssistantDevice() HomeAssistantDevice {
	return HomeAssistantDevice{
		Identifiers:  []string{"weather_" + hexaDumpName},
		Manufacturer: "",
		Name:         hexaDumpName,
		Model:        "Garni external sensor",
	}
}

// Set default for Config
func FillDefaultHomeAssistantConfig() HomeAssistantConfig {
	return HomeAssistantConfig{
		EnabledByDefault: true,
		// StateClass:       "measurement",
		Device:     FillDefaultHomeAssistantDevice(),
		Origin:     FillDefaultHomeAssistantOrigin(),
		StateTopic: topic,
	}
}

// Updates the device information in the HomeAssistantConfig struct
func addSensor(name, class, unit, key, id, entity, measurement string) HomeAssistantConfig {
	c := FillDefaultHomeAssistantConfig()

	c.Name = name
	c.DeviceClass = class
	c.UnitOfMeasurement = unit
	c.UniqueId = id
	c.ValueTemplate = key
	c.DefaultEntityId = entity
	c.StateClass = measurement
	if class == "wind_speed" {
		c.SuggestedUnitOfMeasurement = unit
	}

	return c
}

// Split string to string and number as string
func GetDeviceModelINFO(input string) (string, string) {
	regexName := regexp.MustCompile(`[a-zA-Z]+`)
	regexVersion := regexp.MustCompile(`[0-9]+`)

	name := regexName.FindString(input)
	version := regexVersion.FindString(input)

	return name, version
}

// Security: Input validation patterns
var (
	inputPattern = regexp.MustCompile(`^[a-zA-Z0-9_\-\.]+$`)
	// numericPattern  = regexp.MustCompile(`^-?\d+\.?\d*$`)
	// stationIdPattern = regexp.MustCompile(`^[a-zA-Z0-9_\-]+$`)
)

// Rate limiting for HTTP requests
var (
	requestCount    int64
	lastRequestTime int64
	rateLimitWindow = time.Second // 1 request per second max
)

// validateInput validates and sanitizes user input
func validateInput(key, value string) (string, string, bool) {
	key = strings.TrimSpace(key)
	value = strings.TrimSpace(value)

	// Check key format
	if len(key) == 0 || len(key) > 100 {
		return "", "", false
	}

	// Validate key format against allowed pattern
	if !inputPattern.MatchString(key) {
		customLog("WARN", "Invalid key format: %q", key)
		return "", "", false
	}

	// Validate value length
	if len(value) > 1000 {
		customLog("WARN", "Value too long for key: %q, value: %q", key, value)
		return "", "", false
	}

	return key, value, true
}

// checkRateLimit implements simple rate limiting
func checkRateLimit() bool {
	// Get the current time in nanoseconds
	now := time.Now().UnixNano()
	// Get the last request time
	lastTime := atomic.LoadInt64(&lastRequestTime)

	// Validate if request is within the rate limit window
	if now-lastTime < rateLimitWindow.Nanoseconds() {
		return false
	}

	// Update the last request time
	if !atomic.CompareAndSwapInt64(&lastRequestTime, lastTime, now) {
		return false
	}

	// Increment the request count
	atomic.AddInt64(&requestCount, 1)
	return true
}

var connectHandler mqtt.OnConnectHandler = func(client mqtt.Client) {
	customLog("INFO", "Connected to MQTT Broker")
}

var connectLostHandler mqtt.ConnectionLostHandler = func(client mqtt.Client, err error) {
	customLog("ERROR", "Connection lost: %v", err)
}

func getLevelPriority(level string) int {
	switch strings.ToUpper(level) {
	case "DEBUG":
		return 1
	case "INFO":
		return 2
	case "WARN":
		return 3
	case "ERROR":
		return 4
	default:
		return 2 // Default to INFO
	}
}

// Simple logging function with color coding and log level filtering
func customLog(level string, format string, v ...any) {
	level = strings.ToUpper(strings.TrimSpace(level))
	if getLevelPriority(level) < currentLogLevelPriority {
		return
	}
	var color string
	timestamp := time.Now().Format("2006-01-02 15:04:05")

	switch level {
	case "DEBUG":
		color = ColorCyan
	case "INFO":
		color = ColorReset
	case "WARN", "WARNING":
		color = ColorYellow
	case "ERROR":
		color = ColorRed
	default:
		color = ColorReset
	}

	// Use Sprintf to combine the format string with the variables
	msg := fmt.Sprintf(format, v...)
	fmt.Printf("%s[%s] %s: %s%s\n", color, timestamp, level, msg, ColorReset)
}

// UnitsConfig provides localized names for sensor types
var UnitsConfig = map[string]map[string]string{
	"tempf": {
		"en": "Temperature Outdoor",
		"sk": "Vonkajšia teplota",
		"cz": "Venkovní Teplota",
	},
	"indoortempf": {
		"en": "Temperature indoor",
		"sk": "Vnútorná teplota",
		"cz": "Vnitřní teplota",
	},
	"dewptf": {
		"en": "Dew Point",
		"sk": "Rosný bod",
		"cz": "Rosný bod",
	},
	"humidity": {
		"en": "Humidity outdoor",
		"sk": "Vonkajšia vlhkosť",
		"cz": "Venkovní vlhkost",
	},
	"indoorhumidity": {
		"en": "Humidity indoor",
		"sk": "Vnútorná vlhkosť",
		"cz": "Vnitřní vlhkost",
	},
	"baromin": {
		"en": "Barometric Pressure",
		"sk": "Barometrický tlak",
		"cz": "Barometrický tlak",
	},
	"windspeedmph": {
		"en": "Wind speed average",
		"sk": "Rýchlosť vetra priemerná",
		"cz": "Rychlost větru průměrná",
	},
	"windgustmph": {
		"en": "Wind speed maximum instantaneous",
		"sk": "Rýchlosť vetra maximálna okamžitá",
		"cz": "Rychlost větru maximální okamžitá",
	},
	"winddir": {
		"en": "Wind Direction",
		"sk": "Smer vetra",
		"cz": "Směr větru",
	},
	"rainin": {
		"en": "Rainfall hourly",
		"sk": "Zrážky intenzita dažďa hodinová",
		"cz": "Srážky intenzita deště hodinová",
	},
	"dailyrainin": {
		"en": "Rainfall daily",
		"sk": "Zrážky denné",
		"cz": "Srážky denní",
	},
	"solarradiation": {
		"en": "Solar Radiation",
		"sk": "Solárne žiarenie",
		"cz": "Sluneční záření",
	},
	"uv": {
		"en": "UV Index",
		"sk": "UV Index",
		"cz": "UV Index",
	},
	"rtfreq": {
		"en": "Real-Time Frequency",
		"sk": "Frekvencia aktualizácie v reálnom čase",
		"cz": "Frekvence aktualizace v reálném čase",
	},
	"dateutc": {
		"en": "Date/Time (UTC)",
		"sk": "Dátum/čas (UTC)",
		"cz": "Datum/čas (UTC)",
	},
	"id": {
		"en": "Station ID",
		"sk": "ID stanice",
		"cz": "ID stanice",
	},
	"password": {
		"en": "Station Key",
		"sk": "Kľúč stanice",
		"cz": "Klíč stanice",
	},
	"action": {
		"en": "Action type",
		"sk": "Typ akcie",
		"cz": "Typ akce",
	},
	"realtime": {
		"en": "Real-Time data status",
		"sk": "Stav dát v reálnom čase",
		"cz": "Stav dat v reálném čase",
	},
	"winddirsite": {
		"en": "Wind Direction site-specific",
		"sk": "Smer vetra zemepisný",
		"cz": "Směr větru zeměpisný",
	},
	"windchillf": {
		"en": "Temperature wind chill",
		"sk": "Vonkajšia teplota ochladzujúci účinok vetrom",
		"cz": "Vonkajší teplota ochlazující účinek větrem",
	},
	"uvcategories": {
		"en": "UV Categories",
		"sk": "UV Kategória",
		"cz": "UV Kategorie",
	},
}

type UnitSource struct {
	DeviceClass string
	DefaultUnit string
	HaInputUnit string
}

// Define default units for each device class.
// HaInputUnit is the name of the field in Config struct, define as HA input unit for user
var entityUnitSource = map[string]UnitSource{
	"duration":       {DeviceClass: "duration", DefaultUnit: "s", HaInputUnit: ""},
	"humidity":       {DeviceClass: "humidity", DefaultUnit: "%", HaInputUnit: ""},
	"illuminance":    {DeviceClass: "illuminance", DefaultUnit: "lx", HaInputUnit: ""},
	"precipitation":  {DeviceClass: "precipitation", DefaultUnit: "in", HaInputUnit: "UnitPrecipitation"},
	"pressure":       {DeviceClass: "pressure", DefaultUnit: "inHg", HaInputUnit: "UnitPressure"},
	"temperature":    {DeviceClass: "temperature", DefaultUnit: "°F", HaInputUnit: "UnitTemperature"},
	"timestamp":      {DeviceClass: "timestamp", DefaultUnit: "", HaInputUnit: ""},
	"wind_direction": {DeviceClass: "wind_direction", DefaultUnit: "°", HaInputUnit: ""},
	"wind_speed":     {DeviceClass: "wind_speed", DefaultUnit: "mph", HaInputUnit: "UnitSpeed"},
	"":               {DeviceClass: "", DefaultUnit: "", HaInputUnit: ""},
}

// SensorConfig defines device class and unit of measurement for a sensor
type SensorConfig struct {
	Status      string
	DeviceClass string
	Unit        string
	Measurement string
}

// Default sensor config fallback
var defaultSensorConfig = SensorConfig{
	Status:      "disabled",
	DeviceClass: "",
	Unit:        "",
	Measurement: "",
}

var unitsSystemConfig = map[string]SensorConfig{
	"tempf":          {Status: "enabled", DeviceClass: "temperature", Measurement: "measurement"},          // Unit: "°C",
	"indoortempf":    {Status: "enabled", DeviceClass: "temperature", Measurement: "measurement"},          // Unit: "°C",
	"dewptf":         {Status: "enabled", DeviceClass: "temperature", Measurement: "measurement"},          // Unit: "°C",
	"humidity":       {Status: "enabled", DeviceClass: "humidity", Measurement: "measurement"},             // Unit: "%",
	"indoorhumidity": {Status: "enabled", DeviceClass: "humidity", Measurement: "measurement"},             // Unit: "%",
	"baromin":        {Status: "enabled", DeviceClass: "pressure", Measurement: "measurement"},             // Unit: "hPa",
	"windspeedmph":   {Status: "enabled", DeviceClass: "wind_speed", Measurement: "measurement"},           // Unit: "m/s", km/h, suggested_unit_of_measurement
	"windgustmph":    {Status: "enabled", DeviceClass: "wind_speed", Measurement: "measurement"},           // Unit: "m/s", km/h,  suggested_unit_of_measurement
	"winddir":        {Status: "enabled", DeviceClass: "wind_direction", Measurement: "measurement_angle"}, // Unit: "°",
	"rainin":         {Status: "enabled", DeviceClass: "precipitation", Measurement: "measurement"},        // Unit: "mm",
	"dailyrainin":    {Status: "enabled", DeviceClass: "precipitation", Measurement: "measurement"},        // Unit: "mm",
	"solarradiation": {Status: "enabled", DeviceClass: "illuminance", Measurement: "measurement"},          // Unit: "lx",
	"uv":             {Status: "enabled", DeviceClass: "", Measurement: "measurement"},                     // Unit: "",
	"windchillf":     {Status: "enabled", DeviceClass: "temperature", Measurement: "measurement"},          // Unit: "°C",
	"winddirsite":    {Status: "enabled", DeviceClass: "", Measurement: ""},                                // Unit: "",
	"uvcategories":   {Status: "enabled", DeviceClass: "", Measurement: ""},
	// Disabled sensors that are not relevant for HomeAssistant
	"rtfreq":   {Status: "disabled", DeviceClass: "duration", Measurement: ""},  // Unit: "s",
	"dateutc":  {Status: "disabled", DeviceClass: "timestamp", Measurement: ""}, // Unit: "",
	"id":       {Status: "disabled", DeviceClass: "", Measurement: ""},          // Unit: "",
	"password": {Status: "disabled", DeviceClass: "", Measurement: ""},          // Unit: "",
	"action":   {Status: "disabled", DeviceClass: "", Measurement: ""},          // Unit: "",
	"realtime": {Status: "disabled", DeviceClass: "", Measurement: ""},          // Unit: "",
}

// Map of common unit symbols to go-units keys for conversion
var unitMap = map[string]string{
	// Temperature
	"°F": "f",
	"°C": "c",

	// Speed
	"mph":  "mph",
	"m/s":  "ms",
	"km/h": "kph",
	"kn":   "knot",

	// Pressure
	"inHg": "inhg",
	"hPa":  "hpa",
	"mbar": "mbar",
	"mmHg": "mmhg",

	// Misc
	"in": "in",
	"°":  "deg",
}

// convertToMetric converts imperial values to metric for specific sensor types
func convertUnitValue(key, value string, defaultUnit string, convertToUnit string) string {

	// If the default unit is the same as the desired unit, return the original value
	if defaultUnit == convertToUnit {
		customLog("DEBUG", "No conversion needed for key %q, default unit %q is the same as desired unit %q", key, defaultUnit, convertToUnit)
		return value
	}

	// Transform string value to float64
	val, err := strconv.ParseFloat(value, 64)
	if err != nil {
		customLog("DEBUG", "Failed to parse value %q for key %q, using original", value, key)
		return value
	}

	// Look up the mapping. If not found, use the original string.
	fromKey, ok := unitMap[defaultUnit]
	if !ok {
		fromKey = defaultUnit
	}

	toKey, ok := unitMap[convertToUnit]
	if !ok {
		toKey = convertToUnit
	}

	// Use go-units library to find the input units
	from, errFrom := u.Find(fromKey)
	to, errTo := u.Find(toKey)

	if fromKey == "mph" {
		var converted float64
		switch toKey {
		case "km/h", "kph":
			converted = val * 1.60934
		case "m/s", "ms":
			converted = val * 0.44704
		case "kn", "knot":
			converted = val * 0.868976
		default:
			converted = val // already m/s
		}
		customLog("DEBUG", "Manual conversion - Input units are 'from:' %q, 'to:' %q for 'key:' %q, 'value:' %q and 'converted': %q", fromKey, toKey, key, value, strconv.FormatFloat(converted, 'f', 2, 64))
		return strconv.FormatFloat(converted, 'f', 2, 64)
	}

	// If there is an error finding the units, log it and return the original value
	if errFrom != nil || errTo != nil {
		customLog("ERROR", "Finding units - from: %q, to: %q", from, to)
		return value
	}

	// Perform the conversion using go-units
	result, err := u.ConvertFloat(val, from, to)
	if err != nil {
		return value
	}

	resultString := strconv.FormatFloat(result.Float(), 'f', 2, 64)
	customLog("DEBUG", "Input units are 'from:' %q, 'to:' %q for 'key:' %q original value 'value:' %q and 'converted:' %q", fromKey, toKey, key, value, resultString)

	// Format the result to 2 decimal places and return as string
	// return fmt.Sprintf("%.2f", result.Float())
	return resultString
}

// transformInput maps sensor keys to appropriate HomeAssistant device classes and units
func transformInput(key, value string, config Config) (status, DeviceClass, unit, localizedName, convertedValue, measurement string) {
	// Convert key to lowercase for case-insensitive lookup
	normalizedKey := strings.ToLower(strings.TrimSpace(key))
	// Get the localized name of the sensor
	localizedName = getLocalizedName(normalizedKey, config.Language)

	// Find the device class for the sensor key and get the user input unit
	// If user has defined a unit for this device class, use it. Otherwise, use the default unit from entityUnitSource
	var sensorUnit string
	if inputUnit := entityUnitSource[unitsSystemConfig[normalizedKey].DeviceClass].HaInputUnit; inputUnit != "" {
		r := reflect.ValueOf(config)
		field := r.FieldByName(inputUnit)
		if field.IsValid() {
			sensorUnit = field.String()
		}
	} else {
		sensorUnit = entityUnitSource[unitsSystemConfig[normalizedKey].DeviceClass].DefaultUnit
	}

	// Convert the value to the desired unit if necessary
	convertedValue = convertUnitValue(normalizedKey, value, entityUnitSource[unitsSystemConfig[normalizedKey].DeviceClass].DefaultUnit, sensorUnit)

	if sensorConfig, exists := unitsSystemConfig[normalizedKey]; exists {
		return sensorConfig.Status, sensorConfig.DeviceClass, sensorUnit, localizedName, convertedValue, sensorConfig.Measurement
	}

	return defaultSensorConfig.Status, defaultSensorConfig.DeviceClass, defaultSensorConfig.Unit, localizedName, value, defaultSensorConfig.Measurement
}

// Returns the localized name for a sensor key
func getLocalizedName(sensorKey, language string) string {
	if localizedNames, exists := UnitsConfig[sensorKey]; exists {
		if name, langExists := localizedNames[strings.ToLower(language)]; langExists {
			return name
		}
		// Default to English if language not found
		return localizedNames["en"]
	}
	// If sensor key not found in UnitsConfig, return the undefined placeholder
	return undefined
}

// Define a struct that matches your config.yaml options
type Config struct {
	MQTT struct {
		Host      string `json:"host"`
		Port      int    `json:"port"`
		Username  string `json:"username"`
		Password  string `json:"password"`
		SSL       bool   `json:"ssl"`
		SslVerify bool   `json:"ssl_verify"`
	} `json:"mqtt"`

	UnitTemperature   string `json:"unit_temperature"`
	UnitPrecipitation string `json:"unit_precipitation"`
	UnitPressure      string `json:"unit_pressure"`
	UnitSpeed         string `json:"unit_speed"`
	Language          string `json:"language"`
	// HttpServerPort    int    `json:"http_server_port"`
}

// loadConfig loads configuration from file with fallback to defaults
// func loadConfig(configPath string) Config {
func loadConfig() (Config, error) {

	defaultConfig := Config{
		MQTT: struct {
			Host      string `json:"host"`
			Port      int    `json:"port"`
			Username  string `json:"username"`
			Password  string `json:"password"`
			SSL       bool   `json:"ssl"`
			SslVerify bool   `json:"ssl_verify"`
		}{
			Host:      "homeassistant",
			Port:      1883,
			Username:  os.Getenv("MQTT_USERNAME"),
			Password:  os.Getenv("MQTT_PASSWORD"),
			SSL:       false,
			SslVerify: false,
		},

		UnitTemperature:   "°C",  // UNIT_TEMPERATURE
		UnitPrecipitation: "mm",  // UNIT_PRECIPITATION
		UnitPressure:      "hPa", // UNIT_PRESSURE
		UnitSpeed:         "m/s", // UNIT_SPEED
		Language:          "en",
		// HttpServerPort:    80,
	}

	// Set the default values from environment variables
	config := defaultConfig

	// Override with environment variables if set
	if envHost := os.Getenv("MQTT_HOSTNAME"); envHost != "" {
		config.MQTT.Host = strings.TrimSpace(envHost)
	}

	if envUser := os.Getenv("MQTT_USERNAME"); envUser != "" {
		config.MQTT.Username = strings.TrimSpace(envUser)
	}

	if envPort := os.Getenv("MQTT_PORT"); envPort != "" {
		if port, err := strconv.Atoi(envPort); err == nil {
			config.MQTT.Port = port
		} else {
			customLog("ERROR", "Failed to parse MQTT_PORT %q, using default", envPort)
		}
	}

	if config.MQTT.Port <= 0 || config.MQTT.Port > 65535 {
		return Config{}, fmt.Errorf("mqtt port must be between 1 and 65535")
	}

	if envPass := os.Getenv("MQTT_PASSWORD"); envPass != "" {
		config.MQTT.Password = strings.TrimSpace(envPass)
	} else {
		customLog("ERROR", "MQTT_PASSWORD is not defined in environment variables")
		os.Exit(1)
	}

	if config.MQTT.Username == "" || config.MQTT.Password == "" {
		return Config{}, fmt.Errorf("mqtt username and password must be provided")
	}

	if config.MQTT.Username == "" || config.MQTT.Password == "" {
		return Config{}, fmt.Errorf("mqtt username and password must be provided")
	}

	if envSSL := os.Getenv("MQTT_SSL"); envSSL != "" {
		val := strings.TrimSpace(envSSL)
		config.MQTT.SSL = strings.EqualFold(val, "true") ||
			strings.EqualFold(val, "yes") ||
			val == "1"
	}

	if envSSLVerify := os.Getenv("MQTT_SKIP_SSL_VERIFY"); envSSLVerify != "" {
		val := strings.TrimSpace(envSSLVerify)
		config.MQTT.SslVerify = strings.EqualFold(val, "true") ||
			strings.EqualFold(val, "yes") ||
			val == "1"
	}

	if config.MQTT.Port <= 0 || config.MQTT.Port > 65535 {
		return Config{}, fmt.Errorf("mqtt port must be between 1 and 65535")
	}

	if haLanguage := os.Getenv("HA_LANGUAGE"); haLanguage != "" {
		config.Language = strings.TrimSpace(haLanguage)
	}

	if uTemperature := os.Getenv("UNIT_TEMPERATURE"); uTemperature != "" {
		config.UnitTemperature = strings.TrimSpace(uTemperature)
	}

	if uPrecipitation := os.Getenv("UNIT_PRECIPITATION"); uPrecipitation != "" {
		config.UnitPrecipitation = strings.TrimSpace(uPrecipitation)
	}

	if uPressure := os.Getenv("UNIT_PRESSURE"); uPressure != "" {
		config.UnitPressure = strings.TrimSpace(uPressure)
	}

	if uSpeed := os.Getenv("UNIT_SPEED"); uSpeed != "" {
		config.UnitSpeed = strings.TrimSpace(uSpeed)
	}

	// if httpServerPort := os.Getenv("HTTP_SERVER_PORT"); httpServerPort != "" {
	// 	if port, err := strconv.Atoi(httpServerPort); err == nil {
	// 		config.HttpServerPort = port
	// 	} else {
	// 		customLog("ERROR", "Failed to parse HTTP_SERVER_PORT %q, using default", httpServerPort)
	// 	}
	// }

	customLog("INFO", "Load variable port: '%d'", config.MQTT.Port)
	customLog("INFO", "Load variable ssl: '%t'", config.MQTT.SSL)
	customLog("INFO", "Load variable ssl verify: '%t'", config.MQTT.SslVerify)
	customLog("INFO", "Load variable username: '%s'", config.MQTT.Username)
	customLog("INFO", "Load variable language: '%s'", config.Language)
	customLog("DEBUG", "Load variable unit temperature: '%s'", config.UnitTemperature)
	customLog("DEBUG", "Load variable unit precipitation: '%s'", config.UnitPrecipitation)
	customLog("DEBUG", "Load variable unit pressure: '%s'", config.UnitPressure)
	customLog("DEBUG", "Load variable unit speed: '%s'", config.UnitSpeed)

	return config, nil
}

// Calculate the wind chill
func calculateWindChill(tempF, windSpeedMph string) string {
	tempfVal, _ := strconv.ParseFloat(tempF, 64)
	windspeedVal, _ := strconv.ParseFloat(windSpeedMph, 64)
	wc := 35.74 +
		(0.6215 * tempfVal) -
		(35.75 * math.Pow(windspeedVal, 0.16)) +
		(0.4275 * tempfVal * math.Pow(windspeedVal, 0.16))
	return strconv.FormatFloat(wc, 'f', 2, 64)
}

// Calculate Wind Direction in degrees and normalize it to 0-360 range
func calculateWinDir(windDir string) string {
	// Normalize wind direction to 0-360 range
	val, err := strconv.ParseFloat(windDir, 64)
	if err != nil {
		customLog("WARN", "Failed to parse wind direction %q, using original", windDir)
		return windDir
	}

	// Normalize negative directions
	if val < 0 {
		val += 360
	}

	// Normalize > 360
	if val > 360 {
		val -= 360
	}

	return strconv.FormatFloat(val, 'f', 0, 64)
}

// Calculate Wind Direction as site-specific
func calculateWindDirSite(windDir string) string {
	// Parse wind direction to float64
	value, err := strconv.ParseFloat(windDir, 64)
	if err != nil {
		// log.Printf("WARN: Failed to parse wind direction %q, using N|S", windDir)
		customLog("WARN", "Failed to parse wind direction %q, using N|S", windDir)
		return "N|S"
	}

	// Determine cardinal direction based on degree ranges
	switch {
	case value <= 11.25 || value > 348.75:
		return "N|S"
	case value > 11.25 && value <= 33.75:
		return "NNE|SSV"
	case value > 33.75 && value <= 56.25:
		return "NE|SV"
	case value > 56.25 && value <= 78.75:
		return "ENE|VSV"
	case value > 78.75 && value <= 101.25:
		return "E|V"
	case value > 101.25 && value <= 123.75:
		return "ESE|VJV"
	case value > 123.75 && value <= 146.25:
		return "SE|JV"
	case value > 146.25 && value <= 168.75:
		return "SSE|JJV"
	case value > 168.75 && value <= 191.25:
		return "S|J"
	case value > 191.25 && value <= 213.75:
		return "SSW|JJZ"
	case value > 213.75 && value <= 236.25:
		return "SW|JZ"
	case value > 236.25 && value <= 258.75:
		return "WSW|ZJZ"
	case value > 258.75 && value <= 281.25:
		return "W|Z"
	case value > 281.25 && value <= 303.75:
		return "WNW|ZSZ"
	case value > 303.75 && value <= 326.25:
		return "NW|SZ"
	case value > 326.25 && value <= 348.75:
		return "NNW|SSZ"
	default:
		return "N|S"
	}
}

// Calculate UV categories based on UV index value
func calculateUvCategories(uvValue string) string {
	uvNormalize, err := strconv.ParseFloat(uvValue, 64)
	if err != nil {
		return "Invalid UV Value"
	}

	switch {
	case uvNormalize >= 0 && uvNormalize <= 2:
		return "Low"
	case uvNormalize > 2 && uvNormalize <= 5:
		return "Moderate"
	case uvNormalize > 5 && uvNormalize <= 7:
		return "High"
	case uvNormalize > 7 && uvNormalize <= 10:
		return "Very High"
	default:
		return "Extreme"
	}
}

// Establishes a connection to the MQTT broker using the provided configuration
func mqttConnect(config Config) mqtt.Client {
	opts := mqtt.NewClientOptions()

	scheme := "tcp"
	if config.MQTT.SSL {
		scheme = "ssl"
		opts.SetTLSConfig(&tls.Config{
			MinVersion:         tls.VersionTLS12,
			InsecureSkipVerify: config.MQTT.SslVerify,
		})
	}

	// Configure broker connection string
	broker := fmt.Sprintf("%s://%s:%d", scheme, config.MQTT.Host, config.MQTT.Port)
	customLog("INFO", "Connecting to MQTT at: '%s' with user: '%s'", broker, config.MQTT.Username)

	// Set credentials and broker URI
	opts.SetUsername(config.MQTT.Username)
	opts.SetPassword(config.MQTT.Password)
	opts.AddBroker(broker)

	// Use MQTT protocol version 4 (MQTT 3.1.1) for compatibility with Home Assistant
	opts.SetProtocolVersion(4)

	// Generate a unique ClientID to avoid connection loops
	uniqueID := fmt.Sprintf("weather-station-%d", time.Now().UnixNano()%100000)
	opts.SetClientID(uniqueID)

	// Configure message handling and logging
	opts.SetDefaultPublishHandler(func(client mqtt.Client, msg mqtt.Message) {
		customLog("INFO", "Topic: '%s' Message: '%s'", msg.Topic(), msg.Payload())
	})

	// Disable strict ordering to allow concurrent message processing in goroutines
	opts.SetOrderMatters(false)

	// Set timeouts and keep-alive for connection stability
	opts.SetKeepAlive(60 * time.Second)
	opts.SetPingTimeout(10 * time.Second)
	opts.SetWriteTimeout(10 * time.Second)

	// Enable automatic reconnection and persistence
	opts.SetAutoReconnect(true)
	opts.SetConnectRetry(true)
	opts.SetCleanSession(false)

	// Assign connection event handlers
	opts.OnConnect = connectHandler
	opts.OnConnectionLost = connectLostHandler

	mqtt.ERROR = log.New(os.Stdout, "[MQTT-ERROR] ", 0)
	mqtt.CRITICAL = log.New(os.Stdout, "[MQTT-CRIT] ", 0)

	// Initialize the client and wait for a successful connection
	client := mqtt.NewClient(opts)
	token := client.Connect()
	if !token.WaitTimeout(5 * time.Second) {
		customLog("ERROR", "MQTT Fatal: Connection timeout")
		os.Exit(1)
	}
	if err := token.Error(); err != nil {
		customLog("ERROR", "MQTT Fatal: %v", err)
		os.Exit(1)
	}
	time.Sleep(500 * time.Millisecond)
	if !client.IsConnected() {
		customLog("ERROR", "MQTT Fatal: Connection lost immediately after connecting")
		os.Exit(1)
	}

	customLog("INFO", "Connecting to MQTT successfully!")

	// Verify the actual protocol version used by the client
	customLog("DEBUG", "Used MQTT protocol version: %d", opts.ProtocolVersion)

	return client
}

func applySecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("Server", "GoWeatherStation")

		// Pokračuj na skutočný handler (napr. handleData)
		next.ServeHTTP(w, r)
	})
}

func main() {

	// Set log level from environment variable with default to INFO
	envLogLevel := os.Getenv("LOG_LEVEL")
	if envLogLevel == "" {
		envLogLevel = "INFO"
	}
	currentLogLevelPriority = getLevelPriority(envLogLevel)

	// Load configuration with fallback to defaults
	config, err := loadConfig()
	if err != nil {
		customLog("ERROR", "Configuration error: %v", err)
		os.Exit(1)
	}

	// Establish MQTT connection
	client := mqttConnect(config)
	if client == nil {
		customLog("ERROR", "MQTT connection failed, aborting")
		os.Exit(1)
	}

	// Ensure the client disconnects gracefully on application exit
	defer client.Disconnect(250)

	customLog("INFO", "Build version: '%s'", version)
	customLog("INFO", "Build commit: '%s'", commit)
	customLog("INFO", "Build date: '%s'", date)

	// if config.HttpServerPort != 80 {
	// 	hexaDumpName = "0x776-GO-TEST-CODE"
	// }
	// customLog("INFO", "System name: %s", hexaDumpName)

	// Set up HTTP server and route for weather station updates
	mux := http.NewServeMux()
	mux.HandleFunc("/weatherstation/updateweatherstation.php", func(w http.ResponseWriter, r *http.Request) {
		handleData(w, r, config, client)
	})

	server := &http.Server{
		// Addr:              ":" + strconv.Itoa(config.HttpServerPort),
		Addr:              ":80",
		Handler:           applySecurityHeaders(mux),
		ReadHeaderTimeout: 5 * time.Second,  // Protects against Slowloris attacks
		ReadTimeout:       15 * time.Second, // Time to read request body
		WriteTimeout:      15 * time.Second, // Time to write response
		IdleTimeout:       60 * time.Second, // Keep-alive duration
	}

	customLog("INFO", "Starting server on: '%s'", server.Addr)
	if err := server.ListenAndServe(); err != nil {
		customLog("ERROR", "Server failed: %v", err)
	}
}

// Registers sensors with Home Assistant using MQTT discovery, with goroutines and concurrency control to prevent MQTT flooding
func registerSensors(client mqtt.Client, sensors []HomeAssistantConfig) {
	// Use a WaitGroup to wait for all goroutines to finish before exiting the function
	var wg sync.WaitGroup
	// Create a buffered channel to limit concurrency and prevent MQTT flooding
	semaphore := make(chan struct{}, 7)

	for _, itemSenzor := range sensors {
		// Increment the WaitGroup counter for each sensor registration
		wg.Add(1)

		// Gorutine to handle each sensor registration concurrently
		go func(localSenzor HomeAssistantConfig) {
			// Decrement the WaitGroup counter when the goroutine finishes
			defer wg.Done()

			// Create topic by removing prefix from UniqueId
			sensorID := strings.TrimPrefix(localSenzor.UniqueId, UniqIdPrefix)
			topic := fmt.Sprintf(topicConfig, sensorID)

			// Fast check if already registered to skip network operations
			if _, alreadyRegistered := registeredTopics.Load(topic); alreadyRegistered {
				return
			}

			// Acquire a slot in the semaphore to proceed with MQTT communication
			semaphore <- struct{}{}
			// Release the slot when the goroutine finishes
			defer func() { <-semaphore }()

			// Verify client connection state before publishing
			if !client.IsConnected() {
				customLog("WARN", "MQTT disconnected. Registration for %q deferred.", sensorID)
				return
			}

			// Prepare the JSON payload for Home Assistant discovery
			payload, err := json.Marshal(localSenzor)
			if err != nil {
				customLog("ERROR", "Failed to marshal JSON for %s: %v", sensorID, err)
				return
			}

			// Publish discovery message with QoS 1 and Retain flag set to true
			customLog("INFO", "Registering new sensor: '%s'", sensorID)
			token := client.Publish(topic, 1, true, payload)

			// Wait for broker acknowledgment and store success in the local map
			if token.Wait() && token.Error() == nil {
				registeredTopics.Store(topic, true)
				customLog("INFO", "Sensor '%s' successfully registered.", sensorID)
				// This delay gives Mosquitto 100ms to breathe between sensors
				// This prevents the "flood" that causes the EOF disconnect.
				time.Sleep(100 * time.Millisecond)
			} else {
				customLog("ERROR", "Registration failed for '%s': %v", sensorID, token.Error())
			}
		}(itemSenzor)
	}

	// Block until all sensor registrations are processed
	wg.Wait()
}

// Handles incoming HTTP requests from the weather station, processes the data, and publishes it to MQTT for Home Assistant integration
func handleData(w http.ResponseWriter, r *http.Request, config Config, client mqtt.Client) {
	customLog("DEBUG", "Start handleData method for request from %s", r.RemoteAddr)
	customLog("WARN", "Start handleData method")
	var sensors []HomeAssistantConfig
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		customLog("ERROR", "HTTP method %q not allowed, only POST is accepted", r.Method)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Add rate limiting
	if !checkRateLimit() {
		customLog("WARN", "Rate limit exceeded, rejecting request")
		customLog("ERROR", "Rate limit exceeded for request from %s", r.RemoteAddr)
		http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
		return
	}

	// Validate Content-Type header to ensure it's a form submission
	ct := r.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "application/x-www-form-urlencoded") {
		customLog("WARN", "Invalid Content-Type: %s", ct)
		http.Error(w, "unsupported media type", http.StatusUnsupportedMediaType)
		return
	}

	// Limit the size of the request body to prevent abuse and ensure stability
	r.Body = http.MaxBytesReader(w, r.Body, 16*1024)

	if err := r.ParseForm(); err != nil {
		customLog("ERROR", "Failed to parse request form: %v", err)
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	// Respond immediately to the station to avoid timeouts, while processing the data concurrently
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte("success")); err != nil {
		customLog("WARN", "Failed to write response to station: %v", err)
	}

	customLog("DEBUG", "Validation finished, processing data for MQTT publication")

	var data = make(map[string]string)

	// Iterate over the form values and generate the data map
	for key, values := range r.Form {
		for _, val := range values {
			// Skip empty entries
			if key == "" || val == "" {
				customLog("WARN", "Skipping empty key or value - key=%q, value=%q", key, val)
				continue
			}

			// Validate input
			key, val, valid := validateInput(key, val)
			if !valid {
				customLog("WARN", "Skipping invalid input - key=%q", key)
				continue
			}

			data[key] = val
		}
	}

	// Fill the default values for HomeAssistant Origin MQTT config
	homeAssistantOrigin := FillDefaultHomeAssistantOrigin()
	customLog("DEBUG", "Origin payload: '%+v'", homeAssistantOrigin)

	// Fill the default values for HomeAssistant Device MQTT config
	homeAssistantDevice := FillDefaultHomeAssistantDevice()

	// Fill the default value
	stationId := Id
	if stationPayloadId, ok := data["ID"]; ok {
		stationId = stationPayloadId
	}
	homeAssistantDevice.ModeId = stationId

	// Fill the model and hardware version based on the station ID
	modelName, modelVersion := GetDeviceModelINFO(stationId)
	homeAssistantDevice.Model = strings.ToUpper(modelName)
	homeAssistantDevice.HwVersion = modelVersion

	customLog("DEBUG", "Device payload: '%+v'", homeAssistantDevice)

	// Add the windchillf sensor
	if tempf, ok_t := data["tempf"]; ok_t {
		if windSpeed, ok_w := data["windspeedmph"]; ok_w {
			data["windchillf"] = calculateWindChill(tempf, windSpeed)
		}
	}

	// Add the winddir and winddirsite sensors
	if windDir, ok := data["winddir"]; ok {
		normWindDir := calculateWinDir(windDir)
		data["winddir"] = normWindDir
		data["winddirsite"] = calculateWindDirSite(normWindDir)
	}

	// Add the UV categories sensor
	if uv, ok := data["UV"]; ok {
		data["uvcategories"] = calculateUvCategories(uv)
	}

	customLog("INFO", "Received original map data: %v", data)

	// Load and parse the original JSON payload for logging purposes
	jsonOriginalData, err := json.Marshal(data)
	if err != nil {
		customLog("ERROR", "Message: %v", err)
		return
	}
	customLog("INFO", "Received original json payload: '%s'", jsonOriginalData)

	// Process and validate each sensor
	for key, value := range data {
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)

		// Transform the input key and value to determine if the sensor should be registered.
		status, deviceCl, unitOfMesure, localizedName, convertedValue, measurement := transformInput(key, value, config)

		// Update the data map with the converted value for MQTT payload
		data[key] = convertedValue

		// Only add sensors that are enabled in the configuration
		if status == "enabled" {
			sensors = append(sensors, addSensor(
				localizedName,                           // Name
				deviceCl,                                // DeviceClass
				unitOfMesure,                            // UnitOfMeasurement
				fmt.Sprintf("{{ value_json.%s }}", key), // ValueTemplate
				fmt.Sprintf("%s%s", UniqIdPrefix, key),  // UniqueId
				fmt.Sprintf("sensor.%s_%s", hexaDumpName, key), // DefaultEntityId
				measurement, // StateClass
			))
		}
	}

	// Convert the final data and logging the JSON payload that will be published to MQTT
	jsonData, err := json.Marshal(data)
	if err != nil {
		customLog("ERROR", "Message: %v", err)
		return
	}
	customLog("INFO", "Converted json payload: '%s'", jsonData)

	// Goroutine to register sensors concurrently without blocking the main HTTP handler
	go registerSensors(client, sensors)

	// Publish the sensor data to MQTT with QoS 1 and Retain flag set to true
	client.Publish(topic, 1, true, jsonData).Wait()
}
