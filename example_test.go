package iotcore_test

import (
	"log"
	"os"
	"time"

	"github.com/mtraver/iotcore"
)

func Example() {
	d := iotcore.Device{
		ProjectID:   "my-gcp-project",
		RegistryID:  "my-iot-core-registry",
		DeviceID:    "my-device",
		PrivKeyPath: "my-device.pem",
		Region:      "us-central1",
	}

	certs, err := os.Open("roots.pem")
	if err != nil {
		log.Fatalf("Failed to open certs file: %v", err)
	}
	defer certs.Close()

	client, err := d.NewClient(iotcore.DefaultBroker, certs)
	if err != nil {
		log.Fatalf("Failed to make MQTT client: %v", err)
	}

	if token := client.Connect(); !token.Wait() || token.Error() != nil {
		log.Fatalf("Failed to connect to MQTT broker: %v", token.Error())
	}

	if token := client.Publish(d.TelemetryTopic(), 1, false, []byte("{\"temp\": 18.0}")); !token.Wait() || token.Error() != nil {
		log.Printf("Failed to publish: %v", token.Error())
	}

	client.Disconnect(250)
	time.Sleep(500 * time.Millisecond)
}
