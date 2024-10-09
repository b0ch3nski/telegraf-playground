//go:build !custom || inputs || inputs.dnsmasq

package all

import _ "github.com/influxdata/telegraf/plugins/inputs/dnsmasq" // register plugin
