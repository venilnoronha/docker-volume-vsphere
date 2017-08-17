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

package config_test

import (
	"github.com/stretchr/testify/assert"
	"github.com/vmware/vsphere-storage-for-docker/client_plugin/utils/config"
	"os"
	"testing"
)

func TestWindowsPath(t *testing.T) {
	assert.Equal(t, config.DefaultConfigPath, os.Getenv("PROGRAMDATA")+`\vsphere-storage-for-docker\vsphere-storage-for-docker.conf`)
	assert.Equal(t, config.DefaultLogPath, os.Getenv("LOCALAPPDATA")+`\vsphere-storage-for-docker\logs\vsphere-storage-for-docker.log`)
}
