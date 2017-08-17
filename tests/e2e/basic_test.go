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

// This test suite includes test cases to verify basic functionality
// in most common configurations

// +build runalways runoncewin

package e2e

import (
	"github.com/vmware/vsphere-storage-for-docker/tests/utils/dockercli"
	"github.com/vmware/vsphere-storage-for-docker/tests/utils/inputparams"
	"github.com/vmware/vsphere-storage-for-docker/tests/utils/misc"
	"github.com/vmware/vsphere-storage-for-docker/tests/utils/verification"
	. "gopkg.in/check.v1"
)

type BasicTestSuite struct {
	config        *inputparams.TestConfig
	esx           string
	vm1           string
	vm2           string
	vm1Name       string
	vm2Name       string
	volName1      string
	volName2      string
	containerName string
}

func (s *BasicTestSuite) SetUpSuite(c *C) {
	s.config = inputparams.GetTestConfig()
	if s.config == nil {
		c.Skip("Unable to retrieve test config, skipping basic tests")
	}

	s.esx = s.config.EsxHost
	s.vm1 = s.config.DockerHosts[0]
	s.vm1Name = s.config.DockerHostNames[0]
	if len(s.config.DockerHosts) == 2 {
		s.vm2 = s.config.DockerHosts[1]
		s.vm2Name = s.config.DockerHostNames[1]
	}
}

func (s *BasicTestSuite) SetUpTest(c *C) {
	s.volName1 = inputparams.GetUniqueVolumeName(c.TestName())
	s.volName2 = inputparams.GetUniqueVolumeName(c.TestName())
	s.containerName = inputparams.GetUniqueContainerName(c.TestName())
}

var _ = Suite(&BasicTestSuite{})

// Test volume lifecycle management on different datastores:
// VM1 - created on local VMFS datastore
// VM2 - created on shared VMFS datastore
// VM3 - created on shared VSAN datastore (TODO: currently not available)
//
// Test steps:
// 1. Create a volume
// 2. Verify the volume is available
// 3. Attach the volume
// 4. Verify volume status is attached
// 5. Remove the volume (expect fail)
// 6. Remove the container
// 7. Verify volume status is detached
// 8. Remove the volume
// 9. Verify the volume is unavailable
func (s *BasicTestSuite) TestVolumeLifecycle(c *C) {
	misc.LogTestStart(c.TestName())

	for _, host := range s.config.DockerHosts {
		out, err := dockercli.CreateVolume(host, s.volName1)
		c.Assert(err, IsNil, Commentf(out))

		accessible := verification.CheckVolumeAvailability(host, s.volName1)
		c.Assert(accessible, Equals, true, Commentf("Volume %s is not available", s.volName1))

		out, err = dockercli.AttachVolume(host, s.volName1, s.containerName)
		c.Assert(err, IsNil, Commentf(out))

		status := verification.VerifyAttachedStatus(s.volName1, host, s.esx)
		c.Assert(status, Equals, true, Commentf("Volume %s is not attached", s.volName1))

		out, err = dockercli.DeleteVolume(host, s.volName1)
		c.Assert(err, Not(IsNil), Commentf(out))

		out, err = dockercli.RemoveContainer(host, s.containerName)
		c.Assert(err, IsNil, Commentf(out))

		status = verification.VerifyDetachedStatus(s.volName1, host, s.esx)
		c.Assert(status, Equals, true, Commentf("Volume %s is still attached", s.volName1))

		out, err = dockercli.DeleteVolume(host, s.volName1)
		c.Assert(err, IsNil, Commentf(out))

		accessible = verification.CheckVolumeAvailability(host, s.volName1)
		c.Assert(accessible, Equals, false, Commentf("Volume %s is still available", s.volName1))
	}

	misc.LogTestEnd(c.TestName())
}
