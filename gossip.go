package main

import (
	"context"
	"fmt"
	"github.com/norbertcyran/gossip-multicast/gossip"
	"github.com/testground/sdk-go/network"
	"github.com/testground/sdk-go/run"
	"github.com/testground/sdk-go/runtime"
	"github.com/testground/sdk-go/sync"
	"net"
	"time"
)

func gossipSimulation(runenv *runtime.RunEnv, initCtx *run.InitContext) error {
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	initCtx.MustWaitAllInstancesInitialized(ctx)

	client := initCtx.SyncClient
	netclient := initCtx.NetClient

	config := &network.Config{
		Network: "default",
		Enable:  true,
	}
	netclient.MustConfigureNetwork(ctx, config)
	ip := netclient.MustGetDataNetworkIP()
	runenv.RecordMessage("My ip is %q", ip)

	st := sync.NewTopic("addrs", &net.IP{})
	ch := make(chan *net.IP)
	seq, _ := client.MustPublishSubscribe(ctx, st, ip, ch)
	neighbours := make([]string, 0, runenv.TestInstanceCount-1)
	for i := 1; i < runenv.TestInstanceCount; i++ {
		n := <-ch
		if ip.Equal(*n) {
			neighbours = append(neighbours, n.String())
		}
	}

	tConfig := &gossip.UDPTransportConfig{BindAddr: ip.String(), BindPort: 3333}
	t, err := gossip.NewUDPTransport(tConfig)
	if err != nil {
		return err
	}
	gConfig := &gossip.Config{
		GossipInterval:       200,
		RetransmitMultiplier: 2,
		Fanout:               1,
		Transport:            t,
		Tracer: &TestTracer{
			ctx:    ctx,
			client: client,
			runenv: runenv,
			seq:    seq,
		},
		Neighbours: neighbours,
	}
	_, err = gossip.StartService(gConfig)
	if err != nil {
		return err
	}

	if initCtx.GlobalSeq == 1 {
		addr, err := net.ResolveUDPAddr("udp", "localhost:3333")
		if err != nil {
			panic(err)
		}
		conn, err := net.DialUDP("udp", nil, addr)
		if err != nil {
			panic(err)
		}
		if _, err = fmt.Fprint(conn, []byte("dGVzdAo=")); err != nil {
			panic(err)
		}

	}

	<-client.MustBarrier(ctx, "rcv-msg", runenv.TestInstanceCount).C

	return nil
}
