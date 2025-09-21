package smartthings

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

const api = "https://api.smartthings.com/v1"

var cli *Client

type Client struct {
	token         string
	conversionMap map[string]map[string]float64
}

func Init(token string, conversionMap map[string]map[string]float64) *Client {
	cli = &Client{token: token, conversionMap: conversionMap}

	return cli
}

func ParseConversionMap(valuemap map[string]interface{}) (map[string]map[string]float64, error) {
	conversionMap := make(map[string]map[string]float64)
	for key, value := range valuemap {
		innermap, ok := value.(map[string]any)
		if !ok {
			return map[string]map[string]float64{}, fmt.Errorf("could not parse valuemap, it shold be in the format of metric to a map of values, got %v", value)
		}

		_, ok = conversionMap[key]
		if !ok {
			conversionMap[key] = make(map[string]float64)
		}

		for inkey, inval := range innermap {
			var inFloat float64
			_, ok := inval.(int)
			if ok {
				inFloat = float64(inval.(int))
			} else {
				inFloat, ok = inval.(float64)
				if !ok {
					return map[string]map[string]float64{}, fmt.Errorf("could not convert %v to a number for metric %s", inval, key)
				}
			}

			list := conversionMap[key]
			list[inkey] = inFloat
			conversionMap[key] = list
		}
	}
	return conversionMap, nil
}

func (c Client) ConvertValueToFloat(metric string, value any) (float64, error) {
	_, ok := value.(float64)
	if ok {
		return value.(float64), nil
	}

	_, ok = value.(string)
	if ok {
		stValue := value.(string)
		// Check if there is a map for metric
		metricMap, ok := c.conversionMap[strings.ToLower(metric)]
		if !ok {
			return 0, fmt.Errorf("there is no value map for metric '%s' and value '%s', can't convert", metric, stValue)
		}
		return metricMap[stValue], nil
	}
	return 0, nil
}

// Only convert items to binary that have an explicit mapping in the value otherwise return -1
func (c Client) ConvertValueToBinary(metric string, value any) (int8, error) {
	_, ok := value.(string)
	if ok {
		stValue := value.(string)
		// Check if there is a map for metric
		metricMap, ok := c.conversionMap[metric]
		if !ok {
			return -1, nil
		}
		intVal := int8(metricMap[stValue])
		if intVal < 2 && intVal > -1 {
			return intVal, nil
		}
		return -1, nil
	}
	return -1, nil
}

func (c Client) GetDevices() (devices DevicesList, err error) {
	data, err := c.get("/devices")
	if err != nil {
		return
	}

	err = json.Unmarshal([]byte(data), &devices)
	devices.client = &c

	// Update device health for each device
	for _, device := range devices.Devices {
		device.UpdateHealth()
	}

	return
}

func (c Client) GetDeviceStatus(deviceID uuid.UUID) (status DeviceStatus, err error) {
	url := "/devices/" + deviceID.String() + "/status"

	data, err := c.get(url)
	if err != nil {
		return
	}

	err = json.Unmarshal([]byte(data), &status)
	return 
}

func (c Client) GetDeviceHealth(deviceID uuid.UUID) (health Health, err error) {
	url := "/devices/" + deviceID.String() + "/health"

	data, err := c.get(url)
	if err != nil {
		return
	}

	err = json.Unmarshal([]byte(data), &health)
	return 
}

// Currently not used
func (c Client) GetDeviceCapabilityStatus(deviceID uuid.UUID, componentId string, capabilityId string) (status map[string]CapabilityStatus, err error) {
	url := "/devices/" + deviceID.String() + "/components/" + componentId + "/capabilities/" + capabilityId + "/status"

	data, err := c.get(url)
	if err != nil {
		return
	}

	err = json.Unmarshal(data, &status)
	if err != nil {
		return status, fmt.Errorf("could not unmarshall device capability status payload: '%s'", string(data))
	}

	return 
}

func (c Client) get(endpoint string) ([]byte, error) {
	// Create a new request using http
	req, err := http.NewRequest("GET", api+endpoint, nil)
	if err != nil {
		return []byte{}, err
	}

	// add authorization header to the req
	req.Header.Add("Authorization", "Bearer "+c.token)

	// Send req using http Client
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return []byte{}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return []byte{}, err
	} else if resp.StatusCode != 200 {
		return []byte{}, fmt.Errorf("non-200 response from SmartThings API: %d, body: %s", resp.StatusCode, string(body))
	}

	return body, nil
}

