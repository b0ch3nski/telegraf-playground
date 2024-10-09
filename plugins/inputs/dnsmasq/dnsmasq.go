//go:generate ../../../tools/readme_config_includer/generator
package dnsmasq

import (
	"context"
	_ "embed"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"sync"

	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/metric"
	"github.com/influxdata/telegraf/plugins/inputs"

	godnsmasq "github.com/b0ch3nski/go-dnsmasq-utils/dnsmasq"
)

var (
	_ telegraf.ServiceInput = (*DNSMasq)(nil)
	_ telegraf.Initializer  = (*DNSMasq)(nil)
)

//go:embed sample.conf
var sampleConfig string

type DNSMasq struct {
	LogFilePath string          `toml:"log_file_path"`
	Log         telegraf.Logger `toml:"-"`

	wg     sync.WaitGroup
	cancel context.CancelFunc
}

func (*DNSMasq) SampleConfig() string {
	return sampleConfig
}

func (d *DNSMasq) Init() error {
	if d.LogFilePath == "" {
		return errors.New("'log_file_path' is required")
	}
	if err := os.MkdirAll(filepath.Dir(d.LogFilePath), os.ModePerm); err != nil {
		return err
	}
	return nil
}

func (d *DNSMasq) Start(acc telegraf.Accumulator) error {
	ctx, cancel := context.WithCancel(context.Background())
	d.cancel = cancel

	queryChan := make(chan *godnsmasq.Query)

	go func() {
		if err := godnsmasq.WatchLogs(ctx, d.LogFilePath, queryChan, nil); err != nil {
			acc.AddError(err)
		}
	}()

	d.wg.Add(1)
	go func() {
		defer d.wg.Done()

		for query := range queryChan {
			fields := map[string]any{
				"domain":  query.Domain,
				"made_by": query.MadeBy,
				"took_ms": query.Finished.Sub(query.Started).Milliseconds(),
			}
			for i, q := range query.Queried {
				fields["queried_"+strconv.Itoa(i+1)] = q
			}
			for i, r := range query.Result {
				fields["result_"+strconv.Itoa(i+1)] = r
			}

			acc.AddMetric(metric.New("dnsmasq", map[string]string{}, fields, query.Finished))
		}
	}()

	d.Log.Infof("Watching DNSMasq logs on %s", d.LogFilePath)
	return nil
}

func (d *DNSMasq) Stop() {
	d.cancel()
	d.wg.Wait()

	d.Log.Debug("Exiting DNSMasq logs watcher")
}

func (*DNSMasq) Gather(_ telegraf.Accumulator) error {
	return nil
}

func init() {
	inputs.Add("dnsmasq", func() telegraf.Input { return &DNSMasq{} })
}
