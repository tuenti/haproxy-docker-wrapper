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
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/vishvananda/netlink"
)

var nextQueueId = uint(0)

func newQueueId() uint {
	id := nextQueueId
	nextQueueId++
	return id
}

func pingHTTPServer(ip net.IP, port int) (io.Closer, error) {
	l, err := net.ListenTCP("tcp", &net.TCPAddr{IP: ip, Port: port})
	if err != nil {
		return nil, err
	}
	go func() {
		err = http.Serve(l, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintf(w, "pong")
		}))
	}()
	return l, nil
}

type ReleasedCheck struct {
	sync.Mutex
	Released bool
	fault    bool
}

func (c *ReleasedCheck) FailIfNotReleased(t *testing.T) {
	c.Lock()
	defer c.Unlock()

	if !c.fault && !c.Released {
		c.fault = true
		t.Fatal("GET finished before release queue")
	}
}

func waitForQueued(id, n uint) error {
	pn, err := ReadProcNetfilter()
	if err != nil {
		return err
	}
	for retries := 5; retries > 0; retries-- {
		pn.Update()
		if err != nil {
			return fmt.Errorf("while reading %s: %s", procNetfilterQueuePath, err)
		}
		if procQueue, found := pn.Get(id); found {
			if procQueue.Waiting >= n {
				return nil
			}
		}
		<-time.After(5 * time.Millisecond)
	}
	return fmt.Errorf("timeout while waiting for queued packages")
}

func sanityCheckQueue(queue uint) error {
	pn, err := ReadProcNetfilter()
	if err != nil {
		return fmt.Errorf("while reading %s: %s", procNetfilterQueuePath, err)
	}

	if procQueue, found := pn.Get(queue); found {
		if procQueue.QueueDropped > 0 || procQueue.UserDropped > 0 {
			return fmt.Errorf("packets dropped")
		}
	} else {
		return fmt.Errorf("queue not found in %s", procNetfilterQueuePath)
	}
	return nil
}

func TestNetfilterQueue(t *testing.T) {
	lo, _ := netlink.LinkByName("lo")
	addr, _ := netlink.ParseAddr("127.0.1.100/32")
	err := netlink.AddrAdd(lo, addr)
	if err != nil {
		t.Fatal("couldn't change network configuration: ", err)
	}
	defer netlink.AddrDel(lo, addr)

	queueId := newQueueId()
	port := 80
	nfQueue := NewNetQueue(queueId, []net.IP{addr.IP})
	s, err := pingHTTPServer(addr.IP, port)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	requests := uint(100)

	errResp := make(chan error)
	releaseCheck := ReleasedCheck{}
	nfQueue.Capture()
	for i := uint(0); i < requests; i++ {
		go func() {
			_, err = http.Get(fmt.Sprintf("http://%s:%d/", addr.IP, port))
			releaseCheck.FailIfNotReleased(t)
			errResp <- err
		}()
	}

	err = waitForQueued(queueId, requests)
	if err != nil {
		t.Fatal(err)
	}

	releaseCheck.Lock()
	releaseCheck.Released = true
	nfQueue.Release()
	releaseCheck.Unlock()

	for i := uint(0); i < requests; i++ {
		select {
		case e := <-errResp:
			if e != nil {
				t.Fatal(e)
			}
		case <-time.After(500 * time.Millisecond):
			t.Fatalf("Client timeout after %d packets", i)
		}
	}

	err = sanityCheckQueue(queueId)
	if err != nil {
		t.Fatal(err)
	}
}

func TestNetfilterQueueNoIPs(t *testing.T) {
	queueId := newQueueId()
	nfQueue := NewNetQueue(queueId, nil)

	nfQueue.Capture()
	defer nfQueue.Release()

	pn, err := ReadProcNetfilter()
	if err != nil {
		t.Fatal(err)
	}

	_, found := pn.Get(queueId)

	if found {
		t.Fatal("queue configured but it should be disabled")
	}
}

func TestNetfilterQueueExists(t *testing.T) {
	lo, _ := netlink.LinkByName("lo")
	addr, _ := netlink.ParseAddr("127.0.1.101/32")
	err := netlink.AddrAdd(lo, addr)
	if err != nil {
		t.Fatal("couldn't change network configuration: ", err)
	}
	defer netlink.AddrDel(lo, addr)

	queueId := newQueueId()
	nfQueue := NewNetQueue(queueId, []net.IP{addr.IP})

	nfQueue.Capture()
	defer nfQueue.Release()

	pn, err := ReadProcNetfilter()
	if err != nil {
		t.Fatal(err)
	}

	_, found := pn.Get(queueId)

	if !found {
		t.Fatal("queue couldn't be found")
	}
}

func BenchmarkProcNetfilterUpdateAndRead(b *testing.B) {
	lo, _ := netlink.LinkByName("lo")
	addr, _ := netlink.ParseAddr("127.0.1.101/32")
	err := netlink.AddrAdd(lo, addr)
	if err != nil {
		b.Fatal("couldn't change network configuration: ", err)
	}
	defer netlink.AddrDel(lo, addr)

	queueId := newQueueId()
	_ = NewNetQueue(queueId, []net.IP{addr.IP})

	pn, err := ReadProcNetfilter()
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pn.Update()
		_, found := pn.Get(queueId)

		if !found {
			b.Fatal("queue couldn't be found")
		}
	}
}

func BenchmarkNetfilterQueueCaptureReleaseOverload(b *testing.B) {
	lo, _ := netlink.LinkByName("lo")
	addr, _ := netlink.ParseAddr("127.0.1.100/32")
	err := netlink.AddrAdd(lo, addr)
	if err != nil {
		b.Fatal("couldn't change network configuration: ", err)
	}
	defer netlink.AddrDel(lo, addr)

	queueId := newQueueId()
	nfQueue := NewNetQueue(queueId, []net.IP{addr.IP})

	// TODO: Send packets during the capture
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		nfQueue.Capture()
		nfQueue.Release()
	}
	b.StopTimer()
}
