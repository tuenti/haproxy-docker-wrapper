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
	"os"
	"os/exec"
	"strconv"
	"time"
)

var reloadTimeout = 15 * time.Second

type HaproxyServer struct {
	path, pidFile, configFile string

	command *exec.Cmd
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
	if reload && s.command != nil && s.command.Process != nil {
		args = append(args, "-sf", strconv.Itoa(s.command.Process.Pid))
	}
	cmd := exec.Command(s.path, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stdout
	return cmd
}

func (s *HaproxyServer) Start() error {
	if s.command != nil {
		return fmt.Errorf("Server already started")
	}
	cmd := s.buildCommand(false)
	if err := cmd.Start(); err != nil {
		return err
	}
	s.command = cmd
	return nil
}

func (s *HaproxyServer) Stop() error {
	if s.command == nil || s.command.Process == nil {
		return fmt.Errorf("Server not started")
	}
	err := s.command.Process.Kill()
	if err != nil {
		return fmt.Errorf("Couldn't kill process: %v", err)
	}
	err = s.command.Wait()
	s.command = nil
	return err
}

func (s *HaproxyServer) Reload() error {
	if s.command == nil || s.command.Process == nil {
		return fmt.Errorf("Server not started")
	}
	cmd := s.buildCommand(true)
	if err := cmd.Start(); err != nil {
		return err
	}
	current := make(chan error)
	go func() {
		current <- cmd.Wait()
	}()
	select {
	case err := <-current:
		s.command = cmd
		return err
	case <-time.After(reloadTimeout):
	}
	return fmt.Errorf("Server couldn't be reloaded")
}
