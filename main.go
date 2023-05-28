package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"

	"github.com/florianl/go-nfqueue"
	"github.com/google/gopacket/layers"
	"github.com/kiritow/nfq-worker/utils"
)

func ipToInt(ip net.IP) uint32 {
	ip = ip.To4()
	return (uint32(ip[0]) << 24) | (uint32(ip[1]) << 16) | (uint32(ip[2]) << 8) | uint32(ip[3])
}

func intToIP(ipInt uint32) net.IP {
	ip := make(net.IP, 4)
	ip[0] = byte(ipInt >> 24)
	ip[1] = byte(ipInt >> 16)
	ip[2] = byte(ipInt >> 8)
	ip[3] = byte(ipInt)
	return ip
}

func main() {
	var runMode int
	var queueNumber int
	var queueLength int
	var fromSubnet string
	var toSubnet string
	var showUsage bool

	flag.IntVar(&runMode, "mode", 1, "worker mode. egress(1), ingress(2)")
	flag.IntVar(&queueNumber, "num", 1, "nfqueue number")
	flag.IntVar(&queueLength, "len", 1024, "nfqueue length")
	flag.StringVar(&fromSubnet, "from", "", "rewrite from subnet. leave empty to skip check")
	flag.StringVar(&toSubnet, "to", "", "rewrite to subnet")
	flag.BoolVar(&showUsage, "help", false, "print usage")

	flag.Parse()

	fmt.Printf("mode: %v num: %v length: %v from: %v to: %v\n", runMode, queueNumber, queueLength, fromSubnet, toSubnet)

	if showUsage {
		flag.PrintDefaults()
		os.Exit(1)
	}

	var fromNetwork *net.IPNet
	if fromSubnet != "" {
		var err error
		_, fromNetwork, err = net.ParseCIDR(fromSubnet)
		if err != nil {
			fmt.Printf("[ERROR] error while parsing `from` network: %v\n", err)
			os.Exit(1)
		}
	}
	_, toNetwork, err := net.ParseCIDR(toSubnet)
	if err != nil {
		fmt.Printf("[ERROR] error while parsing `to` network: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("from: %v to: %v\n", fromNetwork, toNetwork)

	if fromNetwork != nil {
		fromNetworkPrefix, _ := fromNetwork.Mask.Size()
		toNetworkPrefix, _ := toNetwork.Mask.Size()
		if fromNetworkPrefix != toNetworkPrefix {
			fmt.Printf("[ERROR] `from`/`to` network mask not equal. from: %v to: %v\n", fromNetworkPrefix, toNetworkPrefix)
			os.Exit(1)
		}
	}

	fn := func(nfpkt *utils.NFQueuePacket) error {
		pkt := nfpkt.ToIPv4Packet()
		// fmt.Printf("%v\n", pkt)

		if ipLayer := pkt.Layer(layers.LayerTypeIPv4); ipLayer != nil {
			ipHeader, _ := ipLayer.(*layers.IPv4)
			fmt.Printf("packet source: %v packet destination: %v\n", ipHeader.SrcIP, ipHeader.DstIP)

			if runMode == 1 {
				// egress mode, change destination ip
				if fromNetwork != nil && !fromNetwork.Contains(ipHeader.DstIP) {
					nfpkt.Accept()
					return nil
				}

				toNetworkPrefix, _ := toNetwork.Mask.Size()
				nMask := (uint32(1) << (32 - toNetworkPrefix)) - 1
				hostId := ipToInt(ipHeader.DstIP) & nMask
				ipHeader.DstIP = intToIP(ipToInt(toNetwork.IP) | hostId)

				fmt.Printf("mask: %x host id: %x src: %v dst: %v\n", nMask, hostId, ipHeader.SrcIP, ipHeader.DstIP)
			} else {
				// ingress mode, change source ip
				if fromNetwork != nil && !fromNetwork.Contains(ipHeader.SrcIP) {
					nfpkt.Accept()
					return nil
				}

				toNetworkPrefix, _ := toNetwork.Mask.Size()
				nMask := (uint32(1) << (32 - toNetworkPrefix)) - 1
				hostId := ipToInt(ipHeader.SrcIP) & nMask
				ipHeader.SrcIP = intToIP(ipToInt(toNetwork.IP) | hostId)

				fmt.Printf("mask: %v host id: %v src: %v dst: %v\n", nMask, hostId, ipHeader.SrcIP, ipHeader.DstIP)
			}

			nfpkt.AcceptWithPacket(pkt)
		}

		return nil
	}

	service, err := utils.NewNFQueueService(uint16(queueNumber), fn,
		utils.WithDefaultVerdict(nfqueue.NfAccept),
		utils.WithMaxQueueLen(uint32(queueLength)))
	if err != nil {
		fmt.Printf("unable to create nfqueue wrapper: %v\n", err)
		return
	}
	defer service.Close()

	c := make(chan os.Signal, 1)
	done := make(chan bool, 1)
	go func() {
		sig := <-c
		fmt.Println(sig)
		done <- true
	}()
	signal.Notify(c, os.Interrupt)

	fmt.Printf("nfq-worker started, listening on queue %v\n", queueNumber)
	<-done
}
