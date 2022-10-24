package transfer

import (
	"encoding/json"
	"fmt"
)

func DataToBytes(data interface{}) []byte {
	// check if data is nil
	if data == nil {
		return nil
	}

	// check type of data
	switch dataTyped := data.(type) {
	case map[string]interface{}, []interface{}:
		dataBytes, err := json.Marshal(dataTyped)
		if err != nil {
			return []byte(fmt.Sprint(dataTyped))
		}

		return dataBytes
	case []byte:
		return dataTyped
	case string:
		return []byte(dataTyped)
	default:
		return []byte(fmt.Sprint(dataTyped))
	}
}
