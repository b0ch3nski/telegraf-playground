# Project settings
APP_NAME := telegraf
APP_VERSION := $(shell date -u '+%Y%m%d%H%M')

# Build settings
BIN_NAME := $(APP_NAME)-$(APP_VERSION)
IMAGE_NAME := b0ch3nski/$(APP_NAME)
IMAGE_PLATFORMS ?= linux/amd64,linux/arm64,linux/arm/v7

TELEGRAF_TAGS := custom,inputs.cpu,inputs.disk,inputs.diskio,inputs.dnsmasq,inputs.dns_query,inputs.docker,inputs.http,inputs.http_listener_v2,inputs.iptables,inputs.kernel_vmstat,inputs.linux_cpu,inputs.mem,inputs.mqtt_consumer,inputs.net,inputs.netflow,inputs.netstat,inputs.processes,inputs.procstat,inputs.starlink,inputs.syslog,inputs.system,inputs.tail,inputs.temp,inputs.wireless,outputs.file,outputs.http,outputs.influxdb_v2,outputs.loki,outputs.mqtt,outputs.prometheus_client,parsers.influx,parsers.json,parsers.json_v2,parsers.prometheusremotewrite,processors.dhcp,serializers.csv,serializers.influx,serializers.json,serializers.prometheus,serializers.prometheusremotewrite
ifeq ($(TARGET),openwrt)
TELEGRAF_TAGS := custom,inputs.cpu,inputs.disk,inputs.dnsmasq,inputs.http,inputs.mem,inputs.mqtt_consumer,inputs.net,inputs.netstat,inputs.processes,inputs.procstat,inputs.syslog,inputs.system,outputs.file,outputs.mqtt,parsers.json,parsers.json_v2,processors.dhcp,serializers.influx,serializers.json
endif

# Versions
GOLANG_VERSION := 1.23.5
ALPINE_VERSION := 3.21
TELEGRAF_VERSION := $(or $(shell awk '/telegraf /{print $$2}' go.mod),master)
GRAFANA_VERSION := 11.5.0
PROMETHEUS_VERSION := 3.1.0
LOKI_VERSION := 3.3.2
INFLUXDB_VERSION := 2.7.11
MOSQUITTO_VERSION := 2.0.20

# Make settings
.ONESHELL:

# Make goals
run: ## Runs testing environment
	GRAFANA_VERSION="$(GRAFANA_VERSION)" \
	PROMETHEUS_VERSION="$(PROMETHEUS_VERSION)" \
	LOKI_VERSION="$(LOKI_VERSION)" \
	INFLUXDB_VERSION="$(INFLUXDB_VERSION)" \
	MOSQUITTO_VERSION="$(MOSQUITTO_VERSION)" \
	APP_VERSION="$(APP_VERSION)" \
	GOLANG_VERSION="$(GOLANG_VERSION)" \
	ALPINE_VERSION="$(ALPINE_VERSION)" \
	TELEGRAF_VERSION="$(TELEGRAF_VERSION)" \
	TELEGRAF_TAGS="$(TELEGRAF_TAGS)" \
	SOFTFLOWD_INTERFACE="$(shell ip -o -4 route show to default | awk 'NR==1{print $$5}')" \
	docker compose --file docker-compose.yaml up $(SERVICES)

build-linux-%: ## Builds app binary for specified architecture running Linux
	GOARCH=$(*) GOOS=linux \
	GOLANG_VERSION="$(GOLANG_VERSION)" \
	ALPINE_VERSION="$(ALPINE_VERSION)" \
	TELEGRAF_VERSION="$(TELEGRAF_VERSION)" \
	TELEGRAF_TAGS="$(TELEGRAF_TAGS)" \
	BIN_NAME="$(BIN_NAME)" \
	BIN_OWNER="$(shell id --user):$(shell id --group)" \
	docker compose --file docker-compose-build.yaml up build-bin --force-recreate

clean: ## Removes Docker resources created by this project
	docker compose --file docker-compose.yaml down --volumes
	docker compose --file docker-compose-build.yaml down --volumes

