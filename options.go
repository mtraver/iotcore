package iotcore

import (
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// JWTTTL sets the TLL of JWTs created when connecting to the MQTT broker.
// This is an option meant to be passed to NewClient.
func JWTTTL(ttl time.Duration) func(Device, *mqtt.ClientOptions) error {
	return func(d Device, opts *mqtt.ClientOptions) error {
		opts.SetCredentialsProvider(d.credentialsProvider(ttl))
		return nil
	}
}

// CacheJWT caches the JWTs created when connecting to the MQTT broker. When (re)connecting the cached JWT is
// checked for validity (including expiration) and is reused if valid. If the cached JWT is invalid, a new JWT is
// created and cached. This is an option meant to be passed to NewClient.
func CacheJWT(ttl time.Duration) func(Device, *mqtt.ClientOptions) error {
	return func(d Device, opts *mqtt.ClientOptions) error {
		opts.SetCredentialsProvider(d.cachedCredentialsProvider(ttl))
		return nil
	}
}

// PersistentlyCacheJWT caches to disk the JWTs created when connecting to the MQTT broker. When (re)connecting the
// cached JWT is read from disk and checked for validity (including expiration) and is reused if valid. If the cached
// JWT is invalid, a new JWT is created and saved to disk. This is an option meant to be passed to NewClient.
func PersistentlyCacheJWT(ttl time.Duration, path string) func(Device, *mqtt.ClientOptions) error {
	return func(d Device, opts *mqtt.ClientOptions) error {
		opts.SetCredentialsProvider(d.persistentlyCachedCredentialsProvider(ttl, path))
		return nil
	}
}
