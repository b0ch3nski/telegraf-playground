//go:generate ../../../tools/readme_config_includer/generator
package dhcp

import (
	"context"
	_ "embed"
	"errors"
	"os"
	"path/filepath"
	"sync"

	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/plugins/processors"

	"github.com/b0ch3nski/go-dnsmasq-utils/dnsmasq"
)

var (
	_ telegraf.StreamingProcessor = (*DHCP)(nil)
	_ telegraf.Initializer        = (*DHCP)(nil)
)

//go:embed sample.conf
var sampleConfig string

type DHCP struct {
	LeasesFilePath string          `toml:"leases_file_path"`
	IPFields       []string        `toml:"ip_fields"`
	IPTags         []string        `toml:"ip_tags"`
	Log            telegraf.Logger `toml:"-"`

	leases map[string]*dnsmasq.Lease
	mtx    sync.RWMutex
	wg     sync.WaitGroup
	cancel context.CancelFunc
}

func (*DHCP) SampleConfig() string {
	return sampleConfig
}

func (d *DHCP) Init() error {
	if len(d.IPFields) == 0 && len(d.IPTags) == 0 {
		return errors.New("at least one of 'ip_fields' or 'ip_tags' is required")
	}
	if d.LeasesFilePath == "" {
		return errors.New("'leases_file_path' is required")
	}
	if errMkdir := os.MkdirAll(filepath.Dir(d.LeasesFilePath), os.ModePerm); errMkdir != nil {
		return errMkdir
	}
	d.leases = make(map[string]*dnsmasq.Lease)

	leaseFile, errOpen := os.OpenFile(d.LeasesFilePath, os.O_RDONLY|os.O_CREATE, 0644)
	if errOpen != nil {
		return errOpen
	}
	defer leaseFile.Close()

	leases, errRead := dnsmasq.ReadLeases(leaseFile)
	if errRead != nil {
		return errRead
	}
	d.replaceLeases(leases)

	return nil
}

func (d *DHCP) Start(acc telegraf.Accumulator) error {
	ctx, cancel := context.WithCancel(context.Background())
	d.cancel = cancel

	leasesChan := make(chan []*dnsmasq.Lease)

	go func() {
		if err := dnsmasq.WatchLeases(ctx, d.LeasesFilePath, leasesChan); err != nil {
			acc.AddError(err)
		}
	}()

	d.wg.Add(1)
	go func() {
		defer d.wg.Done()

		for leases := range leasesChan {
			d.replaceLeases(leases)
		}
	}()

	d.Log.Infof("Watching DNSMasq leases on %s", d.LeasesFilePath)
	return nil
}

func (d *DHCP) replaceLeases(leases []*dnsmasq.Lease) {
	d.mtx.Lock()
	defer d.mtx.Unlock()

	clear(d.leases)

	for _, lease := range leases {
		d.leases[lease.IPAddr.String()] = lease
	}
}

func (d *DHCP) Add(metric telegraf.Metric, acc telegraf.Accumulator) error {
	for _, f := range d.IPFields {
		if val, ok := metric.GetField(f); ok {
			valStr, okStr := val.(string)
			if !okStr {
				return errors.New("not a string value in field: " + f)
			}
			d.process(valStr, f, func(k, v string) { metric.AddField(k, v) })
		}
	}
	for _, t := range d.IPTags {
		if val, ok := metric.GetTag(t); ok {
			d.process(val, t, func(k, v string) { metric.AddTag(k, v) })
		}
	}

	acc.AddMetric(metric)
	return nil
}

func (d *DHCP) process(ip, prefix string, fn func(string, string)) {
	d.mtx.RLock()
	defer d.mtx.RUnlock()

	if lease, ok := d.leases[ip]; ok {
		fn(prefix+"_host_name", lease.Hostname)
		fn(prefix+"_mac_addr", lease.MacAddr.String())
	}
}

func (d *DHCP) Stop() {
	d.cancel()
	d.wg.Wait()

	d.Log.Debug("Exiting DNSMasq DHCP processor")
}

func init() {
	processors.AddStreaming("dhcp", func() telegraf.StreamingProcessor { return &DHCP{} })
}
