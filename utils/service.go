package utils

import (
	"context"
	"fmt"
	"time"

	nfqueue "github.com/florianl/go-nfqueue"
	"github.com/mdlayher/netlink"
)

type NFQueueService struct {
	q              *nfqueue.Nfqueue
	ctx            context.Context
	cancel         context.CancelFunc
	defaultVerdict int
	enableENOBUFS  bool
	nfConfig       *nfqueue.Config
	errHandler     func(error) error
}

type NFQueueServiceOpt func(*NFQueueService)

var ErrPacketVerdictTwice = fmt.Errorf("packet cannot be verdicted twice")

func NewNFQueueService(number uint16, handler func(*NFQueuePacket) error, opts ...NFQueueServiceOpt) (*NFQueueService, error) {
	ctx, cancel := context.WithCancel(context.Background())
	svc := &NFQueueService{
		ctx:            ctx,
		cancel:         cancel,
		defaultVerdict: nfqueue.NfDrop,
		enableENOBUFS:  false,
		nfConfig: &nfqueue.Config{
			NfQueue:      number,
			MaxPacketLen: 0xFFFF,
			MaxQueueLen:  0xFF,
			Copymode:     nfqueue.NfQnlCopyPacket,
			WriteTimeout: 15 * time.Millisecond,
		},
	}
	for _, opt := range opts {
		opt(svc)
	}

	q, err := nfqueue.Open(svc.nfConfig)
	if err != nil {
		return nil, err
	}
	svc.q = q

	if !svc.enableENOBUFS {
		q.Con.SetOption(netlink.NoENOBUFS, true)
	}

	realCallback := func(attr nfqueue.Attribute) int {
		pkt := &NFQueuePacket{
			q:    q,
			attr: attr,
		}
		if e := handler(pkt); e != nil {
			fmt.Printf("nfqueue service callback error: %v\n", e)
			return 1
		}
		if !pkt.hasVerdict {
			pkt.q.SetVerdict(*pkt.attr.PacketID, svc.defaultVerdict)
		}
		return 0
	}

	errfn := func(e error) int {
		if svc.errHandler != nil {
			if svc.errHandler(e) != nil {
				return 1
			}
		}

		return 0
	}

	err = q.RegisterWithErrorFunc(svc.ctx, realCallback, errfn)
	if err != nil {
		q.Close()
		return nil, err
	}

	return svc, nil
}

func WithDefaultVerdict(verdict int) NFQueueServiceOpt {
	return func(s *NFQueueService) {
		s.defaultVerdict = verdict
	}
}

func WithMaxQueueLen(length uint32) NFQueueServiceOpt {
	return func(s *NFQueueService) {
		s.nfConfig.MaxQueueLen = length
	}
}

func WithMaxPacketLen(length uint32) NFQueueServiceOpt {
	return func(s *NFQueueService) {
		s.nfConfig.MaxPacketLen = length
	}
}

func WithErrorFunc(fn func(error) error) NFQueueServiceOpt {
	return func(s *NFQueueService) {
		s.errHandler = fn
	}
}

func WithENOBUFS() NFQueueServiceOpt {
	return func(s *NFQueueService) {
		s.enableENOBUFS = true
	}
}

func (s *NFQueueService) Close() {
	fmt.Println("closing nfqueue service...")
	s.cancel()
	s.q.Close()
}
