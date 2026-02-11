package protocol

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
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

func Decode(r io.Reader) (*Message, error) {
	// 1️⃣ 读固定头
	fixedHeader := make([]byte, 10)
	if _, err := io.ReadFull(r, fixedHeader); err != nil {
		return nil, err
	}

	if binary.BigEndian.Uint16(fixedHeader[0:2]) != Magic {
		return nil, fmt.Errorf("protocol: invalid magic")
	}

	headerLen := binary.BigEndian.Uint32(fixedHeader[2:6])
	bodyLen := binary.BigEndian.Uint32(fixedHeader[6:10])

	// 2️⃣ 读 header
	headerBytes := make([]byte, headerLen)
	if _, err := io.ReadFull(r, headerBytes); err != nil {
		return nil, err
	}

	// 3️⃣ 读 body
	bodyBytes := make([]byte, bodyLen)
	if _, err := io.ReadFull(r, bodyBytes); err != nil {
		return nil, err
	}

	var header Header
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return nil, err
	}

	return &Message{
		Header: &header,
		Body:   bodyBytes,
	}, nil
}

// DecodeHeaderLen 从字节切片解析 headerLen
func DecodeHeaderLen(data []byte) uint32 {
	return binary.BigEndian.Uint32(data)
}

// DecodeBodyLen 从字节切片解析 bodyLen
func DecodeBodyLen(data []byte) uint32 {
	return binary.BigEndian.Uint32(data)
}

// DecodeBytes 从字节数组解码完整的 Message（用于粘包处理）
func DecodeBytes(data []byte) (*Message, error) {
	if len(data) < 10 {
		return nil, fmt.Errorf("data too short for header")
	}

	if binary.BigEndian.Uint16(data[0:2]) != Magic {
		return nil, fmt.Errorf("protocol: invalid magic")
	}

	headerLen := binary.BigEndian.Uint32(data[2:6])
	bodyLen := binary.BigEndian.Uint32(data[6:10])

	totalLen := 10 + int(headerLen) + int(bodyLen)
	if len(data) < totalLen {
		return nil, fmt.Errorf("data incomplete, expected %d, got %d", totalLen, len(data))
	}

	headerBytes := data[10 : 10+headerLen]
	bodyBytes := data[10+headerLen:]

	var header Header
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return nil, err
	}

	return &Message{
		Header: &header,
		Body:   bodyBytes,
	}, nil
}
