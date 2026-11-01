package metricstore

import (
	"context"
	"testing"
	"time"

	v1 "github.com/komari-monitor/komari/protocol/v1"
)

// Adapted from upstream bf6b45e for the fork's v1/v2-compatible store.
func TestWriteReportRebasesTrafficAfterAgentRestart(t *testing.T) {
	ctx := context.Background()
	s := useReportTestStore(t, nil)
	base := time.Now().UTC().Truncate(time.Minute).Add(5 * time.Second)
	report := v1.Report{UUID: "restarted-node", UpdatedAt: base}
	for i, sample := range []struct{ uptime, up, down int64 }{
		{1000, 100, 200}, {1003, 150, 260}, {1, 155, 265}, {4, 180, 300},
	} {
		report.UpdatedAt = base.Add(time.Duration(i*3) * time.Second)
		report.Uptime = sample.uptime
		report.Network.TotalUp = sample.up
		report.Network.TotalDown = sample.down
		if _, err := WriteReport(ctx, report); err != nil {
			t.Fatalf("write sample %d: %v", i, err)
		}
	}
	assertMetricValues(t, s, MetricTrafficUp, report.UUID, base.Add(-time.Second), base.Add(time.Minute), []float64{0, 50, 0, 25})
	assertMetricValues(t, s, MetricTrafficDown, report.UUID, base.Add(-time.Second), base.Add(time.Minute), []float64{0, 60, 0, 35})
}
