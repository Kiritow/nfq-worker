package utils

import (
	"fmt"

	nfqueue "github.com/florianl/go-nfqueue"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

type NFQueuePacket struct {
	q          *nfqueue.Nfqueue
	attr       nfqueue.Attribute
	hasVerdict bool
}

func (p *NFQueuePacket) Accept() error {
	if p.hasVerdict {
		return ErrPacketVerdictTwice
	}
	if err := p.q.SetVerdict(*p.attr.PacketID, nfqueue.NfAccept); err != nil {
		return err
	}
	p.hasVerdict = true
	return nil
}

func (p *NFQueuePacket) Drop() error {
	if p.hasVerdict {
		return ErrPacketVerdictTwice
	}
	if err := p.q.SetVerdict(*p.attr.PacketID, nfqueue.NfDrop); err != nil {
		return err
	}
	p.hasVerdict = true
	return nil
}

func (p *NFQueuePacket) AcceptWithPayload(payload []byte) error {
	if p.hasVerdict {
		return ErrPacketVerdictTwice
	}
	if err := p.q.SetVerdictModPacket(*p.attr.PacketID, nfqueue.NfAccept, payload); err != nil {
		return err
	}
	p.hasVerdict = true
	return nil
}

func (p *NFQueuePacket) AcceptWithPacket(pkt gopacket.Packet) error {
	bytes, err := serializePacket(pkt)
	if err != nil {
		return err
	}
	return p.AcceptWithPayload(bytes)
}

func (p *NFQueuePacket) Payload() []byte {
	return *p.attr.Payload
}

func (p *NFQueuePacket) ToIPv4Packet() gopacket.Packet {
	return gopacket.NewPacket(p.Payload(), layers.IPProtocolIPv4, gopacket.Default)
}

func (p *NFQueuePacket) ToIPv6Packet() gopacket.Packet {
	return gopacket.NewPacket(p.Payload(), layers.IPProtocolIPv6, gopacket.Default)
}

func serializePacket(packet gopacket.Packet) ([]byte, error) {
	type setNetworkLayerForChecksum interface {
		SetNetworkLayerForChecksum(gopacket.NetworkLayer) error
	}

	var l gopacket.NetworkLayer
	for _, layer := range packet.Layers() {
		if n, ok := layer.(gopacket.NetworkLayer); ok {
			l = n
		}
		if s, ok := layer.(setNetworkLayerForChecksum); ok {
			if l == nil {
				return nil, fmt.Errorf("no enclosing network layer found before: %v", s)
			}
			if err := s.SetNetworkLayerForChecksum(l); err != nil {
				return nil, fmt.Errorf("failed to set network layer(%v) on layer(%v): %v", l, s, err)
			}
		}
	}

	buffer := gopacket.NewSerializeBuffer()
	err := gopacket.SerializePacket(buffer, gopacket.SerializeOptions{
		ComputeChecksums: true,
		FixLengths:       true,
	}, packet)
	if err != nil {
		return nil, err
	}

	return buffer.Bytes(), nil
}
