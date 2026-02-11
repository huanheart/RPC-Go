package protocol

import (
	"encoding/binary"
	"encoding/json"
)

const Magic uint16 = 0x1234

type Message struct {
	Header *Header
	Body   []byte
}

func Encode(msg *Message) ([]byte, error) {
	hb, _ := json.Marshal(msg.Header)
	headerLen := uint32(len(hb))
	bodyLen := uint32(len(msg.Body))

	total := 2 + 4 + 4 + headerLen + bodyLen
	buf := make([]byte, total)

	binary.BigEndian.PutUint16(buf[0:2], Magic)
	binary.BigEndian.PutUint32(buf[2:6], headerLen)
	binary.BigEndian.PutUint32(buf[6:10], bodyLen)

	copy(buf[10:], hb)
	copy(buf[10+headerLen:], msg.Body)
	return buf, nil
}
