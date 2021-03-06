// Copyright © 2017 Tuenti Technologies S.L.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	nfqueue "github.com/tuenti/go-netfilter-queue"
)

func init() {
	nfqueue.PacketReceiveTimeout = 10 * time.Millisecond
}

const maxPacketsInQueue = 65536

const iptablesAddFlag = "-A"
const iptablesDeleteFlag = "-D"

const procNetfilterQueuePath = "/proc/net/netfilter/nfnetlink_queue"

var netQueue NetQueue

func ipArgs(arg string) ([]net.IP, error) {
	if len(arg) == 0 {
		return nil, nil
	}
	ipArgs := strings.Split(arg, ",")
	ips := make([]net.IP, len(ipArgs))
	for i := range ipArgs {
		ip := net.ParseIP(ipArgs[i])
		if ip == nil {
			return nil, fmt.Errorf("incorrect IP: %s", ipArgs[i])
		}
		ips[i] = ip
	}
	return ips, nil
}

// A NetQueue retains new connections while haproxy is reloaded
type NetQueue interface {
	Capture()
	Release()
	Stop()
}

type dummyNetQueue struct{}

func (*dummyNetQueue) Capture() {}
func (*dummyNetQueue) Release() {}
func (*dummyNetQueue) Stop()    {}

type netfilterQueue struct {
	Number uint
	IPs    []net.IP

	capture, capturing, release chan struct{}

	cancel context.CancelFunc
}

