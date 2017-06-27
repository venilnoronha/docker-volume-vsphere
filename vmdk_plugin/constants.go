// Copyright 2016-2017 VMware, Inc. All Rights Reserved.
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

const (
	// vDVS drivers.
	photonDriver  = "photon"
	vmdkDriver    = "vmdk"
	vsphereDriver = "vsphere"

	// Default ESX service port.
	defaultPort = 1019

	// Docker driver types.
	volumeDriver = "VolumeDriver"

	// Docker plugin handshake endpoint.
	// Also see https://docs.docker.com/engine/extend/plugin_api/#handshake-api
	pluginActivatePath = "/Plugin.Activate"

	// Docker volume plugin endpoints.
	// Also see https://docs.docker.com/engine/extend/plugins_volume/#volume-plugin-protocol
	volumeDriverCreatePath = "/VolumeDriver.Create"
)
