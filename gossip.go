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
		Network:       "default",
		Enable:        true,
		CallbackState: "network-configured",
	}
	netclient.MustConfigureNetwork(ctx, config)
	ip := netclient.MustGetDataNetworkIP()
	addr := &net.UDPAddr{IP: ip, Port: 3333}
	runenv.RecordMessage("My ip is %q", ip)

	st := sync.NewTopic("addrs", &net.UDPAddr{})
	ch := make(chan *net.UDPAddr)
	client.MustPublishSubscribe(ctx, st, addr, ch)
	neighbours := make([]string, 0, runenv.TestInstanceCount-1)
	for i := 0; i < runenv.TestInstanceCount; i++ {
		n := <-ch
		if nAddr := n.String(); addr.String() != nAddr {
			neighbours = append(neighbours, nAddr)
		}
	}
	runenv.RecordMessage("Found %d neighbours", len(neighbours))

	tConfig := &gossip.UDPTransportConfig{BindAddr: addr.IP.String(), BindPort: addr.Port}
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
			seq:    initCtx.GlobalSeq,
		},
		Neighbours: neighbours,
	}
	_, err = gossip.StartService(gConfig)
	if err != nil {
		return err
	}

	runenv.RecordMessage("Waiting for other instances to be initiaized")
	<-client.MustBarrier(ctx, "gossip-started", runenv.TestInstanceCount).C

	var ts time.Time
	if initCtx.GlobalSeq == 1 {
		ts = time.Now()
		go func() {
			defer run.HandlePanics()
			runenv.RecordMessage("All instances initialized, sending messages")
			conn, err := net.DialUDP("udp", nil, addr)
			defer conn.Close()
			if err != nil {
				panic(err)
			}
			if _, err = fmt.Fprint(conn, "dGVzdAo="); err != nil {
				panic(err)
			}
		}()
	}
	runenv.RecordMessage("Waiting for other instances to receive a message")
	<-client.MustBarrier(ctx, "rcv-msg", runenv.TestInstanceCount).C
	if initCtx.GlobalSeq == 1 {
		elapsed := time.Since(ts)
		runenv.R().RecordPoint("total-sync-time", elapsed.Seconds())
		runenv.RecordMessage("Broadcast succeeded in %fs", elapsed.Seconds())
	}
	return nil
}
