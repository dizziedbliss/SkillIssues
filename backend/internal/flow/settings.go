package flow

import "github.com/dizziedbliss/skillissues/internal/settings"

// SettingsLimits reads a Publisher's limits from the live settings.
func SettingsLimits(live func() *settings.Snapshot) func() Limits {
	return func() Limits {
		s := live()
		return Limits{
			Enabled:        s.Bool(settings.FlowEnabled),
			BufferSize:     s.Int(settings.FlowBufferSize),
			PublishTimeout: s.Millis(settings.FlowPublishTimeoutMs),
			MaxLabelChars:  s.Int(settings.FlowMaxLabelChars),
		}
	}
}
