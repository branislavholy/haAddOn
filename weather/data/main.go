package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

const (
	clientID   = "go-mqtt-subscriber"
	topic      = "homeassistant/sensor/weather/state"
	ColorReset = "\033[0m"
	ColorCyan  = "\033[36m" // DEBUG
	ColorGreen = "\033[32m" // INFO
)

var mqttMsgChan = make(chan mqtt.Message)

var messagePubHandler mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
	mqttMsgChan <- msg
}

var (
	version = "1.4.18" // x-release-please-version
	// Define by GoReleaser
	commit = "none"
	date   = "unknown"
	binary = "none"

	githubUrl = "https://github.com/branislavholy/haAddOn/ha-addon"

	// Do not change this variable.
	// It is define device in a HomeAssistant
	// echo -n "weatherG" | hexdump -ve '1/1 "%02x"' | sed 's/^/0x/'
	hexaDumpName = "0x7765617468657247"

	topicConfig  = "homeassistant/sensor/weather/%s/config"
	Id           = "garni2055"
	undefined    = "undefined"
	UniqIdPrefix = "sensor_"

	// mock start
	// tempf        = 33.0
	// windspeedmph = 2.6
	// mock end
)

// Define a struct that matches your config.yaml options
type Config struct {
	MQTT struct {
		Host     string `json:"host"`
		Port     int    `json:"port"`
		Username string `json:"username"`
		Password string `json:"password"`
		SSL      bool   `json:"ssl"`
	} `json:"mqtt"`

	UnitOfMeasurement string `json:"uof"`
	Language          string `json:"language"`
}

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
	DefaultEntityId   string              `json:"default_entity_id"`
	DeviceClass       string              `json:"device_class,omitempty"`
	EnabledByDefault  bool                `json:"enabled_by_default"`
	StateClass        string              `json:"state_class,omitempty"`
	StateTopic        string              `json:"state_topic"`
	UniqueId          string              `json:"unique_id"`
	UnitOfMeasurement string              `json:"unit_of_measurement,omitempty"`
	ValueTemplate     string              `json:"value_template"`
	Name              string              `json:"name"`
	ObjectId          string              `json:"object_id,omitempty"`
	Device            HomeAssistantDevice `json:"device"`
	Origin            HomeAssistantOrigin `json:"origin"`
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

func addSensor(name, class, unit, key, id, entity, measurement string) HomeAssistantConfig {
	c := FillDefaultHomeAssistantConfig()

	c.Name = name
	c.DeviceClass = class
	c.UnitOfMeasurement = unit
	c.UniqueId = id
	c.ValueTemplate = key
	c.DefaultEntityId = entity
	c.StateClass = measurement

	return c
}

// Split string to string and number as string
func GetDeviceModelInfo(input string) (string, string) {
	regexName := regexp.MustCompile(`[a-zA-Z]+`)
	regexVersion := regexp.MustCompile(`[0-9]+`)

	name := regexName.FindString(input)
	version := regexVersion.FindString(input)

	return name, version
}

var connectHandler mqtt.OnConnectHandler = func(client mqtt.Client) {
	fmt.Println("Connected to MQTT Broker")
}

var connectLostHandler mqtt.ConnectionLostHandler = func(client mqtt.Client, err error) {
	fmt.Printf("Connection lost: %v", err)
}

func myLog(level string, msg string, payload any) {
	// timestamp := time.Now().Format("2006-01-02 15:04:05")
	// fmt.Printf("[%s] %s: \t%s '%s'\n", timestamp, level, msg, payload)
	var color string
	// timestamp := time.Now().Format("15:04:05")
	timestamp := time.Now().Format("2006-01-02 15:04:05")

	switch level {
	case "DEBUG":
		color = ColorCyan
	case "INFO":
		color = ColorGreen
	default:
		color = ColorReset
	}

	fmt.Printf("%s[%s] %s: %s '%s'%s\n", color, timestamp, level, msg, payload, ColorReset)
}

// SensorConfig defines device class and unit of measurement for a sensor
type SensorConfig struct {
	Status      string
	DeviceClass string
	Unit        string
	Measurement string
}

