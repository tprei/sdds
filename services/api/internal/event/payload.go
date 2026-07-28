package event

import "encoding/json"

const (
	payloadMaxBytes = 8 * 1024
	resultMaxCount  = 50
)

func marshalPayload(payload Payload) ([]byte, error) {
	return json.Marshal(payload)
}
