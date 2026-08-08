package disk

import (
	"crypto/sha256"
	"encoding/base64"
	"time"
)

type Health string
type AttributeFailure string

const (
	HealthHealthy Health = "healthy"
	HealthFailed  Health = "failed"
	HealthUnknown Health = "unknown"

	AttributeFailureNone AttributeFailure = "none"
	AttributeFailurePast AttributeFailure = "past"
	AttributeFailureNow  AttributeFailure = "now"
)

type ErrorCounters struct {
	PendingSectors       *float64
	ReallocatedSectors   *float64
	UncorrectableSectors *float64
	UDMACRCErrors        *float64
	MediaIntegrityErrors *float64
	ErrorLogEntries      *float64
	CommandTimeouts      *float64
	UnsafeShutdowns      *float64
}

type Device struct {
	ID                             string
	HostID                         string
	Device                         string
	Model                          string
	CapacityBytes                  *int64
	SMARTHealth                    Health
	TemperatureCelsius             *float64
	LifetimeUsedPercent            *float64
	PowerOnHours                   *float64
	CriticalWarning                *int64
	AvailableSparePercent          *float64
	AvailableSpareThresholdPercent *float64
	AttributeFailure               AttributeFailure
	Errors                         ErrorCounters
	CollectionTracked              bool
	ReportedAt                     time.Time
}

type Snapshot struct {
	Devices []Device
}

func StableDeviceID(hostID, wwn, serialNo, device string) string {
	if hostID == "" {
		return ""
	}
	identityKind, identityValue := firstNonEmptyIdentity(wwn, serialNo, device)
	if identityValue == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(hostID + "\x00" + identityKind + "\x00" + identityValue))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func firstNonEmptyIdentity(wwn, serialNo, device string) (string, string) {
	if wwn != "" {
		return "wwn", wwn
	}
	if serialNo != "" {
		return "serial_no", serialNo
	}
	if device != "" {
		return "device", device
	}
	return "", ""
}