// UnitsConfig provides localized names for sensor types
var UnitsConfig = map[string]map[string]string{
	"tempf": {
		"en": "Outdoor Temperature",
		"sk": "Teplota vonkajšia",
	},
	"indoortempf": {
		"en": "Indoor Temperature",
		"sk": "Teplota vnútorná",
	},
	"dewptf": {
		"en": "Dew Point",
		"sk": "Rosný bod",
	},
	"humidity": {
		"en": "Outdoor Humidity",
		"sk": "Vlhkosť vonkajšia",
	},
	"indoorhumidity": {
		"en": "Indoor Humidity",
		"sk": "Vlhkosť vnútorná",
	},
	"baromin": {
		"en": "Barometric Pressure",
		"sk": "Barometrický tlak",
	},
	"windspeedmph": {
		"en": "Average wind speed",
		"sk": "Priemerná rýchlosť vetra",
	},
	"windgustmph": {
		"en": "Maximum instantaneous wind speed",
		"sk": "Maximálna okamžitá rýchlosť vetra",
	},
	"winddir": {
		"en": "Wind Direction",
		"sk": "Smer vetra",
	},
	"rainin": {
		"en": "Hourly Rainfall",
		"sk": "Intenzita dažďa hodinová",
	},
	"dailyrainin": {
		"en": "Daily Rainfall",
		"sk": "Denné zrážky",
	},
	"solarradiation": {
		"en": "Solar Radiation",
		"sk": "Solárne žiarenie",
	},
	"uv": {
		"en": "UV Index",
		"sk": "UV Index",
	},
	"rtfreq": {
		"en": "Real-Time Frequency",
		"sk": "Frekvencia aktualizácie v reálnom čase",
	},
	"dateutc": {
		"en": "Date/Time (UTC)",
		"sk": "Dátum/čas (UTC)",
	},
	"id": {
		"en": "Station ID",
		"sk": "ID stanice",
	},
	"password": {
		"en": "Station Key",
		"sk": "Kľúč stanice",
	},
	"action": {
		"en": "Action Type",
		"sk": "Typ akcie",
	},
	"realtime": {
		"en": "Real-Time Data Status",
		"sk": "Stav dát v reálnom čase",
	},
	"winddirsite": {
		"en": "Site-Specific Wind Direction",
		"sk": "Smer vetra",
	},
	"windchillf": {
		"en": "Wind Chill temperature",
		"sk": "Ochladzujúci účinok vetra",
	},
}

// key=rtfreq, value=5
// key=indoortempf, value=68
// key=windgustmph, value=2
// key=baromin, value=30
// key=temperature, value=300
// key=tempf, value=53
// key=humidity, value=96
// key=winddir, value=24
// key=indoorhumidity, value=5000
// key=UV, value=0
// key=realtime, value=1
// key=dailyrainin, value=0
// key=windspeedmph, value=2
// key=rainin, value=0
// key=dewptf, value=52
// key=solarradiation, value=0

// {"ID":"garni2055","PASSWORD":"garni2055","UV":"0.0","action":"updateraww","dateutc":"2026-03-23T21:01:40Z","dewPoint":1.78,"humidity":"55","humidityIndoor":"60","pressure":1012.08,"rainDailymm":0,"rainmm":0,"realtime":"1","rtfreq":"5","solarradiation":"0.0","temperature":10.56,"temperatureIndoor":19.78,"windChill":2.08,"windDirSite":"SW|JZ","windGustms":0.89,"windSpeedms":0.89,"winddir":"221"}

