package processor

import (
	"fmt"
	"seras-protocol/internal/tun"
	"seras-protocol/pkg/taiga/msg"
)

type Processor struct {
	tun *tun.TUN
}

func NewProcessor(t *tun.TUN) *Processor {
	return &Processor{tun: t}
}

func (p *Processor) Process(data *msg.CookedMsg) error {
	if data.Body.NextHop == nil {
		// Final destination - write to TUN
		n, err := p.tun.Write(data.Body.Data)
		if err != nil {
			return fmt.Errorf("failed to write to TUN: %w", err)
		}
		if n != len(data.Body.Data) {
			return fmt.Errorf("incomplete write: %d/%d bytes", n, len(data.Body.Data))
		}
	}
	// TODO: handle multi-hop routing when NextHop != nil
	return nil
}
