# Google Cloud IoT Core over MQTT in Go

[![GoDoc](https://godoc.org/github.com/mtraver/iotcore?status.svg)](https://godoc.org/github.com/mtraver/iotcore)
[![Go Report Card](https://goreportcard.com/badge/github.com/mtraver/iotcore)](https://goreportcard.com/report/github.com/mtraver/iotcore)

*NOTE:* Google shut down Google Cloud IoT Core on August 16, 2023. This repository has been archived.

Package `iotcore` eases interaction with Google Cloud IoT Core over MQTT. It handles TLS configuration
and authentication. It also makes it easy to construct the fully-qualified MQTT topics that Cloud IoT
Core uses for configuration, telemetry, state, and commands.
