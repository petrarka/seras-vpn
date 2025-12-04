package helpers

import (
	"fmt"
	"seras-protocol/pkg/taiga/msg"
)

func ConvertStringToConnType(s string) (msg.Protocol, error) {
	switch s {
	case "wg":
		return msg.Wg, nil
	case "wss":
		return msg.Wss, nil
	default:
		return "", fmt.Errorf("invalid connection type")
	}
}
