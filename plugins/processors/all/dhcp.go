//go:build !custom || processors || processors.dhcp

package all

import _ "github.com/influxdata/telegraf/plugins/processors/dhcp" // register plugin
