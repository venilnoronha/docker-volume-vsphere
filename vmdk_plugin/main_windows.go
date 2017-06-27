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

// A VMDK Docker Data Volume plugin implementation for windows.

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"

	"github.com/Microsoft/go-winio"
	log "github.com/Sirupsen/logrus"
	"github.com/docker/go-plugins-helpers/volume"
)

const (
	volumeDriver = "VolumeDriver"         // Docker volume driver type
	npipeAddr    = `\\.\pipe\vsphere-dvs` // Plugin's npipe address

	// Docker plugin handshake endpoint.
	// Also see https://docs.docker.com/engine/extend/plugin_api/#handshake-api
	pluginActivatePath = "/Plugin.Activate"

	// Docker volume plugin endpoints.
	// Also see https://docs.docker.com/engine/extend/plugins_volume/#volume-plugin-protocol
	volumeDriverCreatePath = "/VolumeDriver.Create"
)

var (
	mountRoot = filepath.Join(os.Getenv("LOCALAPPDATA"), "docker-volume-vsphere", "mounts") // VMDK volumes are mounted here
)

// NpipePluginServer serves HTTP requests from Docker over windows npipe.
type NpipePluginServer struct {
	PluginServer
	driver   *volume.Driver // The driver implementation
	mux      *http.ServeMux // The HTTP mux
	listener net.Listener   // The npipe listener
}

// Response for /Plugin.Activate to activate the Docker plugin.
type PluginActivateResponse struct {
	Implements []string // Slice of implemented driver types
}

// NewPluginServer returns a new instance of NpipePluginServer.
func NewPluginServer(driverName string, driver *volume.Driver) *NpipePluginServer {
	return &NpipePluginServer{driver: driver, mux: http.NewServeMux()}
}

// writeJSON writes the JSON encoding of v to w, or returns an
// error if marshalling fails.
func writeJSON(v interface{}, w *http.ResponseWriter) error {
	bytes, err := json.Marshal(v)
	if err == nil {
		fmt.Fprintf(*w, string(bytes))
	}
	return err
}

// PluginActivate writes the plugin's handshake response to w.
func (s *NpipePluginServer) PluginActivate(w http.ResponseWriter, r *http.Request) {
	resp := &PluginActivateResponse{Implements: []string{volumeDriver}}
	writeJSON(resp, &w)
	log.WithFields(log.Fields{"resp": resp}).Info("Plugin activated ")
}

// VolumeDriverCreate creates a volume and writes the response to w.
func (s *NpipePluginServer) VolumeDriverCreate(w http.ResponseWriter, r *http.Request) {
	var volumeReq volume.Request
	err := json.NewDecoder(r.Body).Decode(&volumeReq)
	if err != nil {
		log.WithFields(log.Fields{"err": err}).Error("Failed to service /VolumeDriver.Create request")
		http.Error(w, err.Error(), 400)
		return
	}

	resp := (*s.driver).Create(volumeReq)
	writeJSON(resp, &w)
	log.WithFields(log.Fields{"resp": resp}).Info("Serviced /VolumeDriver.Create ")
}

// registerHttpHandlers registers handlers with the HTTP mux.
func (s *NpipePluginServer) registerHandlers() {
	s.mux.HandleFunc(pluginActivatePath, s.PluginActivate)
	s.mux.HandleFunc(volumeDriverCreatePath, s.VolumeDriverCreate)
}

// Init initializes the npipe listener which serves HTTP requests
// from Docker using the HTTP mux.
func (s *NpipePluginServer) Init() {
	var err error
	s.listener, err = winio.ListenPipe(npipeAddr, nil)
	if err != nil {
		msg := fmt.Sprintf("Failed to initialize npipe at %s - exiting", npipeAddr)
		log.WithFields(log.Fields{"err": err}).Error(msg)
		fmt.Println(msg)
		os.Exit(1)
	}

	s.registerHandlers()
	log.WithFields(log.Fields{"npipe": npipeAddr}).Info("Going into Serve - Listening on npipe ")
	log.Info(http.Serve(s.listener, s.mux))
}

// Destroy shuts down the npipe listener.
func (s *NpipePluginServer) Destroy() {
	s.listener.Close()
}
