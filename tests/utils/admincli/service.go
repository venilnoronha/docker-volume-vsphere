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

// This util is holding misc small functions for vmdkops service start | restart | stop |  status
// Note: currently suppoarted only "restart"

package admincli

import (
	"log"

	"github.com/vmware/vsphere-storage-for-docker/tests/constants/admincli"
	"github.com/vmware/vsphere-storage-for-docker/tests/utils/ssh"
)

// RestartVmdkopsService - restarts vmdkops service
func RestartVmdkopsService(ip string) (string, error) {
	log.Printf("Restarting vmdkops service on ESX host [%s]\n", ip)
	return ssh.InvokeCommand(ip, admincli.RestartService)
}
