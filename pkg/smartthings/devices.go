package smartthings

import (
	"time"

	"github.com/google/uuid"
)

type DevicesList struct {
	Devices []*Device `json:"items"`
	client  *Client
}

type Device struct {
	DeviceId   uuid.UUID `json:"deviceId"`
	Name       string    `json:"name"`
	Label      string    `json:"label"`
	Health     Health
	Components []Component `json:"components"`
}

type Health struct {
	DeviceId    uuid.UUID `json:"deviceId"`
	State       string    `json:"state"`
	LastUpdated time.Time `json:"lastUpdatedDate"`
}

type Component struct {
	Id           string       `json:"id"`
	Label        string       `json:"label"`
	Capabilities []Capability `json:"capabilities"`
}

type Capability struct {
	Id      string `json:"id"`
	Version int    `json:"version"`
	Status  CapabilityStatus `json:"-"` 
}

type DevicesCapabilitiesResult struct {
	Items []DeviceCapability
}

type DeviceCapability struct {
	Device     *Device
	Component  Component
	Capability Capability
	client     *Client
}

func (d *Device) UpdateStatus() (ds DeviceStatus, err error) {
	ds, err = cli.GetDeviceStatus(d.DeviceId)
	for compId, compS := range ds.ComponentStatus {
		for i, comp := range d.Components {
			if comp.Id == compId {
				for capId, capS := range compS {
					for j, cap := range d.Components[i].Capabilities {
						if cap.Id == capId {
							d.Components[i].Capabilities[j].Status = capS
						}
					}
				}
			}
		}
	}
	return
}

func (d *Device) UpdateHealth() (Health, error) {
	h, err := cli.GetDeviceHealth(d.DeviceId)
	if err != nil {
		return Health{}, err
	}
	d.Health = h
	return d.Health, err
}