// unitsImperial defines all sensor types with their device classes and units (°F, mph, inHg)
var unitsImperial = map[string]SensorConfig{
	"tempf":          {Status: "enabled", DeviceClass: "temperature", Unit: "°F", Measurement: "measurement"},
	"indoortempf":    {Status: "enabled", DeviceClass: "temperature", Unit: "°F", Measurement: "measurement"},
	"dewptf":         {Status: "enabled", DeviceClass: "temperature", Unit: "°F", Measurement: "measurement"},
	"humidity":       {Status: "enabled", DeviceClass: "humidity", Unit: "%", Measurement: "measurement"},
	"indoorhumidity": {Status: "enabled", DeviceClass: "humidity", Unit: "%", Measurement: "measurement"},
	"baromin":        {Status: "enabled", DeviceClass: "pressure", Unit: "inHg", Measurement: "measurement"},
	"windspeedmph":   {Status: "enabled", DeviceClass: "wind_speed", Unit: "mph", Measurement: "measurement"},
	"windgustmph":    {Status: "enabled", DeviceClass: "wind_speed", Unit: "mph", Measurement: "measurement"},
	"winddir":        {Status: "enabled", DeviceClass: "wind_direction", Unit: "°", Measurement: "measurement_angle"},
	"rainin":         {Status: "enabled", DeviceClass: "precipitation", Unit: "in", Measurement: "measurement"},
	"dailyrainin":    {Status: "enabled", DeviceClass: "precipitation_intensity", Unit: "in", Measurement: "measurement"},
	"solarradiation": {Status: "enabled", DeviceClass: "illuminance", Unit: "lx", Measurement: "measurement"},
	"uv":             {Status: "enabled", DeviceClass: "", Unit: "", Measurement: "measurement"},
	"windchillf":     {Status: "enabled", DeviceClass: "temperature", Unit: "°F", Measurement: "measurement"},
	"winddirsite":    {Status: "enabled", DeviceClass: "", Unit: "", Measurement: ""},
	// Disabled sensors that are not relevant for HomeAssistant
	"rtfreq":   {Status: "disabled", DeviceClass: "frequency", Unit: "s", Measurement: ""},
	"dateutc":  {Status: "disabled", DeviceClass: "timestamp", Unit: "", Measurement: ""},
	"id":       {Status: "disabled", DeviceClass: "none", Unit: "", Measurement: ""},
	"password": {Status: "disabled", DeviceClass: "none", Unit: "", Measurement: ""},
	"action":   {Status: "disabled", DeviceClass: "none", Unit: "", Measurement: ""},
	"realtime": {Status: "disabled", DeviceClass: "binary_sensor", Unit: "", Measurement: ""},
}

// TODO: The UV Index Scale
// 0 – 2: Low
// 3 – 5: Moderate
// 6 – 7: High
// 8 – 10: Very High
// 11+: Extreme

// unitsMetric defines all sensor types with their device classes and units (°C, km/h, hPa)
var unitsMetric = map[string]SensorConfig{
	"tempf":          {Status: "enabled", DeviceClass: "temperature", Unit: "°C", Measurement: "measurement"},
	"indoortempf":    {Status: "enabled", DeviceClass: "temperature", Unit: "°C", Measurement: "measurement"},
	"dewptf":         {Status: "enabled", DeviceClass: "temperature", Unit: "°C", Measurement: "measurement"},
	"humidity":       {Status: "enabled", DeviceClass: "humidity", Unit: "%", Measurement: "measurement"},
	"indoorhumidity": {Status: "enabled", DeviceClass: "humidity", Unit: "%", Measurement: "measurement"},
	"baromin":        {Status: "enabled", DeviceClass: "pressure", Unit: "hPa", Measurement: "measurement"},
	"windspeedmph":   {Status: "enabled", DeviceClass: "wind_speed", Unit: "km/h", Measurement: "measurement"},
	"windgustmph":    {Status: "enabled", DeviceClass: "wind_speed", Unit: "km/h", Measurement: "measurement"}, // suggested_unit_of_measurement
	"winddir":        {Status: "enabled", DeviceClass: "wind_direction", Unit: "°", Measurement: "measurement_angle"},
	"rainin":         {Status: "enabled", DeviceClass: "precipitation", Unit: "mm", Measurement: "measurement"},
	"dailyrainin":    {Status: "enabled", DeviceClass: "precipitation", Unit: "mm", Measurement: "measurement"},
	"solarradiation": {Status: "enabled", DeviceClass: "illuminance", Unit: "lx", Measurement: "measurement"},
	"uv":             {Status: "enabled", DeviceClass: "", Unit: "", Measurement: "measurement"},
	"windchillf":     {Status: "enabled", DeviceClass: "temperature", Unit: "°C", Measurement: "measurement"},
	"winddirsite":    {Status: "enabled", DeviceClass: "", Unit: "", Measurement: ""},
	// Disabled sensors that are not relevant for HomeAssistant
	"rtfreq":   {Status: "disabled", DeviceClass: "duration", Unit: "s", Measurement: ""},
	"dateutc":  {Status: "disabled", DeviceClass: "timestamp", Unit: "", Measurement: ""},
	"id":       {Status: "disabled", DeviceClass: "", Unit: "", Measurement: ""},
	"password": {Status: "disabled", DeviceClass: "", Unit: "", Measurement: ""},
	"action":   {Status: "disabled", DeviceClass: "", Unit: "", Measurement: ""},
	"realtime": {Status: "disabled", DeviceClass: "", Unit: "", Measurement: ""},
}

