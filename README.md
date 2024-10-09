# telegraf-playground

A bunch of scripts to produce and test customized [Telegraf][telegraf] build tailored for my needs (mostly related to
running on OpenWRT based routers and Raspberry Pi). Also consists of my out of the tree plugins for scraping data from
[DNSMasq][dnsmasq] and [SpaceX Starlink][starlink] dish.

## OpenWRT

In my network setup, I'm using a bunch of [Xiaomi Mi WiFi R3G][mir3g] routers which have 256 MB RAM, 128 MB flash and
MediaTek MT7621 CPU (MIPS little endian architecture).

There is a dedicated Make target for building **OpenWRT** package for MIR3G (using Docker): `make build-mir3g` \
Resulting package will require around ~10 MB of disk space on a router.

For a build procedure details, see included [Makefile](Makefile).

### install

```sh
# add support for SCP and optionally `softflowd` for collecting NetFlow data
$ opkg update
$ opkg install openssh-sftp-server softflowd

# transfer ipk file to router using SCP and install it
$ opkg install /tmp/telegraf_<date>_<arch>.ipk

# package provides init.d script to manage service
$ PROCD_DEBUG=1 /etc/init.d/telegraf {start,stop,status,enable}

# service logs should be visible in logd
$ logread -f
```

### example

See [telegraf.conf](telegraf.conf) and [telegraf-openwrt.conf](telegraf-openwrt.conf).


[telegraf]: https://github.com/influxdata/telegraf
[dnsmasq]: https://github.com/b0ch3nski/go-dnsmasq-utils
[starlink]: https://github.com/b0ch3nski/go-starlink
[mir3g]: https://openwrt.org/toh/xiaomi/mir3g
