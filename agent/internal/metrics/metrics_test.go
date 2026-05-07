package metrics

import (
	"strings"
	"testing"
)

func TestUpdateAndExposition(t *testing.T) {
	m := New()
	m.Update("8.8.8.8", true, 24.5, 0)
	m.Update("1.1.1.1", false, 0, 100)

	exp := m.Exposition()

	checks := []string{
		`network_up{target="8.8.8.8"} 1`,
		`network_up{target="1.1.1.1"} 0`,
		`network_latency_ms{target="8.8.8.8"} 24.5`,
		`network_packet_loss{target="1.1.1.1"} 100`,
	}
	for _, c := range checks {
		if !strings.Contains(exp, c) {
			t.Errorf("missing %q in exposition:\n%s", c, exp)
		}
	}
}
