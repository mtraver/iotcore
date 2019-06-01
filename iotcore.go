package iotcore

import (
	"crypto/ecdsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"sync"
	"time"

	jwt "github.com/dgrijalva/jwt-go"
	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// Google Cloud IoT Core's MQTT brokers ignore the password when authenticating (they only care about the JWT).
const username = "unused"

// DeviceIDFromCert gets the Common Name from an X.509 cert, which for the purposes of this package is considered to be the device ID.
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
	ProjectID   string `json:"project_id"`
	RegistryID  string `json:"registry_id"`
	DeviceID    string `json:"device_id"`
	PrivKeyPath string `json:"priv_key_path"`
	Region      string `json:"region"`

	// token is used to cache JWTs used for authenticating with Google Cloud IoT Core.
	token string
	tmu   sync.Mutex
}

// NewClient creates a github.com/eclipse/paho.mqtt.golang Client that may be used to connect to the given MQTT broker using TLS,
// which Google Cloud IoT Core requires. By default it sets up a github.com/eclipse/paho.mqtt.golang ClientOptions with the minimal
// options required to establish a connection:
//
//   • Client ID
//   • TLS configuration
//   • Broker
//   • A credentials provider that creates a new JWT with TTL 1 minute on each connection attempt.
//
// By passing in options you may customize the ClientOptions. Options are functions with this signature:
//
//   func(Device, *mqtt.ClientOptions) error
//
// They modify the ClientOptions. The option functions are applied to the ClientOptions in the order given before the
// Client is created. Some options are provided in this package (see options.go), but you may create your own as well.
// For example, if you wish to set the connect timeout, you might write this:
//
//   func ConnectTimeout(t time.Duration) func(Device, *mqtt.ClientOptions) error {
//   	return func(d Device, opts *mqtt.ClientOptions) error {
//   		opts.SetConnectTimeout(t)
//   		return nil
//   	}
//   }
//
// Using option functions allows for sensible defaults — no options are required to establish a
// connection — without loss of customizability.
//
// For more information about connecting to Google Cloud IoT Core's MQTT brokers see https://cloud.google.com/iot/docs/how-tos/mqtt-bridge.
func (d *Device) NewClient(broker MQTTBroker, caCerts io.Reader, options ...func(Device, *mqtt.ClientOptions) error) (mqtt.Client, error) {
	// Load CA certs.
	pemCerts, err := ioutil.ReadAll(caCerts)
	if err != nil {
		return nil, fmt.Errorf("iotcore: failed to read CA certs: %v", err)
	}
	certpool := x509.NewCertPool()
	if !certpool.AppendCertsFromPEM(pemCerts) {
		return nil, fmt.Errorf("iotcore: no certs were parsed from given CA certs")
	}

	tlsConf := &tls.Config{
		RootCAs:            certpool,
		ClientAuth:         tls.NoClientCert,
		ClientCAs:          nil,
		InsecureSkipVerify: true,
		Certificates:       []tls.Certificate{},
		MinVersion:         tls.VersionTLS12,
	}

	opts := mqtt.NewClientOptions()
	opts.AddBroker(broker.URL())
	opts.SetClientID(d.ClientID())
	opts.SetTLSConfig(tlsConf)
	opts.SetCredentialsProvider(d.credentialsProvider(1 * time.Minute))

	for _, option := range options {
		if err := option(*d, opts); err != nil {
			return nil, err
		}
	}

	return mqtt.NewClient(opts), nil
}

func (d *Device) credentialsProvider(ttl time.Duration) mqtt.CredentialsProvider {
	return func() (string, string) {
		token, err := d.NewJWT(ttl)
		if err != nil {
			// We have no way to return an error, so set the JWT to a value that will fail
			// when used to authenticate.
			token = "error making new JWT"
		}
		return username, token
	}
}

func (d *Device) cachedCredentialsProvider(ttl time.Duration) mqtt.CredentialsProvider {
	return func() (string, string) {
		d.tmu.Lock()
		defer d.tmu.Unlock()

		// Check the cached JWT's validity. If any errors are encountered or if the JWT
		// is not valid then we will make a new JWT.
		if ok, err := d.VerifyJWT(d.token); ok && err == nil {
			return username, d.token
		}

		token, err := d.NewJWT(ttl)
		if err != nil {
			// We have no way to return an error, so set the JWT to a value that will fail
			// when used to authenticate.
			token = "error making new JWT"
		} else {
			// Cache the JWT.
			d.token = token
		}

		return username, token
	}
}

func (d *Device) persistentlyCachedCredentialsProvider(ttl time.Duration, path string) mqtt.CredentialsProvider {
	return func() (string, string) {
		d.tmu.Lock()
		defer d.tmu.Unlock()

		// Read the cached JWT and check its validity. If any errors are encountered or if the JWT
		// is not valid then we will make a new JWT.
		b, err := ioutil.ReadFile(path)
		if err == nil {
			if ok, err := d.VerifyJWT(string(b)); ok && err == nil {
				return username, string(b)
			}
		}

		token, err := d.NewJWT(ttl)
		if err != nil {
			// We have no way to return an error, so set the JWT to a value that will fail
			// when used to authenticate.
			token = "error making new JWT"
		} else {
			// Persist the JWT. Ignore the error returned by ioutil.WriteFile because the signature
			// of mqtt.CredentialsProvider provides no way to return it.
			ioutil.WriteFile(path, []byte(token), 0600)
		}

		return username, token
	}
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
func (d *Device) NewJWT(ttl time.Duration) (string, error) {
	key, err := d.privateKey()
	if err != nil {
		return "", fmt.Errorf("iotcore: failed to parse priv key: %v", err)
	}

	token := jwt.New(jwt.SigningMethodES256)
	token.Claims = jwt.StandardClaims{
		Audience:  d.ProjectID,
		IssuedAt:  time.Now().Unix(),
		ExpiresAt: time.Now().Add(ttl).Unix(),
	}

	return token.SignedString(key)
}
