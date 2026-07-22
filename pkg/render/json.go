package render

import "encoding/json"

func JSON(report Report) ([]byte, error) {
	return json.MarshalIndent(report, "", "  ")
}
