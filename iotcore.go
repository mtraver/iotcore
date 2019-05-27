// Package iotcore provides an abstraction around Google Cloud IoT Core devices and eases interaction with Cloud IoT Core over MQTT.
package iotcore

import (
	"crypto/ecdsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"os"
	"time"

	jwt "github.com/dgrijalva/jwt-go"
	MQTT "github.com/eclipse/paho.mqtt.golang"
)

// NewMQTTOptions creates a Paho MQTT ClientOptions that may be used to connect to the given MQTT bridge using TLS.
func NewMQTTOptions(device Device, bridge MQTTBridge, caCertsPath string) (*MQTT.ClientOptions, error) {
	// Load CA certs
	certpool := x509.NewCertPool()
	pemCerts, err := ioutil.ReadFile(caCertsPath)
	if err != nil {
		return nil, fmt.Errorf("iotcore: failed to read CA certs: %v", err)
	}
	certpool.AppendCertsFromPEM(pemCerts)

	tlsConf := &tls.Config{
		RootCAs:            certpool,
		ClientAuth:         tls.NoClientCert,
		ClientCAs:          nil,
		InsecureSkipVerify: true,
		Certificates:       []tls.Certificate{},
		MinVersion:         tls.VersionTLS12,
	}

	opts := MQTT.NewClientOptions()
	opts.AddBroker(bridge.URL())
	opts.SetClientID(device.ClientID())
	opts.SetTLSConfig(tlsConf)
	opts.SetUsername("unused")

	return opts, nil
}

// DeviceIDFromCert gets the Common Name from an X.509 cert, which in this case is known to be the device ID.
func DeviceIDFromCert(certPath string) (string, error) {
	certBytes, err := ioutil.ReadFile(certPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("iotcore: cert file does not exist: %v", certPath)
		}

		return "", fmt.Errorf("iotcore: failed to read cert: %v", err)
	}

	block, _ := pem.Decode(certBytes)
	if block == nil || block.Type != "CERTIFICATE" {
		return "", fmt.Errorf("iotcore: failed to decode PEM certificate")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return "", err
	}

	return cert.Subject.CommonName, nil
}

// Device represents a Google Cloud IoT Core device.
type Device struct {
	ProjectID   string
	RegistryID  string
	DeviceID    string
	PrivKeyPath string
	Region      string
}

// ClientID returns the fully-qualified Google Cloud IoT Core device ID.
func (d *Device) ClientID() string {
	return fmt.Sprintf("projects/%v/locations/%v/registries/%v/devices/%v", d.ProjectID, d.Region, d.RegistryID, d.DeviceID)
}

// ConfigTopic returns the MQTT topic to which the device can subscribe to get configuration updates.
func (d *Device) ConfigTopic() string {
	return fmt.Sprintf("/devices/%v/config", d.DeviceID)
}

// CommandTopic returns the MQTT topic to which the device can subscribe to get commands. The topic returned
// ends with a wildcard, which Cloud IoT Core requires. Subscribing to a specific subfolder is not supported.
// For more information see https://cloud.google.com/iot/docs/how-tos/commands.
func (d *Device) CommandTopic() string {
	return fmt.Sprintf("/devices/%v/commands/#", d.DeviceID)
}

// TelemetryTopic returns the MQTT topic to which the device should publish telemetry events.
func (d *Device) TelemetryTopic() string {
	return fmt.Sprintf("/devices/%v/events", d.DeviceID)
}

// StateTopic returns the MQTT topic to which the device should publish state information.
// This is optionally configured in the device registry. For more information see
// https://cloud.google.com/iot/docs/how-tos/config/getting-state.
func (d *Device) StateTopic() string {
	return fmt.Sprintf("/devices/%v/state", d.DeviceID)
}

func (d *Device) publicKey() (*ecdsa.PublicKey, error) {
	priv, err := d.privateKey()
	if err != nil {
		return nil, err
	}

	return &priv.PublicKey, nil
}

func (d *Device) privateKey() (*ecdsa.PrivateKey, error) {
	keyBytes, err := ioutil.ReadFile(d.PrivKeyPath)
	if err != nil {
		return nil, err
	}

	return jwt.ParseECPrivateKeyFromPEM(keyBytes)
}

// VerifyJWT checks the validity of the given JWT, including its signature and expiration. It returns true
// with a nil error if the JWT is valid. Both false and a non-nil error (regardless of the accompanying
// boolean value) indicate an invalid JWT.
func (d *Device) VerifyJWT(jwtStr string) (bool, error) {
	token, err := jwt.Parse(jwtStr, func(token *jwt.Token) (interface{}, error) {
		// Validate the signing algorithm.
		if _, ok := token.Method.(*jwt.SigningMethodECDSA); !ok {
			return nil, fmt.Errorf("iotcore: unexpected signing method %v", token.Header["alg"])
		}

		return d.publicKey()
	})

	if err != nil {
		return false, err
	}

	return token.Valid, err
}

// NewJWT creates a new JWT signed with the device's key and expiring in the given amount of time.
func (d *Device) NewJWT(exp time.Duration) (string, error) {
	key, err := d.privateKey()
	if err != nil {
		return "", fmt.Errorf("iotcore: failed to parse priv key: %v", err)
	}

	token := jwt.New(jwt.SigningMethodES256)
	token.Claims = jwt.StandardClaims{
		Audience:  d.ProjectID,
		IssuedAt:  time.Now().Unix(),
		ExpiresAt: time.Now().Add(exp).Unix(),
	}

	return token.SignedString(key)
}

// MQTTBridge represents an MQTT server.
type MQTTBridge struct {
	Host string
	Port int
}

// URL returns the URL to the MQTT server.
func (b *MQTTBridge) URL() string {
	return fmt.Sprintf("ssl://%v:%v", b.Host, b.Port)
}
