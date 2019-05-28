package iotcore

import "fmt"

var (
	DefaultBroker = MQTTBroker{
		Host: "mqtt.googleapis.com",
		Port: 8883,
	}

	DefaultBroker443 = MQTTBroker{
		Host: "mqtt.googleapis.com",
		Port: 443,
	}

	LTSBroker = MQTTBroker{
		Host: "mqtt.2030.ltsapis.goog",
		Port: 8883,
	}

	LTSBroker443 = MQTTBroker{
		Host: "mqtt.2030.ltsapis.goog",
		Port: 443,
	}
)

// MQTTBroker represents an MQTT server.
type MQTTBroker struct {
	Host string
	Port int
}

// URL returns the URL of the MQTT server.
func (b *MQTTBroker) URL() string {
	return fmt.Sprintf("ssl://%v:%v", b.Host, b.Port)
}

// String returns a string representation of the MQTTBroker.
func (b *MQTTBroker) String() string {
	return b.URL()
}
