package monitor

import (
	"fmt"
	"log"
	"slices"
	"strings"
	"time"

	retry "github.com/avast/retry-go"

	"github.com/gnieboer/smartthings-influx/pkg/smartthings"
	"github.com/influxdata/influxdb/client/v2"
)

type Monitor struct {
	st       *smartthings.Client
	influx   client.HTTPClient
	database string
	metrics  []string
	interval int
	ignore   []string
}

func New(st *smartthings.Client, influx client.HTTPClient, database string, metrics []string, interval int, ignore []string) *Monitor {
	return &Monitor{st: st, influx: influx, database: database, metrics: metrics, interval: interval, ignore: ignore}
}

func (mon Monitor) Run() error {
	duration := time.Duration(0) // Cheap trick not to sleep at the first round

	lastUpdate := make(map[string]time.Time)

	for {
		// Cheap trick not to sleep at the first round
		time.Sleep(duration)
		duration = time.Duration(mon.interval) * time.Second
		// End of cheap trick

		// Using another map so we update the timestamp only when the record is serialized
		newLastUpdate := make(map[string]time.Time)

		// List devices with metrics
		devices, err := mon.st.DevicesWithCapabilities(mon.metrics)
		if err != nil {
			log.Printf("ERROR: could not list devices: %v", err)
			continue
		}
		if len(devices.Items) == 0 {
			log.Printf("ERROR: no devices with any of the metrics: (%s)", strings.Join(mon.metrics, ", "))
			continue
		}

		bp, err := client.NewBatchPoints(client.BatchPointsConfig{
			Database:  mon.database,
			Precision: "s",
		})
		if err != nil {
			log.Printf("ERROR: could not create batch points for influx: %v", err)
			time.Sleep(time.Duration(mon.interval) * time.Second)
			continue
		}

		for i, dev := range devices.Items {
			// log.Printf("%d: Monitoring '%s' from device '%s' (%s)", i, dev.Capability.Id, devLabel, dev.Device.DeviceId)
			// Get measurement
			status, err := dev.Status()
			if err != nil {
				log.Printf("ERROR: could not get metric status: %v", err)
				continue
			}

			for key, val := range status {

				if slices.Contains(mon.ignore, key) {
					log.Printf("%3d: INFO:   %-22s %-27s %s is in ignore list, skipping", i, dev.Device.Label, dev.Capability.Id, key)
					continue
				}

				fields := make(map[string]interface{})

				var deviceId, devLabel string

				if dev.Component.Id != "main" {
					deviceId = dev.Device.DeviceId.String() + dev.Component.Id
					devLabel = dev.Device.Label + " " + dev.Component.Id
				} else {
					deviceId = dev.Device.DeviceId.String()
					devLabel = dev.Device.Label
				}

				// In the groovy logger, 'value' is sent as a string unless it's a number
				// Then there is a conversion done to some strings to create a binary
				// and store it in valueBinary
				// So since this is intended as a drop-in replacement, we'll do that
				// but still retain the valueFloat from the original package
				// though probably it's not needed.

				// save the health state as a value
				fields["health"] = dev.Device.Health.State

				if val.Value == nil {
					log.Printf("%3d: WARNING:  %-22s %-27s %s got nil metric value: %v", i, devLabel, dev.Capability.Id, key, err)
					continue
				} else {
					_, ok := val.Value.(float64)
					if ok {
						fields["value"] = val.Value.(float64)
					} else {
						fields["value"] = val.Value
					}
				}

				// Special routine that will set battery level to 0% if we haven't gotten an update on battery levels for 24 hours
				// Or if the device health is offline or unhealthy
				var convValue float64
				var binaryValue int8
				if (time.Until(val.Timestamp).Hours() > 24 || dev.Device.Health.State != "ONLINE") && dev.Capability.Id == "battery" {
					log.Printf("WARNING: Likely dead battery on %s", devLabel)
					fields["value"] = 0.0
					convValue = 0.0
					fields["valueFloat"] = convValue
					binaryValue = 0
					fields["valueBinary"] = binaryValue
					val.Value = 0.0
					// Set timestamp to 10 seconds later so the state change to offline will trigger a record update
					val.Timestamp = val.Timestamp.Add(10 * time.Second)
					if dev.Device.Health.State != "ONLINE" {
						val.Timestamp = dev.Device.Health.LastUpdated
					}
				} else {

					// Get converted float value
					convValue, err := val.FloatValue(key)
					if err != nil {
						log.Printf("%3d: ERROR: could not convert %-22s %-27s to number %v", i, devLabel, dev.Capability.Id, err)
						continue
					} else {
						fields["valueFloat"] = convValue
					}

					// Get converted binary value
					binaryValue, err := val.BinaryValue(key)
					if err != nil {
						log.Printf("%3d: ERROR: could not convert %-22s %-27s to binary %v", i, devLabel, dev.Capability.Id, err)
						continue
					} else {
						fields["valueBinary"] = binaryValue
					}
				}
				// log.Printf("Key is %s value %v number value %f binary value %d", key, val, convValue, binaryValue)

				if lastUpdate[deviceId+key].Equal(val.Timestamp) {
					if time.Now().Minute() < (mon.interval / 60) {
						action := "HOURLY "
						val.Timestamp = time.Now()
						log.Printf("%3d: %-22s %-27s %s: %s time: %33s value: %12s%1s number: %4.1f binary: %2d", i, devLabel, dev.Capability.Id, dev.Component.Id, action, val.Timestamp, fmt.Sprintf("%v", val.Value), val.Unit, convValue, binaryValue)
					} else {
						action := "SKIPPED"
						log.Printf("%3d: %-22s %-27s %s: %s time: %33s value: %12s%1s number: %4.1f binary: %2d", i, devLabel, dev.Capability.Id, dev.Component.Id, action, val.Timestamp, fmt.Sprintf("%v", val.Value), val.Unit, convValue, binaryValue)
						newLastUpdate[deviceId+key] = val.Timestamp
						continue
					}
				} else {
					action := "CHANGED"
					log.Printf("%3d: %-22s %-27s %s: %s time: %33s value: %12s%1s number: %4.1f binary: %2d", i, devLabel, dev.Capability.Id, dev.Component.Id, action, val.Timestamp, fmt.Sprintf("%v", val.Value), val.Unit, convValue, binaryValue)
				}

				// Data format from Codesaur's groovy version
				// def data = "${measurement},deviceId=${deviceId},deviceName=${deviceName},groupId=${groupId},groupName=${groupName},hubId=${hubId},hubName=${hubName},locationId=${locationId},locationName=${locationName}"
				// then it adds value data to the string, both value and valueBinary for those things that can be converted
				// the 3-axis sensors are unsupported

				// Create point

				tags := map[string]string{
					"deviceId":   deviceId,
					"deviceName": devLabel,
					// "groupId":    dev.Device.groupId,
					// "groupName":  dev.Device.groupName,
					// "hubId":      dev.Device.hubId,
					// "hubName":    dev.Device.hubName,
					"component":  dev.Component.Id,
					"capability": dev.Capability.Id,
					"health":     dev.Device.Health.State,
					"unit":       val.Unit,
					"source":     "docker",
				}

				point, err := client.NewPoint(
					key,
					tags,
					fields,
					val.Timestamp,
				)
				if err != nil {
					log.Printf("could not create point: %v", err)
					time.Sleep(time.Duration(mon.interval) * time.Second)
					continue
				}

				bp.AddPoint(point)
				newLastUpdate[deviceId+key] = val.Timestamp
			}
		}

		if len(bp.Points()) > 0 {
			// Record points
			err := retry.Do(func() error {
				result := mon.influx.Write(bp)
				if result != nil {
					log.Printf("Error writing point: %v", result)
				}
				return result
			})
			if err != nil {
				log.Printf("Error writing point: %v", err)
			} else {
				log.Printf("Record saved %v", bp)
				lastUpdate = newLastUpdate
			}
		} else {
			log.Printf("No new read since last update")
		}
		// Yes this is a bit of a hack, but it gives a quick reference to the version running
		log.Printf("Version 1.0.3")

	}
}
