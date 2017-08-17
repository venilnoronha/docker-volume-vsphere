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

// This util provides various helper methods that can be used by different tests to
// fetch information like capacity, disk-format and attched-to-vm fields
// for volume using docker cli or admin cli.

package verification

import (
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/vmware/vsphere-storage-for-docker/tests/constants/admincli"
	"github.com/vmware/vsphere-storage-for-docker/tests/constants/dockercli"
	"github.com/vmware/vsphere-storage-for-docker/tests/constants/properties"
	"github.com/vmware/vsphere-storage-for-docker/tests/utils/esx"
	"github.com/vmware/vsphere-storage-for-docker/tests/utils/misc"
	"github.com/vmware/vsphere-storage-for-docker/tests/utils/ssh"
)

// GetVMAttachedToVolUsingDockerCli returns attached to vm field of volume using docker cli
// TODO: make this private member after finishing refactoring of volmprop_test.go and remove this TODO
func GetVMAttachedToVolUsingDockerCli(volName string, hostname string) string {
	cmd := dockercli.InspectVolume + " --format \"{{index .Status \\\"attached to VM\\\"}}\" " + volName
	op, _ := ssh.InvokeCommand(hostname, cmd)
	if op == "" {
		log.Fatal("Null value is returned by docker cli when looking for attached to vm field for volume. Output: ", op)
	}
	return op
}

// GetVMAttachedToVolUsingAdminCli returns attached to vm field of volume using admin cli
func GetVMAttachedToVolUsingAdminCli(volName string, hostname string) string {
	cmd := admincli.ListVolumes + "-c volume,attached-to 2>/dev/null | grep " + volName
	op, _ := ssh.InvokeCommand(hostname, cmd)
	volProps := strings.Fields(op)
	if op == "" {
		log.Printf("Null value is returned by admin cli when looking for attached to vm field for volume %s ", volName)
		return op
	}
	if len(volProps) != 2 {
		log.Fatalf("Admin cli output is expected to consist of two elements only - "+
			"volume name and attached-to-vm status. Actual output %s ", op)
	}
	return volProps[1]
}

// CheckVolumeAvailability returns true if the given volume is available
// from the specified VM; false otherwise.
func CheckVolumeAvailability(hostName string, reqVol string) bool {
	return CheckVolumeListAvailability(hostName, []string{reqVol})
}

// CheckVolumeListAvailability returns true if the given volumes specified in list are
// available from the specified VM; false otherwise.
func CheckVolumeListAvailability(hostName string, reqVolList []string) bool {
	log.Printf("Checking volume %s availability from VM [%s]\n", reqVolList, hostName)

	volumes, err := ssh.InvokeCommand(hostName, dockercli.ListVolumes)
	if err != nil {
		return false
	}
	//TODO: add more detailed verification here, e.g. checking volume driver name

	// check if each volume name is present in the output of docker volume ls
	for _, name := range reqVolList {
		name = strings.Replace(name, "'", "", -1)
		name = strings.Replace(name, "\"", "", -1)
		if strings.Contains(volumes, name) != true {
			return false
		}
	}

	return true
}

// GetFullVolumeName returns full volume name from the specified VM; return
// original short name if any error happens
func GetFullVolumeName(hostName string, volumeName string) string {
	log.Printf("Fetching full name for volume [%s] from VM [%s]\n", volumeName, hostName)

	cmd := dockercli.ListVolumes + "--filter name='" + volumeName + "' --format \"'{{.Name}}'\""
	fullName, err := ssh.InvokeCommand(hostName, cmd)
	if err != nil {
		log.Printf("Error: %s\n", err)
		return volumeName
	}

	log.Printf("Full volume name: [%s]\n", fullName)
	return fullName
}

// VerifyAttachedStatus - verify volume is attached and name of the VM attached
// is consistent on both docker host and ESX. The name of the volume MUST be a
// shorter name without @datastore suffix.
func VerifyAttachedStatus(name, hostName, esxName string) bool {
	log.Printf("Confirming attached status for volume [%s]\n", name)

	// Use full name to check volume status on docker host
	fullName := GetFullVolumeName(hostName, name)
	vmAttachedHost := GetVMAttachedToVolUsingDockerCli(fullName, hostName)

	// Use short name to check volume status on ESX
	vmAttachedESX := GetVMAttachedToVolUsingAdminCli(name, esxName)

	// Get VM name based on the host IP
	expectedVMName := esx.RetrieveVMNameFromIP(hostName)

	// Check if volume attached VM info matches between ESX and docker host
	isMatching := ((vmAttachedHost == expectedVMName) && (vmAttachedHost == vmAttachedESX))
	if !isMatching {
		log.Printf("Expected Attached VM name is [%s]", expectedVMName)
		log.Printf("Attached VM name from Docker CLI is [%s]", vmAttachedHost)
		log.Printf("Attached VM name from Admin CLI is [%s]", vmAttachedESX)
	}

	return isMatching
}

