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
	"encoding/json"
	"fmt"
	"log"

	"gopkg.in/mcuadros/go-syslog.v2"
)

type SyslogServer struct {
	port   uint
	server *syslog.Server
}

func NewSyslogServer(port uint) *SyslogServer {
	return &SyslogServer{port: port}
}

func (s *SyslogServer) Start() error {
	if s.server != nil {
		return fmt.Errorf("Server already started")
	}

	channel := make(syslog.LogPartsChannel)
	handler := syslog.NewChannelHandler(channel)

	bindAddress := fmt.Sprintf("127.0.0.1:%d", s.port)

	s.server = syslog.NewServer()
	s.server.SetFormat(syslog.Automatic)
	s.server.SetHandler(handler)

	if err := s.server.ListenUDP(bindAddress); err != nil {
		return err
	}
	if err := s.server.Boot(); err != nil {
		return err
	}

	log.Printf("Syslog embedded server listening on %s", bindAddress)

	go func(channel syslog.LogPartsChannel) {
		for logParts := range channel {
			if content, ok := logParts["content"]; ok {
				log.Println(content)
			} else if d, err := json.Marshal(logParts); err == nil {
				log.Println(d)
			} else {
				log.Println(logParts)
			}
		}
	}(channel)

	return nil
}

func (s *SyslogServer) Stop() error {
	if s.server == nil {
		return fmt.Errorf("Server not started")
	}
	if err := s.server.Kill(); err != nil {
		return fmt.Errorf("Couldn't kill server: %v", err)
	}
	s.server = nil
	return nil
}
