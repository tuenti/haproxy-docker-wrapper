// Copyright © 2016 Tuenti Technologies S.L.
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
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

type HaproxyServer struct {
	path, pidFile, configFile string
}

func NewHaproxyServer(path, pidFile, configFile string) *HaproxyServer {
	return &HaproxyServer{
		path:       path,
		pidFile:    pidFile,
		configFile: configFile,
	}
}

func (s *HaproxyServer) buildCommand(reload bool) *exec.Cmd {
	args := []string{"-f", s.configFile, "-p", s.pidFile}
	if reload && s.IsRunning() {
		args = append(args, "-sf", strconv.Itoa(s.Pid()))
	}
	cmd := exec.Command(s.path, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stdout
	return cmd
}

func (s *HaproxyServer) Pid() int {
	b, err := ioutil.ReadFile(s.pidFile)
	if err != nil {
		return 0
	}
	pid, err := strconv.Atoi(strings.Trim(string(b), " \n"))
	if err != nil {
		return 0
	}
	return pid
}

func (s *HaproxyServer) Signal(signal os.Signal) error {
	p, err := os.FindProcess(s.Pid())
	if err != nil {
		return err
	}
	return p.Signal(signal)
}

func (s *HaproxyServer) IsRunning() bool {
	err := s.Signal(syscall.Signal(0))
	return err == nil
}

func (s *HaproxyServer) Kill() error {
	p, err := os.FindProcess(s.Pid())
	if err != nil {
		return err
	}
	return p.Kill()
}

func (s *HaproxyServer) Start() error {
	if s.IsRunning() {
		return fmt.Errorf("Server already started")
	}
	cmd := s.buildCommand(false)
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Wait()
}

func (s *HaproxyServer) Stop() error {
	if !s.IsRunning() {
		return fmt.Errorf("Server not started")
	}
	err := s.Kill()
	if err != nil {
		return fmt.Errorf("Couldn't kill process: %v", err)
	}
	return nil
}

func (s *HaproxyServer) Reload() error {
	if !s.IsRunning() {
		log.Println("Server not started trying to start...")
		return s.Start()
	}
	currentPid := s.Pid()
	log.Println("Reloading haproxy with pid", currentPid)

	cmd := s.buildCommand(true)
	if err := cmd.Start(); err != nil {
		return err
	}
	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("Haproxy couldn't reload configuration: %v", err)
	}
	log.Println("Haproxy reloaded with pid", s.Pid())
	return nil
}
