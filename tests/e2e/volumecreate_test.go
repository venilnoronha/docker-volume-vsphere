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

// This test is going to cover various volume creation test cases

// +build runonce runoncewin

package e2e

import (
	"strings"
	"sync"

	dockerclicon "github.com/vmware/docker-volume-vsphere/tests/constants/dockercli"
	"github.com/vmware/docker-volume-vsphere/tests/utils/dockercli"
	"github.com/vmware/docker-volume-vsphere/tests/utils/inputparams"
	"github.com/vmware/docker-volume-vsphere/tests/utils/misc"
	"github.com/vmware/docker-volume-vsphere/tests/utils/verification"

	. "gopkg.in/check.v1"
)

type VolumeCreateTestSuite struct {
	config     *inputparams.TestConfig
	volumeList []string
}

func (s *VolumeCreateTestSuite) SetUpSuite(c *C) {
	s.config = inputparams.GetTestConfig()
	if s.config == nil {
		c.Skip("Unable to retrieve test config, skipping volume create tests")
	}
}

func (s *VolumeCreateTestSuite) TearDownTest(c *C) {
	volList := strings.Join(s.volumeList, " ")

	if volList != "" {
		out, err := dockercli.DeleteVolume(s.config.DockerHosts[0], volList)
		c.Assert(err, IsNil, Commentf(out))
	}

	// clean the list of volumes created
	s.volumeList = s.volumeList[:0]
}

var _ = Suite(&VolumeCreateTestSuite{})

// create volume and do valid/invalid assertion
func (s *VolumeCreateTestSuite) createVolCheck(name, option string, valid bool, c *C) {
	var out string
	var err error

	if option == "" {
		out, err = dockercli.CreateVolume(s.config.DockerHosts[0], name)
	} else {
		out, err = dockercli.CreateVolumeWithOptions(s.config.DockerHosts[0], name, option)
	}

	// if creation is successful, add it to the list so that it gets cleaned later
	if err == nil {
		s.volumeList = append(s.volumeList, name)
	}

	if valid {
		// positive test case
		c.Assert(err, IsNil, Commentf(out))
	} else {
		// negative test case
		c.Assert(err, Not(IsNil), Commentf(out))
		c.Assert(strings.HasPrefix(out, dockerclicon.ErrorVolumeCreate), Equals, true, Commentf(out))
	}
}

// loop over volume names to parallely create volumes
func (s *VolumeCreateTestSuite) parallelCreateByName(names []string, valid bool, c *C) {
	var wg sync.WaitGroup
	for _, volName := range names {
		wg.Add(1)
		go func(name string) {
			defer wg.Done()
			s.createVolCheck(name, "", valid, c)
		}(volName)
	}
	wg.Wait()
}

// loop over volume options to parallely create volumes
func (s *VolumeCreateTestSuite) parallelCreateByOption(options []string, valid bool, c *C) {
	var wg sync.WaitGroup
	for _, volOption := range options {
		wg.Add(1)
		go func(option string) {
			defer wg.Done()
			volName := inputparams.GetUniqueVolumeName("option")
			s.createVolCheck(volName, option, valid, c)
		}(volOption)
	}
	wg.Wait()
}

func (s *VolumeCreateTestSuite) accessCheck(hostIP string, volList []string, c *C) {
	isAvailable := verification.CheckVolumeListAvailability(hostIP, volList)
	c.Assert(isAvailable, Equals, true, Commentf("Volume %s is not available after creation", volList))
}

// Valid volume names test
func (s *VolumeCreateTestSuite) TestValidName(c *C) {
	misc.LogTestStart(c.TestName())

	s.parallelCreateByName(s.validVolNames(), true, c)
	s.accessCheck(s.config.DockerHosts[0], s.volumeList, c)

	misc.LogTestEnd(c.TestName())
}

// Invalid volume names test
func (s *VolumeCreateTestSuite) TestInvalidName(c *C) {
	misc.LogTestStart(c.TestName())

	s.parallelCreateByName(invalidVolNameList, false, c)

	misc.LogTestEnd(c.TestName())
}

// Valid volume creation options
// 1. size 10gb
// 2. disk format (thin, zeroedthick, eagerzeroedthick)
// 3. attach-as (persistent, independent_persistent)
// 4. fstype ext4 for linux / ntfs for windows
// 5. access (read-write, read-only)
// 6. clone-from valid volume
func (s *VolumeCreateTestSuite) TestValidOptions(c *C) {
	misc.LogTestStart(c.TestName())

	// Need a valid volume source to test clone-from option
	cloneSrcVol := inputparams.GetUniqueVolumeName("clone_src")
	s.volumeList = append(s.volumeList, cloneSrcVol)
	out, err := dockercli.CreateVolume(s.config.DockerHosts[0], cloneSrcVol)
	c.Assert(err, IsNil, Commentf(out))

	validVolOpts := []string{
		" -o size=10gb",
		" -o diskformat=zeroedthick",
		" -o diskformat=thin",
		" -o diskformat=eagerzeroedthick",
		" -o attach-as=independent_persistent",
		" -o attach-as=persistent",
		" -o fstype=" + validFstype,
		" -o access=read-only",
		" -o access=read-write",
		" -o clone-from=" + cloneSrcVol,
	}

	s.parallelCreateByOption(validVolOpts, true, c)
	s.accessCheck(s.config.DockerHosts[0], s.volumeList, c)

	misc.LogTestEnd(c.TestName())
}

// Invalid volume create operations
// 1. Wrong disk formats
// 2. Wrong volume sizes
// 3. Wrong fs types
// 4. Wrong access types
// 5. Unavailable clone source
func (s *VolumeCreateTestSuite) TestInvalidOptions(c *C) {
	misc.LogTestStart(c.TestName())

	invalidVolOpts := []string{
		" -o diskformat=zeroedthickk",
		" -o diskformat=zeroedthick,thin",
		" -o size=100mbb",
		" -o size=100gbEE",
		" -o sizes=100mb",
		" -o fstype=xfs_ext",
		" -o access=read-write-both",
		" -o access=write-only",
		" -o access=read-write-both",
		" -o clone-from=IDontExist",
	}

	s.parallelCreateByOption(invalidVolOpts, false, c)

	misc.LogTestEnd(c.TestName())
}
