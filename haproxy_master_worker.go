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
	"fmt"
	"log"
	"os"
	"os/exec"
	"syscall"
)

type HaproxyServerMasterWorker struct {
	command *exec.Cmd

	path, pidFile, configFile string
}

func (s *HaproxyServerMasterWorker) IsRunning() bool {
	if s.command == nil || s.command.Process == nil {
		return false
	}
	err := s.command.Process.Signal(syscall.Signal(0))
	return err == nil
}

func (s *HaproxyServerMasterWorker) Reload() error {
	if !s.IsRunning() {
		return s.Start()
	}
	err := s.command.Process.Signal(syscall.SIGUSR2)
	if err != nil {
		return fmt.Errorf("couldn't kill process: %v", err)
	}
	return nil
}

func (s *HaproxyServerMasterWorker) Start() error {
	if s.IsRunning() {
		return fmt.Errorf("server already started")
	}
	args := []string{"-W", "-f", s.configFile, "-p", s.pidFile}
	s.command = exec.Command(s.path, args...)
	s.command.Stdout = os.Stdout
	s.command.Stderr = os.Stdout
	if err := s.command.Start(); err != nil {
		return err
	}

	go func() {
		err := s.command.Wait()
		if err != nil {
			log.Printf("Haproxy finished with error: %v", err)
		} else {
			log.Println("Haproxy finished")
		}
	}()
	return nil
}

func (s *HaproxyServerMasterWorker) Stop() error {
	if !s.IsRunning() {
		return fmt.Errorf("server is not running")
	}
	err := s.command.Process.Kill()
	if err != nil {
		return fmt.Errorf("couldn't kill server")
	}
	return nil
}