// Factory method to obtain a netqueue depending on IP configuration
func NewNetQueue(n uint, ips []net.IP) NetQueue {
	if len(ips) == 0 {
		return &dummyNetQueue{}
	}
	q := netfilterQueue{
		Number:    n,
		IPs:       ips,
		capture:   make(chan struct{}),
		capturing: make(chan struct{}),
		release:   make(chan struct{}),
	}
	queue, err := nfqueue.NewNFQueue(uint16(q.Number), maxPacketsInQueue, nfqueue.NF_DEFAULT_PACKET_SIZE)
	if err != nil {
		panic(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	q.cancel = cancel
	go q.loop(queue, ctx)
	return &q
}

// Call to iptables to configure the rule to send packets
// to the queue
func (q *netfilterQueue) iptables(flag string) {
	for _, ip := range q.IPs {
		if ip.To4() == nil {
			log.Printf("Only IPv4 addresses supported: %s found", ip.String())
			continue
		}
		args := []string{
			flag,
			"INPUT", "-j", "NFQUEUE", "-w",
			"-p", "tcp", "--syn", "--destination", ip.String(),
			"--queue-num", strconv.Itoa(int(q.Number)),
		}

		err := exec.Command("iptables", args...).Run()
		if err != nil {
			panic(fmt.Sprintf("iptables failed: %v", err))
		}
	}
}

func (q *netfilterQueue) loop(queue *nfqueue.NFQueue, ctx context.Context) {
	defer queue.Close()
	defer close(q.capture)
	defer close(q.capturing)
	defer close(q.release)

	procNf, err := ReadProcNetfilter()
	if err != nil {
		panic(err)
	}

	lastQueueDropped := uint(0)
	lastUserDropped := uint(0)

	// Buffered channel, we don't want to block writes on it
	packets := make(chan nfqueue.NFPacket, nfqueue.NF_DEFAULT_PACKET_SIZE)
	queuedPackets := int64(0)
	go func() {
		for {
			// We have to be reading packets before start capturing,
			// or they are lost
			select {
			case packet := <-queue.GetPackets():
				packets <- packet
				atomic.AddInt64(&queuedPackets, 1)
			case <-ctx.Done():
				return
			}
		}
	}()

	for {
		// Control locks
		select {
		case <-q.capture:
		case <-ctx.Done():
			return
		}
		func() {
			q.iptables(iptablesAddFlag)
			defer q.iptables(iptablesDeleteFlag)
			q.capturing <- struct{}{}
			<-q.release
		}()

		err := procNf.Update()
		if err != nil {
			log.Printf("Couldn't update netfilter queue stats: %v\n", err)
			continue
		}

		// Accept all waiting packets according to information in proc fs
		count := int64(0)
		for {
			qData, found := procNf.Get(q.Number)
			if !found || qData.Waiting == 0 {
				break
			}
			// We only trust in the number of queued packets, as the last read
			// value for waiting packets can be outdated and we'd get locked
			// reading from the channel
			n := atomic.LoadInt64(&queuedPackets)
			for i := int64(0); i < n; i++ {
				packet := <-packets
				packet.SetVerdict(nfqueue.NF_ACCEPT)
			}
			atomic.AddInt64(&queuedPackets, -n)
			count += n
			err := procNf.Update()
			if err != nil {
				log.Printf("Couldn't update netfilter queue stats: %v\n", err)
				break
			}
		}

		// Show stats
		if count > 0 {
			log.Printf("Delayed %d packages during reloads\n", count)
		}

		if qData, found := procNf.Get(q.Number); found {
			if qData.QueueDropped > lastQueueDropped {
				log.Printf("Dropped %d packages due to full queue\n",
					qData.QueueDropped-lastQueueDropped)
				lastQueueDropped = qData.QueueDropped
			}
			if qData.UserDropped > lastUserDropped {
				log.Printf("Dropped %d packages before reaching user space\n",
					qData.UserDropped-lastUserDropped)
				lastUserDropped = qData.UserDropped
			}
		}
	}
}

func (q *netfilterQueue) Capture() {
	q.capture <- struct{}{}
	<-q.capturing
}

func (q *netfilterQueue) Release() {
	q.release <- struct{}{}
}

// Canceling the context will finish loop() and close
// all queues and channels, after calling this method
// this object shouldn't be used anymore
func (q *netfilterQueue) Stop() {
	q.cancel()
}

type ProcNetfilterQueue struct {
	ID           uint
	PortID       uint
	Waiting      uint
	CopyMode     uint
	CopyRange    uint
	QueueDropped uint
	UserDropped  uint
	LastSeq      uint
	One          uint
}

type ProcNetfilter struct {
	sync.RWMutex

	queues map[uint]ProcNetfilterQueue
}

func (pn *ProcNetfilter) Get(id uint) (ProcNetfilterQueue, bool) {
	pn.RLock()
	defer pn.RUnlock()

	q, found := pn.queues[id]
	return q, found
}

func (pn *ProcNetfilter) Update() error {
	pn.Lock()
	defer pn.Unlock()

	f, err := os.Open(procNetfilterQueuePath)
	if err != nil {
		return err
	}
	defer f.Close()

	seen := make(map[uint]bool)

	var id, portID, waiting, copyMode, copyRange, queueDropped, userDropped, lastSeq, one uint
	for {
		_, err := fmt.Fscanf(f, "%d %d %d %d %d %d %d %d %d\n",
			&id, &portID, &waiting, &copyMode, &copyRange, &queueDropped, &userDropped, &lastSeq, &one)
		seen[id] = true
		pn.queues[id] = ProcNetfilterQueue{
			ID:           id,
			PortID:       portID,
			Waiting:      waiting,
			CopyMode:     copyMode,
			CopyRange:    copyRange,
			QueueDropped: queueDropped,
			UserDropped:  userDropped,
			LastSeq:      lastSeq,
			One:          one,
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}
	for k := range pn.queues {
		if _, found := seen[k]; !found {
			delete(pn.queues, k)
		}
	}
	return nil
}

func ReadProcNetfilter() (*ProcNetfilter, error) {
	pn := &ProcNetfilter{queues: make(map[uint]ProcNetfilterQueue)}
	err := pn.Update()
	if err != nil {
		return nil, err
	}
	return pn, nil
}
