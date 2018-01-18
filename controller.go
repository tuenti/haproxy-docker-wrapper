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
	"net"
	"net/http"
)

type Controller struct {
	address string
	haproxy HaproxyServer

	done     bool
	listener net.Listener
}

func NewController(address string, haproxy HaproxyServer) *Controller {
	return &Controller{
		address: address,
		haproxy: haproxy,
	}
}

func (c *Controller) Run() error {
	listener, err := net.Listen("tcp", c.address)
	if err != nil {
		return err
	}
	c.listener = listener
	log.Printf("Controller listening on '%s'\n", c.address)

	handler := http.NewServeMux()
	handler.HandleFunc("/reload", func(w http.ResponseWriter, req *http.Request) {
		if err := c.haproxy.Reload(); err != nil {
			msg := fmt.Sprintf("Couldn't reload: %v\n", err)
			log.Println(msg)
			http.Error(w, msg, http.StatusInternalServerError)
			return
		}
		fmt.Fprintf(w, "OK\n")
	})

	err = http.Serve(c.listener, handler)
	if err != nil && !c.done {
		return fmt.Errorf("Controller error: %v", err)
	}
	return nil
}

func (c *Controller) Stop() error {
	c.done = true
	return c.listener.Close()
}