// // unitsSystems maps unit system names to their sensor configuration maps
// var unitsSystems = map[string]map[string]SensorConfig{
// 	"imperial": unitsImperial,
// 	"metric":   unitsMetric,
// }

// Default sensor config fallback
var defaultSensorConfig = SensorConfig{
	Status:      "disabled",
	DeviceClass: "",
	Unit:        "",
	Measurement: "",
}

// convertToMetric converts imperial values to metric for specific sensor types
func convertToMetric(key, value string) string {
	// Parse value to float64
	val, err := strconv.ParseFloat(value, 64)
	if err != nil {
		// log.Printf("WARN: Failed to parse value %q for key %q, using original", value, key)
		return value
	}
	// log.Printf("INFO: Parsed value %q for key %q, using original", value, key)

	var converted float64
	switch key {
	case "tempf", "indoortempf", "dewptf", "windchillf":
		// Fahrenheit to Celsius
		converted = (val - 32) * 5 / 9
	case "windspeedmph", "windgustmph":
		// mph to km/hs
		converted = val * 1.609344
		// // mph to m/s
		// converted = val * 0.44704
	case "baromin":
		// inHg to hPa
		converted = val * 33.8639
	case "rainin", "dailyrainin":
		// inches to mm
		converted = val * 25.4
	default:
		// No conversion needed
		return value
	}
	// log.Printf("INFO: Value %q converted to %q for key %q", value, strconv.FormatFloat(converted, 'f', 2, 64), key)
	// Format back to string with 2 decimal places
	return strconv.FormatFloat(converted, 'f', 2, 64)
}

// transformInput maps sensor keys to appropriate HomeAssistant device classes and units
func transformInput(key, value string, config Config) (status, deviceClass, unit, localizedName, convertedValue, measurement string) {
	// Convert key to lowercase for case-insensitive lookup
	normalizedKey := strings.ToLower(strings.TrimSpace(key))
	normalizedUnitsType := strings.ToLower(strings.TrimSpace(config.UnitOfMeasurement))
	localizedName = getLocalizedName(normalizedKey, config.Language)

	// Select the appropriate unit system map
	var selectedUnits map[string]SensorConfig

	switch normalizedUnitsType {
	case "imperial":
		selectedUnits = unitsImperial
		convertedValue = value
		// log.Printf("DEBUG: Using %q unit system for sensor - key=%q, value=%q", normalizedUnitsType, key, convertedValue)
	case "metric":
		selectedUnits = unitsMetric
		// Convert value from imperial to metric
		convertedValue = convertToMetric(normalizedKey, value)
		// log.Printf("DEBUG: Using %q unit system for sensor - key=%q, original=%q, converted=%q", normalizedUnitsType, key, value, convertedValue)
	default:
		log.Printf("WARN: Unknown unit system %q, defaulting to imperial", normalizedUnitsType)
		// selectedUnits = unitsImperial
		return defaultSensorConfig.Status, defaultSensorConfig.DeviceClass, defaultSensorConfig.Unit, localizedName, value, defaultSensorConfig.Measurement
	}

	// Look up sensor in the selected unit system map
	if sensorConfig, exists := selectedUnits[normalizedKey]; exists {
		// Get localized name from unitsName map
		// log.Printf("DEBUG: Found sensor mapping - key=%q, value=%q, system=%q -> class=%q, unit=%q, name=%q",
		// 	key, convertedValue, normalizedUnitsType, sensorConfig.DeviceClass, sensorConfig.Unit, localizedName)
		return sensorConfig.Status, sensorConfig.DeviceClass, sensorConfig.Unit, localizedName, convertedValue, sensorConfig.Measurement
	}

	// Log warning for unknown sensor types
	log.Printf("WARN: Unknown sensor type - key=%q (in %q system), using default mapping", key, normalizedUnitsType)
	return defaultSensorConfig.Status, defaultSensorConfig.DeviceClass, defaultSensorConfig.Unit, localizedName, value, defaultSensorConfig.Measurement
}

// getLocalizedName returns the localized name for a sensor key
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

