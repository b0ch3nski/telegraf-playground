//go:build !custom || inputs || inputs.starlink

package all

import _ "github.com/influxdata/telegraf/plugins/inputs/starlink" // register plugin
