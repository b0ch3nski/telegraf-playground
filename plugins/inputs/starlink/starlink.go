//go:generate ../../../tools/readme_config_includer/generator
package starlink

import (
	"context"
	_ "embed"
	"time"

	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/config"
	"github.com/influxdata/telegraf/plugins/inputs"

	"github.com/b0ch3nski/go-starlink/starlink"
)

var (
	_ telegraf.Input       = (*Starlink)(nil)
	_ telegraf.Initializer = (*Starlink)(nil)
)

//go:embed sample.conf
var sampleConfig string

type Starlink struct {
	Address string          `toml:"address"`
	Timeout config.Duration `toml:"timeout"`
	Log     telegraf.Logger `toml:"-"`

	client starlink.Client
}

func (*Starlink) SampleConfig() string {
	return sampleConfig
}

func (s *Starlink) Init() error {
	cl, err := starlink.NewClient(context.Background(), s.Address)
	if err != nil {
		return err
	}

	s.client = cl.WithTimeout(time.Duration(s.Timeout))
	return nil
}

func (s *Starlink) Gather(acc telegraf.Accumulator) error {
	st, err := s.client.Status(context.Background())
	if err != nil {
		return err
	}

	devInfo := st.GetDeviceInfo()
	obstrStats := st.GetObstructionStats()
	aligStats := st.GetAlignmentStats()

	fields := map[string]any{
		"boot_count": devInfo.GetBootcount(),
		"uptime_sec": st.GetDeviceState().GetUptimeS(),
		"outage_ns":  st.GetOutage().GetDurationNs(),
		"heating":    st.GetAlerts().GetIsHeating(),

		"ping_drop_rate":          st.GetPopPingDropRate(),
		"ping_latency_ms":         st.GetPopPingLatencyMs(),
		"downlink_throughput_bps": st.GetDownlinkThroughputBps(),
		"uplink_throughput_bps":   st.GetUplinkThroughputBps(),

		"obstruction_percentage":          obstrStats.GetFractionObstructed(),
		"obstruction_time":                obstrStats.GetTimeObstructed(),
		"gps_sats":                        st.GetGpsStats().GetGpsSats(),
		"tilt_angle_deg":                  aligStats.GetTiltAngleDeg(),
		"boresight_azimuth_deg":           aligStats.GetBoresightAzimuthDeg(),
		"boresight_elevation_deg":         aligStats.GetBoresightElevationDeg(),
		"desired_boresight_azimuth_deg":   aligStats.GetDesiredBoresightAzimuthDeg(),
		"desired_boresight_elevation_deg": aligStats.GetDesiredBoresightElevationDeg(),
		"attitude_uncertainty_deg":        aligStats.GetAttitudeUncertaintyDeg(),
	}

	acc.AddGauge("starlink", fields, map[string]string{"id": devInfo.GetId()})
	return nil
}

func init() {
	inputs.Add("starlink", func() telegraf.Input {
		return &Starlink{
			Address: starlink.DefaultDishyAddr,
			Timeout: config.Duration(3 * time.Second),
		}
	})
}
