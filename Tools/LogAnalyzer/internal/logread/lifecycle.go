package logread

import (
	"encoding/json"

	"mosaic-log-analyzer/internal/domain"
)

// lenientString decodes a JSON string and treats any other JSON type as absent,
// so a wrongly typed optional field never makes the whole line unusable.
type lenientString string

// UnmarshalJSON implements json.Unmarshaler.
func (s *lenientString) UnmarshalJSON(b []byte) error {
	var v string
	if string(b) != "null" && json.Unmarshal(b, &v) == nil {
		*s = lenientString(v)
	}
	return nil
}

// lenientBool decodes a JSON boolean and treats any other JSON type as absent.
type lenientBool domain.OptionalBool

// UnmarshalJSON implements json.Unmarshaler.
func (o *lenientBool) UnmarshalJSON(b []byte) error {
	var v bool
	if string(b) != "null" && json.Unmarshal(b, &v) == nil {
		*o = lenientBool(domain.BoolOf(v))
	}
	return nil
}

// wireLifecycle holds the optional wire fields of the run, session and
// notification events. Content fields (cwd, notification message) are
// deliberately absent.
type wireLifecycle struct {
	AdapterVersion   lenientString `json:"adapter_version"`
	Outcome          lenientString `json:"outcome"`
	Reason           lenientString `json:"reason"`
	Resumed          lenientBool   `json:"resumed"`
	NotificationType lenientString `json:"notification_type"`
	RequiresResponse lenientBool   `json:"requires_response"`
	Resolution       lenientString `json:"resolution"`
	NotificationID   lenientString `json:"notification_id"`
}

// applyLifecycle sets the block for the run, session and notification event
// types on ev. It reports whether the wire event type was one of them.
func applyLifecycle(ev *domain.Event, wire wireEvent) bool {
	switch domain.EventType(wire.Event) {
	case domain.EventRunStart:
		ev.Type = domain.EventRunStart
		ev.RunStart = &domain.RunStartFields{
			AdapterVersion: string(wire.AdapterVersion),
			Model:          domain.ModelID(wire.Model),
			Provider:       domain.ProviderID(wire.Provider),
		}
	case domain.EventRunEnd:
		ev.Type = domain.EventRunEnd
		ev.RunEnd = &domain.RunEndFields{Outcome: string(wire.Outcome)}
	case domain.EventSessionStart:
		ev.Type = domain.EventSessionStart
		ev.SessionStart = &domain.SessionStartFields{
			Resumed:  domain.OptionalBool(wire.Resumed),
			Model:    domain.ModelID(wire.Model),
			Provider: domain.ProviderID(wire.Provider),
		}
	case domain.EventSessionEnd:
		ev.Type = domain.EventSessionEnd
		ev.SessionEnd = &domain.SessionEndFields{Reason: string(wire.Reason)}
	case domain.EventNotification:
		ev.Type = domain.EventNotification
		ev.Notification = &domain.NotificationFields{
			NotificationType: string(wire.NotificationType),
			RequiresResponse: domain.OptionalBool(wire.RequiresResponse),
			Resolution:       string(wire.Resolution),
			NotificationID:   string(wire.NotificationID),
		}
	default:
		return false
	}
	return true
}
