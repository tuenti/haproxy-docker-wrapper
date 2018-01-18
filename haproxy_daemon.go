// Copyright © 2018 Tuenti Technologies S.L.
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
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"syscall"
	"time"
)

var nfQueueNumber uint
var netQueueIps string

func init() {
	flag.UintVar(&nfQueueNumber, "nf-queue-number", 0, "Netfilter queue number to retain connections during reload in daemon mode")
	flag.StringVar(&netQueueIps, "net-queue-ips", "", "Comma-separated list of IPs where connections will be retained during reload in daemon mode")
}

type HaproxyServerDaemon struct {
	sync.Mutex
	reloading sync.Mutex
	state     int
	netQueue  NetQueue

	path, pidFile, configFile string
}

func (s *HaproxyServerDaemon) buildCommand(reload bool) *exec.Cmd {
	args := []string{"-D", "-f", s.configFile, "-p", s.pidFile}

	if reload && s.IsRunning() {
		pids, _ := s.Pids()
		pidArgs := make([]string, len(pids))
		for i := range pids {
			pidArgs[i] = strconv.Itoa(pids[i])
		}
		args = append(args, "-sf")
		args = append(args, pidArgs...)
	}
	cmd := exec.Command(s.path, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stdout
	return cmd
}

func (s *HaproxyServerDaemon) Pids() ([]int, error) {
	var pids []int

	file, err := os.Open(s.pidFile)
	if err != nil {
		return nil, fmt.Errorf("couldn't open pidfile %s", s.pidFile)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Split(bufio.ScanWords)
	for scanner.Scan() {
		text := scanner.Text()
		pid, err := strconv.Atoi(text)
		if err == nil {
			pids = append(pids, pid)
		}
	}
	return pids, nil
}

func (s *HaproxyServerDaemon) Pid() int {
	pids, err := s.Pids()
	if err != nil {
		fmt.Println(err)
		return 0
	}
	if len(pids) == 0 {
		return 0
	}
	return pids[0]
}

func (s *HaproxyServerDaemon) Signal(signal os.Signal) error {
	p, err := os.FindProcess(s.Pid())
	if err != nil {
		return err
	}
	return p.Signal(signal)
}

func (s *HaproxyServerDaemon) IsRunning() bool {
	err := s.Signal(syscall.Signal(0))
	return err == nil
}

func (s *HaproxyServerDaemon) Kill() error {
	p, err := os.FindProcess(s.Pid())
	if err != nil {
		return err
	}
	return p.Kill()
}

func (s *HaproxyServerDaemon) Start() error {
	if s.IsRunning() {
		return fmt.Errorf("Server already started")
	}

	ips, err := ipArgs(netQueueIps)
	if err != nil {
		log.Fatalf("Expected comma-separated list of IPs: %v", err)
	}
	s.netQueue = NewNetQueue(nfQueueNumber, ips)

	cmd := s.buildCommand(false)
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Wait()
}

func (s *HaproxyServerDaemon) Stop() error {
	if !s.IsRunning() {
		return fmt.Errorf("Server not started")
	}
	err := s.Kill()
	if err != nil {
		return fmt.Errorf("Couldn't kill process: %v", err)
	}
	s.netQueue.Stop()
	return nil
}

func (s *HaproxyServerDaemon) requestReload() bool {
	s.Lock()
	defer s.Unlock()
	switch s.state {
	case StateIdle:
		s.state = StateReloading
	case StateReloading:
		s.state = StateWaiting
	case StateWaiting:
		return false
	}
	return true
}

func (s *HaproxyServerDaemon) finishReload() {
	s.Lock()
	defer s.Unlock()
	switch s.state {
	case StateIdle:
	case StateReloading:
		s.state = StateIdle
	case StateWaiting:
		s.state = StateReloading
	}
}

func (s *HaproxyServerDaemon) Reload() error {
	if !s.requestReload() {
		return nil
	}
	defer s.finishReload()

	s.reloading.Lock()
	defer s.reloading.Unlock()

	currentPids, _ := s.Pids()

	start := time.Now()
	err := func() error {
		cmd := s.buildCommand(s.IsRunning())

		netQueue.Capture()
		defer netQueue.Release()

		if err := cmd.Start(); err != nil {
			return err
		}
		if err := cmd.Wait(); err != nil {
			return fmt.Errorf("Haproxy couldn't reload configuration: %v", err)
		}
		return nil
	}()
	if err != nil {
		return err
	}
	log.Printf("Reload took %s", time.Since(start))

	for _, pid := range currentPids {
		p, err := os.FindProcess(pid)
		if err != nil {
			// This shouldn't happen in UNIX systems
			log.Printf("os.FindProcess(%d) failed, this shouldn't happen: %v\n", pid, err)
			continue
		}
		go func() {
			if _, err := p.Wait(); err != nil {
				log.Printf("Cannot wait for old haproxy: %v\n", err)
			}
			log.Printf("Old process with pid %d finished\n", p.Pid)
		}()
	}

	log.Println("Haproxy reloaded with pid", s.Pid())
	return nil
}
