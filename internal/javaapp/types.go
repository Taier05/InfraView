package javaapp

import (
	"crypto/sha256"
	"encoding/base64"
	"strings"
	"time"
)

type Service struct {
	ID                        string
	Name                      string
	Address                   string
	HealthLatencyMilliseconds *float64
	HealthUp                  *bool
	PortUp                    *bool
	ProcessUp                 *bool
	PortConsistent            *bool
	ProcessCount              *int64
	ProcessMemoryBytes        *int64
	ProcessCPUPercent         *float64
	ProcessMemoryPercent      *float64
	ProcessStartTimeSeconds   *int64
	CollectionTracked         bool
	ReportedAt                time.Time
}

type Snapshot struct {
	Services []Service
}

func StableServiceID(name, address string) string {
	name = strings.TrimSpace(name)
	address = strings.TrimSpace(address)
	sum := sha256.Sum256([]byte("java-service\x00" + name + "\x00" + address))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func (snapshot Snapshot) Clone() Snapshot {
	var services []Service
	if snapshot.Services != nil {
		services = make([]Service, len(snapshot.Services))
		for index, service := range snapshot.Services {
			services[index] = cloneService(service)
		}
	}
	return Snapshot{Services: services}
}

func cloneService(service Service) Service {
	cloned := service
	cloned.HealthLatencyMilliseconds = clonePointer(service.HealthLatencyMilliseconds)
	cloned.HealthUp = clonePointer(service.HealthUp)
	cloned.PortUp = clonePointer(service.PortUp)
	cloned.ProcessUp = clonePointer(service.ProcessUp)
	cloned.PortConsistent = clonePointer(service.PortConsistent)
	cloned.ProcessCount = clonePointer(service.ProcessCount)
	cloned.ProcessMemoryBytes = clonePointer(service.ProcessMemoryBytes)
	cloned.ProcessCPUPercent = clonePointer(service.ProcessCPUPercent)
	cloned.ProcessMemoryPercent = clonePointer(service.ProcessMemoryPercent)
	cloned.ProcessStartTimeSeconds = clonePointer(service.ProcessStartTimeSeconds)
	return cloned
}

func clonePointer[T any](value *T) *T {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}