// loadConfig loads configuration from file with fallback to defaults
// func loadConfig(configPath string) Config {
func loadConfig() Config {
	defaultConfig := Config{
		MQTT: struct {
			Host     string `json:"host"`
			Port     int    `json:"port"`
			Username string `json:"username"`
			Password string `json:"password"`
			SSL      bool   `json:"ssl"`
		}{
			Host:     "192.168.10.20",
			Port:     1883,
			Username: os.Getenv("MQTT_USERNAME"),
			Password: os.Getenv("MQTT_PASSWORD"),
			SSL:      false,
		},

		UnitOfMeasurement: "metric",
		// UnitOfMeasurement: "imperial",
		Language: "en",
	}

	// // Try to read config file
	// file, err := os.ReadFile(configPath)
	// if err != nil {
	// 	if os.IsNotExist(err) {
	// 		log.Printf("WARN: Config file not found at %s, using defaults", configPath)
	// 	} else {
	// 		log.Printf("ERROR: Failed to read config file: %v, using defaults", err)
	// 	}
	// 	return defaultConfig
	// }

	// Parse JSON config
	var config Config
	// if err := json.Unmarshal(file, &config); err != nil {
	// 	log.Printf("ERROR: Failed to parse config JSON: %v, using defaults", err)
	// 	return defaultConfig
	// }

	// Override with environment variables if set
	if envHost := os.Getenv("MQTT_HOSTNAME"); envHost != "" {
		config.MQTT.Host = envHost
	} else {
		config.MQTT.Host = defaultConfig.MQTT.Host
	}

	if envPort := os.Getenv("MQTT_PORT"); envPort != "" {
		if port, err := strconv.Atoi(envPort); err == nil {
			config.MQTT.Port = port
		} else {
			log.Printf("ERROR: Failed to parse MQTT_PORT %q, using default", envPort)
			config.MQTT.Port = defaultConfig.MQTT.Port
		}
	} else {
		config.MQTT.Port = defaultConfig.MQTT.Port
	}

	if envUser := os.Getenv("MQTT_USERNAME"); envUser != "" {
		config.MQTT.Username = envUser
	} else {
		config.MQTT.Username = defaultConfig.MQTT.Username
	}

	if envPass := os.Getenv("MQTT_PASSWORD"); envPass != "" {
		config.MQTT.Password = envPass
	} else {
		log.Printf("ERROR: MQTT_PASSWORD is not defined in environment variables")
		// os.Exit(1)
	}

	// // Validate and fill missing values with defaults
	// if config.MQTT.Host == "" {
	// 	config.MQTT.Host = defaultConfig.MQTT.Host
	// }
	// if config.MQTT.Port == 0 {
	// 	config.MQTT.Port = defaultConfig.MQTT.Port
	// }
	// if config.MQTT.Username == "" {
	// 	config.MQTT.Username = defaultConfig.MQTT.Username
	// }
	// if config.MQTT.Password == "" {
	// 	config.MQTT.Password = defaultConfig.MQTT.Password
	// }
	if config.UnitOfMeasurement == "" {
		config.UnitOfMeasurement = defaultConfig.UnitOfMeasurement
	}
	if config.Language == "" {
		config.Language = defaultConfig.Language
	}

	log.Printf("INFO: Load variable host: '%s'", config.MQTT.Host)
	log.Printf("INFO: Load variable port: '%d'", config.MQTT.Port)
	log.Printf("INFO: Load variable username: '%s'", config.MQTT.Username)
	// log.Printf("INFO: Config loaded successfully from %s", configPath)
	return config
}

func calculateWindChill(tempF, windSpeedMph string) string {
	tempfVal, _ := strconv.ParseFloat(tempF, 64)
	windspeedVal, _ := strconv.ParseFloat(windSpeedMph, 64)
	wc := 35.74 +
		(0.6215 * tempfVal) -
		(35.75 * math.Pow(windspeedVal, 0.16)) +
		// (0.4275 * math.Pow(windspeedVal, 0.16))
		(0.4275 * tempfVal * math.Pow(windspeedVal, 0.16))
	return strconv.FormatFloat(wc, 'f', 2, 64)
}

