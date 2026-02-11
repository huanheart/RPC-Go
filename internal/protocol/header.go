package protocol

type Header struct {
	RequestID   uint64
	ServiceName string
	MethodName  string
	Error       string
}
