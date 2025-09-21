package monitor

import (
	"fmt"
	"log"
	"slices"
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

		// List devices 
		devices, err := mon.st.GetDevices()
		if err != nil {
			log.Printf("ERROR: could not list devices: %v", err)
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

		var i int = 0
		for _, d := range devices.Devices {
			d.UpdateStatus()
			for _, comp := range d.Components {
				for _, cap := range comp.Capabilities {
					for _, m := range mon.metrics {
						if m == cap.Id {
							for key, val := range cap.Status {
								i++

								if slices.Contains(mon.ignore, key) {
									log.Printf("       INFO:   %s %s is in ignore list, skipping", cap.Id, key)
									continue
								}

								fields := make(map[string]interface{})

								var deviceId, devLabel string

								if comp.Id != "main" {
									deviceId = d.DeviceId.String() + comp.Id
									devLabel = d.Label + " " + comp.Id
								} else {
									deviceId = d.DeviceId.String()
									devLabel = d.Label
								}

								// In the groovy logger, 'value' is sent as a string unless it's a number
								// Then there is a conversion done to some strings to create a binary
								// and store it in valueBinary
								// So since this is intended as a drop-in replacement, we'll do that
								// but still retain the valueFloat from the original package
								// though probably it's not needed.

								if val.Value == nil {
									log.Printf("       WARNING:  %s %s %s got nil metric value: %v", devLabel, cap.Id, key, err)
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
								if (time.Until(val.Timestamp).Hours() > 24 || d.Health.State != "ONLINE") && cap.Id == "battery" {
									log.Printf("       WARNING: Likely dead battery on %s", devLabel)
									fields["value"] = 0.0
									convValue = 0.0
									fields["valueFloat"] = convValue
									binaryValue = 0
									fields["valueBinary"] = binaryValue
									val.Value = 0.0
									// Set timestamp to 10 seconds later so the state change to offline will trigger a record update
									val.Timestamp = val.Timestamp.Add(10 * time.Second)
									if d.Health.State != "ONLINE" {
										val.Timestamp = d.Health.LastUpdated
									}
								} else {

									// Get converted float value
									convValue, err = val.FloatValue(key)
									if err != nil {
										log.Printf("       ERROR: could not convert %-22s %-27s to number %v", devLabel, cap.Id, err)
										continue
									} else {
										fields["valueFloat"] = convValue
									}

									// Get converted binary value
									binaryValue, err = val.BinaryValue(key)
									if err != nil {
										log.Printf("     ERROR: could not convert %-22s %-27s to binary %v", devLabel, cap.Id, err)
										continue
									} else {
										fields["valueBinary"] = binaryValue
									}
								}

								if lastUpdate[deviceId+key].Equal(val.Timestamp) {
									if time.Now().Minute() < (mon.interval / 60) {
										action := "HOURLY "
										val.Timestamp = time.Now()
										log.Printf("%3d: %-23s %-6s %-27ss: %s time: %14s value: %12s%1s number: %4.1f binary: %2d", i, devLabel, comp.Id, cap.Id, action, val.Timestamp.Format("01-02 15:04:05"), fmt.Sprintf("%v", val.Value), val.Unit, convValue, binaryValue)
									} else {
										action := "SKIPPED"
										log.Printf("%3d: %-23s %-6s %-27s: %s time: %14s value: %12s%1s number: %4.1f binary: %2d", i, devLabel, comp.Id, cap.Id, action, val.Timestamp.Format("01-02 15:04:05"), fmt.Sprintf("%v", val.Value), val.Unit, convValue, binaryValue)
										newLastUpdate[deviceId+key] = val.Timestamp
										continue
									}
								} else {
									action := "CHANGED"
									log.Printf("%3d: %-23s %-6s %-27s: %s time: %14s value: %12s%1s number: %4.1f binary: %2d", i, devLabel, comp.Id, cap.Id, action, val.Timestamp.Format("01-02 15:04:05"), fmt.Sprintf("%v", val.Value), val.Unit, convValue, binaryValue)
								}

								// Create point
								tags := map[string]string{
									"deviceId":   deviceId,
									"deviceName": devLabel,
									// "groupId":    dev.Device.groupId,
									// "groupName":  dev.Device.groupName,
									// "hubId":      dev.Device.hubId,
									// "hubName":    dev.Device.hubName,
									"component":  comp.Id,
									"capability": cap.Id,
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
					}
				}
			}
			if !(lastUpdate[d.DeviceId.String()+"health"].Equal(d.Health.LastUpdated)) {
				// Add a point for the health status
				tags := map[string]string{
					"deviceId":   d.DeviceId.String(),
					"deviceName": d.Label,
					"component":  "health",
					"capability": "health",
					"source":     "docker",
				}
				fields := make(map[string]interface{})
				fields["value"] = d.Health.State
				// Get converted float value
				convValue, err := mon.st.ConvertValueToFloat("health", d.Health.State)
				if err != nil {
					log.Printf("       ERROR: could not convert %-22s %-27s to number %v", d.Label, "health", err)
					continue
				} else {
					fields["valueFloat"] = convValue
				}

				// Get converted binary value
				binaryValue, err := mon.st.ConvertValueToBinary("health", d.Health.State)
				if err != nil {
					log.Printf("     ERROR: could not convert %-22s %-27s to binary %v", d.Label, "health", err)
					continue
				} else {
					fields["valueBinary"] = binaryValue
				}
				point, err := client.NewPoint(
					"health",
					tags,
					fields,
					d.Health.LastUpdated,
				)
				if err != nil {
					log.Printf("could not create point: %v", err)
					time.Sleep(time.Duration(mon.interval) * time.Second)
					continue
				}
				bp.AddPoint(point)
			}
			newLastUpdate[d.DeviceId.String()+"health"] = d.Health.LastUpdated
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
		log.Printf("Version 1.0.5")

	}
}
