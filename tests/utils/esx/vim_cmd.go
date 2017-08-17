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

// vim-cmd helpers

package esx

import (
	"fmt"

	"github.com/vmware/vsphere-storage-for-docker/tests/constants/esx"
	"github.com/vmware/vsphere-storage-for-docker/tests/utils/ssh"
)

// SuspendResumeVM - suspend and resume a VM identified by name
func SuspendResumeVM(ip, name string) (string, error) {
	cmd := fmt.Sprintf(esx.VMId, name)
	id, err := ssh.InvokeCommand(ip, cmd)
	if err != nil {
		return "", err
	}

	cmd = fmt.Sprintf(esx.VMSuspendResume, id)
	return ssh.InvokeCommand(ip, cmd)
}
