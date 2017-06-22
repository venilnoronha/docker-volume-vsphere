// Copyright 2017 VMware, Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

// A VMDK Docker Data Volume plugin implementation for linux.
// relies on docker/go-plugins-helpers/volume API

import (
	"os"
	"path/filepath"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/go-plugins-helpers/volume"
)

const (
	mountRoot     = "/mnt/vmdk" // VMDK and photon volumes are mounted here
	pluginSockDir = "/run/docker/plugins"
)

// SockServer serves HTTP requests over unix sock.
type SockServer struct {
	Server
	driverName string
	driver     *volume.Driver
}

// An equivalent function is not exported from the SDK.
// API supports passing a full address instead of just name.
// Using the full path during creation and deletion. The path
// is the same as the one generated internally. Ideally SDK
// should have ability to clean up sock file instead of replicating
// it here.
func fullSocketAddress(pluginName string) string {
	return filepath.Join(pluginSockDir, pluginName+".sock")
}

// NewServer creates a new instance of SockServer.
func NewServer(driverName string, driver *volume.Driver) *SockServer {
	return &SockServer{driverName: driverName, driver: driver}
}

// Init initializes a handler to service Docker requests using the driver.
func (s *SockServer) Init() {
	handler := volume.NewHandler(*s.driver)

	log.WithFields(log.Fields{
		"address": fullSocketAddress(s.driverName),
	}).Info("Going into ServeUnix - Listening on Unix socket ")

	log.Info(handler.ServeUnix("root", fullSocketAddress(s.driverName)))
}

// Destroy removes the Docker plugin socket.
func (s *SockServer) Destroy() {
	os.Remove(fullSocketAddress(s.driverName))
}
