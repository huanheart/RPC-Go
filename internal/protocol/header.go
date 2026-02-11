package protocol

// CodecType 编解码器类型
type CodecType byte

const (
	CodecTypeJSON CodecType = iota + 1
	CodecTypeProto
)

// CompressionType 压缩类型
type CompressionType byte

const (
	CompressionNone CompressionType = iota
	CompressionGzip
)

type Header struct {
	RequestID   uint64
	ServiceName string
	MethodName  string
	Error       string
	CodecType   CodecType
	Compression CompressionType
}
