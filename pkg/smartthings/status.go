package smartthings
import (
	"time"
)

type DeviceStatus struct {
	ComponentStatus map[string]ComponentStatus `json:"components"`
}

type ComponentStatus map[string]CapabilityStatus 

type CapabilityStatus map[string]AttributeState 

type AttributeState struct {
	Timestamp time.Time `json:"timestamp"`
	Unit      string    `json:"unit"`
	Value     any       `json:"value"`
}

func (status AttributeState) FloatValue(metric string) (float64, error) {
	return cli.ConvertValueToFloat(metric, status.Value)
}

func (status AttributeState) BinaryValue(metric string) (int8, error) {
	return cli.ConvertValueToBinary(metric, status.Value)
}