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
)

const (
	StateIdle = iota
	StateReloading
	StateWaiting
)

type HaproxyServer interface {
	Start() error
	Stop() error
	Reload() error
	IsRunning() bool
}

func NewHaproxyServer(path, pidFile, configFile, mode string) (HaproxyServer, error) {
	switch mode {
	case "daemon":
		return &HaproxyServerDaemon{
			path:       path,
			pidFile:    pidFile,
			configFile: configFile,
		}, nil
	case "master-worker":
		return &HaproxyServerMasterWorker{
			path:       path,
			pidFile:    pidFile,
			configFile: configFile,
		}, nil
	default:
		return nil, fmt.Errorf("unknown haproxy mode: %s", mode)
	}
}
