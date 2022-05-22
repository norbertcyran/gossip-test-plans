package main

import (
	"context"
	"github.com/norbertcyran/gossip-multicast/gossip"
	"github.com/testground/sdk-go/runtime"
	"github.com/testground/sdk-go/sync"
)

type TestTracer struct {
	seq    int64
	ctx    context.Context
	runenv *runtime.RunEnv
	client sync.Client
}

func (t *TestTracer) Trace(evt gossip.EventType) {
	switch evt {
	case gossip.ReceivedMessage:
		t.runenv.RecordMessage("Instance %d received message", t.seq)
		t.client.MustSignalEntry(t.ctx, "rcv-msg")
	case gossip.DuplicatedMessage:
		t.runenv.RecordMessage("Duplicated message on instance %d", t.seq)
	case gossip.ServiceStarted:
		t.client.MustSignalEntry(t.ctx, "gossip-started")
	}
}
