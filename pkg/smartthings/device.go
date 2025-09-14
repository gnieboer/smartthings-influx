package smartthings

import (
	"github.com/google/uuid"
	"time"
)

type Device struct {
	DeviceId   uuid.UUID   `json:"deviceId"`
	Name       string      `json:"name"`
	Label      string      `json:"label"`
	Health     Health
	Components []Component `json:"components"`
}

type Component struct {
	Id           string       `json:"id"`
	Label        string       `json:"label"`
	Capabilities []Capability `json:"capabilities"`
}

type Health struct {
	Id		 		string 		`json:"deviceId"`
	State 			string 		`json:"state"`
	LastUpdated 	time.Time	`json:"lastUpdatedDate"`
}

type Capability struct {
	Id      string `json:"id"`
	Version int    `json:"version"`
}

type DeviceStatus map[string]interface{}

func (d *Device) Status() (DeviceStatus, error) {
	return cli.DeviceStatus(d.DeviceId)
}

func (d *Device) UpdateHealth() (Health, error) {
	h, err := cli.DeviceHealth(d.DeviceId)
	if err != nil {
		return Health{}, err
	}
	d.Health = h
	return d.Health, err
}