// getVolumeStatusHost - get the volume status on a given host
func getVolumeStatusHost(name, hostName string) string {
	cmd := dockercli.InspectVolume + " --format \"{{index .Status.status}}\" " + name
	// ignoring the error here, helper is part of polling util
	// error most likely to be "unable to reach host [ssh:255 error]"
	// VerifyDetachedStatus takes care of retry mechanism
	out, _ := ssh.InvokeCommand(hostName, cmd)
	return out
}

// GetSharedVolumeStatusHost - get the volume status on a given host, the volume is created by
// vsphere shared driver
func GetSharedVolumeStatusHost(name, hostName string) string {
	cmd := dockercli.InspectVolume + " --format \"{{index .Status \\\"Volume Status\\\"}}\" " + name
	log.Printf("GetSharedVolumeStatusHost: cmd[]", cmd)
	out, _ := ssh.InvokeCommand(hostName, cmd)
	return out
}

// VerifyDetachedStatus - check if the status gets detached within the timeout.
// The name of the volume MUST be a shorter name without @datastore suffix.
// Use this util in test scenarios where the test expects instant change of status to detached.
// eg: start a container ->  stop a container -> verify detached status
func VerifyDetachedStatus(name, hostName, esxName string) bool {
	log.Printf("Confirming detached status for volume [%s]\n", name)

	fullName := GetFullVolumeName(hostName, name)
	// Use full name to check volume status on docker host
	status := getVolumeStatusHost(fullName, hostName)
	if status != properties.DetachedStatus {
		return false
	}
	// Use short name to check volume status on ESX
	// this api returnes "detached" in when volume is detached
	status = GetVMAttachedToVolUsingAdminCli(name, esxName)
	if status != properties.DetachedStatus {
		return false
	}

	return true
}

// PollDetachedStatus Poll for detached status of a volume after 2 seconds till maxAttemps
// returns true if status is detached within retrials, else returns false
// Use this util in test scenarios where status update takes time.
// eg: restarts, failovers where some time is spent for plugin to stabilize and then perform mount/unmounts
func PollDetachedStatus(name, hostName, esxName string) bool {
	log.Printf("Polling detached status for volume [%s]\n", name)

	//TODO: Need to implement generic polling logic for better reuse
	const maxAttempt = 60
	name = "'" + name + "'"
	for attempt := 0; attempt < maxAttempt; attempt++ {
		if VerifyDetachedStatus(name, hostName, esxName) {
			return true
		}
		misc.SleepForSec(2)
	}
	log.Printf("Timed out to poll status\n")
	return false
}

// GetAssociatedPolicyName returns the vsan policy name used by the volume using docker cli
func GetAssociatedPolicyName(hostname string, volName string) (string, error) {
	cmd := dockercli.InspectVolume + " --format '{{index .Status \"vsan-policy-name\"}}' " + volName
	op, err := ssh.InvokeCommand(hostname, cmd)
	if op == "" {
		log.Printf("Null value is returned by docker cli when looking for the name of vsan policy used by volume. Output: %s", op)
	}
	return op, err
}

// GetVMGroupForVolume returns vmgroup field of volume using admin cli
// If the volume does not exist, err will be filled with "exit status 1"
func GetVMGroupForVolume(hostName string, volName string) (string, error) {
	cmd := admincli.ListVolumes + "-c volume,vmgroup 2>/dev/null | grep " + volName
	op, err := ssh.InvokeCommand(hostName, cmd)
	if err != nil {
		log.Printf("GetVMGroupForVolume return with err: %s", err.Error())
		return "", err
	}
	volProps := strings.Fields(op)
	if op == "" {
		msg := fmt.Sprintf("Null value is returned by admin cli when looking for vmgroup field for volume %s", volName)
		log.Printf(msg)
		return op, errors.New(msg)
	}
	if len(volProps) != 2 {
		log.Printf("Admin cli output is expected to consist of two elements only - "+
			"volume name and vmgrop. Actual output %s ", op)
	}
	return volProps[1], nil
}