package-ipk-%: ## Create IPK package for specified architecture running OpenWRT
	TMPDIR=$$(mktemp --directory)
	trap "rm -rv $${TMPDIR}" EXIT

	mkdir -p $${TMPDIR}/control
	cat <<- EOF > $${TMPDIR}/control/control
		Package: $(APP_NAME)
		Version: $(APP_VERSION)
		Architecture: $(*)
	EOF
	tar --numeric-owner --group=0 --owner=0 -czf $${TMPDIR}/control.tar.gz --directory=$${TMPDIR}/control .

	mkdir -p $${TMPDIR}/data/etc/init.d
	cat <<- EOF > "$${TMPDIR}/data/etc/init.d/$(APP_NAME)"
		#!/bin/sh /etc/rc.common
		USE_PROCD=1
		START=95
		STOP=01

		go_mem_limit() {
			awk -v frac="\$${1:-0.5}" '/^MemTotal/ {print int(\$$2*frac)"KiB"}' /proc/meminfo
		}

		start_service() {
			procd_open_instance
			procd_set_param limits memlock="32768 65536"
			procd_set_param env "GOMEMLIMIT=\$$(go_mem_limit 0.2)"
			procd_set_param command /usr/bin/$(APP_NAME) --config=/etc/telegraf.conf --watch-config=notify

			procd_set_param stdout 1
			procd_set_param stderr 1
			procd_set_param term_timeout 10
			procd_set_param respawn
			procd_close_instance
		}
	EOF
	chmod +x "$${TMPDIR}/data/etc/init.d/$(APP_NAME)"

	cp "telegraf-openwrt.conf" "$${TMPDIR}/data/etc/telegraf.conf"

	mkdir -p $${TMPDIR}/data/usr/bin
	cp "bin/$(BIN_NAME)" "$${TMPDIR}/data/usr/bin/$(APP_NAME)"

	tar --numeric-owner --group=0 --owner=0 -czf $${TMPDIR}/data.tar.gz --directory=$${TMPDIR}/data .

	echo "2.0" > $${TMPDIR}/debian-binary
	tar --numeric-owner --group=0 --owner=0 -czf "bin/$(APP_NAME)_$(APP_VERSION)_$(*).ipk" --directory=$${TMPDIR} ./debian-binary ./data.tar.gz ./control.tar.gz

build-linux: build-linux-amd64 ## Builds app binary for Linux on x86-64 architecture

build-mir3g: BIN_NAME := $(BIN_NAME)-mir3g
build-mir3g: build-linux-mipsle package-ipk-mipsel_24kc ## Builds package for Xiaomi Mi WiFi R3G running OpenWRT

build-rpi2: build-linux-arm ## Builds app binary for Linux on Raspberry Pi 2
build-rpi3: build-linux-arm64 ## Builds app binary for Linux on Raspberry Pi 3 and newer

build-rpi2-wrt: BIN_NAME := $(BIN_NAME)-rpi2
build-rpi2-wrt: build-rpi2 package-ipk-arm_cortex-a7_neon-vfpv4 ## Builds package for Raspberry Pi 2 running OpenWRT

build-rpi3-wrt build-rpi4-wrt: BIN_NAME := $(BIN_NAME)-rpi3
build-rpi3-wrt: build-rpi3 package-ipk-aarch64_cortex-a53 ## Builds package for Raspberry Pi 3 running OpenWRT
build-rpi4-wrt: build-rpi3 package-ipk-aarch64_cortex-a72 ## Builds package for Raspberry Pi 4 running OpenWRT

build-docker: ## Builds multi-arch Docker image
	docker buildx build \
	--pull \
	--push \
	--platform="$(IMAGE_PLATFORMS)" \
	--build-arg GOLANG_VERSION="$(GOLANG_VERSION)" \
	--build-arg ALPINE_VERSION="$(ALPINE_VERSION)" \
	--build-arg TELEGRAF_VERSION="$(TELEGRAF_VERSION)" \
	--build-arg TELEGRAF_TAGS="$(TELEGRAF_TAGS)" \
	--label="org.opencontainers.image.title=$(APP_NAME)" \
	--label="org.opencontainers.image.version=$(APP_VERSION)" \
	--label="org.opencontainers.image.revision=$(shell git log -1 --format=%H)" \
	--label="org.opencontainers.image.created=$(shell date --iso-8601=seconds)" \
	--tag="$(IMAGE_NAME):$(APP_VERSION)" \
	--tag="$(IMAGE_NAME):latest" \
	.
