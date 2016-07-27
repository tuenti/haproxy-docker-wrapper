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
	"bufio"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/url"
	"strings"
)

type Controller struct {
	socketPath string
	haproxy    *HaproxyServer

	done     bool
	listener net.Listener
}

func NewController(socketPath string, haproxy *HaproxyServer) *Controller {
	return &Controller{
		socketPath: socketPath,
		haproxy:    haproxy,
	}
}

func (c *Controller) handle(connection net.Conn) {
	defer connection.Close()
	r := bufio.NewReader(connection)
	d, err := ioutil.ReadAll(r)
	if err != nil {
		log.Println(err)
		return
	}
	command := strings.Trim(string(d), " \n")
	switch command {
	case "reload":
		if err := c.haproxy.Reload(); err != nil {
			log.Printf("Couldn't reload: %v\n", err)
		}
	default:
		log.Printf("Unknown command: %s\n", command)
	}
}

func (c *Controller) listen() error {
	u, err := url.Parse(c.socketPath)
	if err != nil {
		return fmt.Errorf("Couldn't parse control socket")
	}
	if u.Scheme == "" {
		u = &url.URL{Scheme: "unix", Opaque: c.socketPath}
	}

	var address string
	switch u.Scheme {
	case "unix":
		address = u.Opaque
	case "tcp", "udp":
		address = u.Host
	}

	listener, err := net.Listen(u.Scheme, address)
	c.listener = listener
	return err
}

func (c *Controller) Run() error {
	if err := c.listen(); err != nil {
		return err
	}
	log.Printf("Controller listening on '%s'\n", c.socketPath)

	for {
		fd, err := c.listener.Accept()
		if err != nil {
			if c.done {
				return nil
			}
			return fmt.Errorf("Accept error: %v", err)
		}
		go c.handle(fd)
	}
}

func (c *Controller) Stop() error {
	c.done = true
	return c.listener.Close()
}
