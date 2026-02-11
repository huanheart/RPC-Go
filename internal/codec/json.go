package codec

import "encoding/json"

const JSON Type = 1

type JSONCodec struct{}

func (j *JSONCodec) Marshal(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

func (j *JSONCodec) Unmarshal(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}

func init() {
	Register(JSON, func() Codec {
		return &JSONCodec{}
	})
}