func calculateWinDir(windDir string) string {
	// Normalize wind direction to 0-360 range
	val, err := strconv.ParseFloat(windDir, 64)
	if err != nil {
		log.Printf("WARN: Failed to parse wind direction %q, using original", windDir)
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

func calculateWindDirSite(windDir string) string {
	// Parse wind direction to float64
	value, err := strconv.ParseFloat(windDir, 64)
	if err != nil {
		log.Printf("WARN: Failed to parse wind direction %q, using N|S", windDir)
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

func main() {

	// Load configuration with fallback to defaults
	// config := loadConfig("/data/options.json")
	config := loadConfig()

	// // Now you can initialize your MQTT client
	// broker := fmt.Sprintf("tcp://%s:%d", config.MQTT.Host, config.MQTT.Port)
	// fmt.Printf("Connecting to MQTT at: %s with user: %s\n", broker, config.MQTT.Username)

	// temperatureFahrenheit := u.NewValue(tempf, u.Fahrenheit)
	// temperatureCelsius := temperatureFahrenheit.MustConvert(u.Celsius)
	// myLog("Info", "temperatureCelsius:", strconv.FormatFloat(temperatureCelsius.Float(), 'f', 2, 64))

	// windspeedkph := windspeedmph * 1.609344
	// myLog("Info", "windspeedKilometerPerHour:", strconv.FormatFloat(windspeedkph, 'f', 2, 64))

	myLog("Info", "Build version:", version)
	myLog("Info", "Build commit: ", commit)
	myLog("Info", "Build date:   ", date)

	mux := http.NewServeMux()
	// Wrap handleData with config using a closure
	// mux.HandleFunc("/weatherstation/updateweatherstation2.php", handleData)
	mux.HandleFunc("/weatherstation/updateweatherstation.php", func(w http.ResponseWriter, r *http.Request) {
		handleData(w, r, config)
	})

	server := &http.Server{
		Addr:              ":80",
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,  // Protects against Slowloris attacks
		ReadTimeout:       15 * time.Second, // Time to read request body
		WriteTimeout:      15 * time.Second, // Time to write response
		IdleTimeout:       60 * time.Second, // Keep-alive duration
	}

	// log.Printf("Starting server on %s", server.Addr)
	myLog("Info", "Starting server on:", server.Addr)
	if err := server.ListenAndServe(); err != nil {
		// log.Fatalf("Server failed: %s", err)
		myLog("Error", "Server failed: ", err)
	}
}

func registerSensors(client mqtt.Client, sensors []HomeAssistantConfig) {
	var wg sync.WaitGroup

	for _, itemSenzor := range sensors {
		wg.Add(1)

		go func(localSenzor HomeAssistantConfig) {
			defer wg.Done()

			payload, _ := json.Marshal(localSenzor)
			// payload, _ := json.MarshalIndent(localSenzor, "", "  ") // prettyJSON
			// fmt.Printf("....Published: %s\n", payload)

			// Publish to the broker
			// topic := fmt.Sprintf(topicConfig, localSenzor.Name)
			topic := fmt.Sprintf(topicConfig, strings.TrimPrefix(localSenzor.UniqueId, UniqIdPrefix))
			fmt.Printf("....Topic: %s\n", topic)
			token := client.Publish(topic, 1, true, payload)

			if token.Wait() && token.Error() != nil {
				fmt.Printf("--Failed to register %s: %v\n", strings.TrimPrefix(localSenzor.UniqueId, UniqIdPrefix), token.Error())
			}

		}(itemSenzor)
	}

	// Wait for ALL goroutines to finish
	wg.Wait()
	fmt.Println("--All sensors registered successfully in parallel!")
}

func handleData(w http.ResponseWriter, r *http.Request, config Config) {
	var sensors []HomeAssistantConfig

	// Fill the default values for HomeAssistant Origin MQTT config
	homeAssistantOrigin := FillDefaultHomeAssistantOrigin()
	log.Printf("Default Origin: %+v", homeAssistantOrigin)

	// Fill the default values for HomeAssistant Device MQTT config
	homeAssistantDevice := FillDefaultHomeAssistantDevice()
	homeAssistantDevice.ModeId = Id

	modelName, modelVersion := GetDeviceModelInfo(Id)
	homeAssistantDevice.Model = strings.ToUpper(modelName)
	homeAssistantDevice.HwVersion = modelVersion
	log.Printf("Default Device: %+v", homeAssistantDevice)

	var data = make(map[string]string)
	if err := r.ParseForm(); err != nil {
		myLog("Error", "Message: ", err)
		return
	}

	for key, values := range r.Form {
		for _, val := range values {
			// Skip empty entries
			if key == "" || val == "" {
				log.Printf("WARN: Skipping empty key or value - key=%q, value=%q", key, val)
				continue
			}
			data[key] = val
		}
	}

	data["windchillf"] = calculateWindChill(data["tempf"], data["windspeedmph"])
	data["winddir"] = calculateWinDir(data["winddir"])
	data["winddirsite"] = calculateWindDirSite(data["winddir"])

	log.Printf("data: %s\n", data)

	jsonOriginalData, err := json.Marshal(data)
	if err != nil {
		myLog("Error", "Message: ", err)
		return
	}
	myLog("Debug", "Original data:", jsonOriginalData)

	// Process and validate each sensor
	for key, value := range data {
		// Input validation
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)

		// log.Printf("Processing - key=%s, value=%s", key, value)
		// log.Printf("UnitOfMeasurement=%s", config.UnitOfMeasurement)
		// log.Printf("Language=%s", config.Language)

		status, deviceCl, unitOfMesure, localizedName, convertedValue, measurement := transformInput(key, value, config)
		// log.Printf("Processing end - key=%s, value=%s", key, value)

		// log.Printf("Sensor config: status=%s, class=%s, unit=%s, name=%s, converted=%s", status, deviceCl, unitOfMesure, localizedName, convertedValue)
		data[key] = convertedValue
		// var deviceCl string
		// var unitOfMesure string
		// // if key == "temperature" {
		// deviceCl = "temperature"
		// unitOfMesure = "°F"
		// // }

		if status == "enabled" {
			// name, class, unit, key, id, entity
			sensors = append(sensors, addSensor(
				// key,           // Name
				localizedName,                           // Name
				deviceCl,                                // deviceClass
				unitOfMesure,                            // UnitOfMeasurement
				fmt.Sprintf("{{ value_json.%s }}", key), // ValueTemplate
				// convertedValue,                                 // ValueTemplate
				fmt.Sprintf("%s%s", UniqIdPrefix, key),         // UniqueId
				fmt.Sprintf("sensor.%s_%s", hexaDumpName, key), // DefaultEntityId
				measurement, // StateClass
			))
		}
	}

	// jsonData, err := json.Marshal(convertedData)
	// if err != nil {
	// 	myLog("Error", "Message: ", err)
	// 	return
	// }
	jsonData, err := json.Marshal(data)
	if err != nil {
		myLog("Error", "Message: ", err)
		return
	}
	myLog("Info", "Converted data:", jsonData)
	// payload, err := json.MarshalIndent(sensors, "", "  ")

	// Correct logging
	// payload, err := json.Marshal(sensors)
	// if err != nil {
	// 	log.Fatalf("Error marshaling sensors: %s", err)
	// }

	// fmt.Println("--- Registered Sensors Configuration ---")
	// // fmt.Println(string(payload))
	// fmt.Printf("..Published: %s\n", payload)

	opts := mqtt.NewClientOptions()

	// Now you can initialize your MQTT client
	broker := fmt.Sprintf("tcp://%s:%d", config.MQTT.Host, config.MQTT.Port)
	fmt.Printf("Connecting to MQTT at: %s with user: %s\n", broker, config.MQTT.Username)

	// opts.SetUsername(os.Getenv("MQTT_USERNAME"))
	// opts.SetPassword(os.Getenv("MQTT_PASSWORD"))
	opts.SetUsername(config.MQTT.Username)
	opts.SetPassword(config.MQTT.Password)
	opts.AddBroker(broker)
	opts.SetClientID(clientID)
	opts.SetDefaultPublishHandler(messagePubHandler)

	opts.SetKeepAlive(30 * time.Second)
	opts.SetDefaultPublishHandler(func(client mqtt.Client, msg mqtt.Message) {
		log.Printf("TOPIC: %s\n", msg.Topic())
		log.Printf("MSG: %s\n", msg.Payload())
	})

	opts.SetPingTimeout(1 * time.Second)
	opts.SetAutoReconnect(true)
	opts.SetMaxReconnectInterval(10 * time.Second)
	opts.SetConnectRetry(true)
	opts.SetConnectRetryInterval(10 * time.Second)
	opts.SetWriteTimeout(60 * time.Second)

	opts.OnConnect = connectHandler
	opts.OnConnectionLost = connectLostHandler

	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		myLog("Info", "MQTT:", token.Error())
	}
	defer client.Disconnect(250)

	registerSensors(client, sensors)

	client.Publish(topic, 1, true, jsonData).Wait()
	// fmt.Printf("..Published State: %s\n", jsonData)

	client.Disconnect(250)
}
