package server

// Connection is the interface that both WSS and UDP connections implement
type Connection interface {
	Send(data []byte) error
}

// Server is the interface that both WSS and UDP servers implement
type Server interface {
	Start() error
	SetOnDisconnect(callback func(conn Connection))
}
