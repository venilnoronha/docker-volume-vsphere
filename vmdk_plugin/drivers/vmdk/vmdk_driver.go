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

package vmdk

//
// VMWare vSphere Docker Data Volume plugin.
//
// Provide support for --driver=vsphere in Docker, when Docker VM is running under ESX.
//
// Serves requests from Docker Engine related to VMDK volume operations.
// Depends on vmdk-opsd service to be running on hosting ESX
// (see ./esx_service)
///

import (
	"fmt"
	"path/filepath"
	"sync"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/go-plugins-helpers/volume"
	"github.com/vmware/docker-volume-vsphere/vmdk_plugin/drivers/vmdk/vmdkops"
	"github.com/vmware/docker-volume-vsphere/vmdk_plugin/utils/fs"
	"github.com/vmware/docker-volume-vsphere/vmdk_plugin/utils/plugin_utils"
	"github.com/vmware/docker-volume-vsphere/vmdk_plugin/utils/refcount"
)

const version = "vSphere Volume Driver v0.5"

// VolumeDriver - VMDK driver struct
type VolumeDriver struct {
	useMockEsx    bool
	ops           vmdkops.VmdkOps
	refCounts     *refcount.RefCountsMap
	mountIDtoName map[string]string // map of mountID -> full volume name
}

var mountRoot string

// NewVolumeDriver creates Driver which to real ESX (useMockEsx=False) or a mock
func NewVolumeDriver(port int, useMockEsx bool, mountDir string, driverName string) *VolumeDriver {
	var d *VolumeDriver

	vmdkops.EsxPort = port
	mountRoot = mountDir

	if useMockEsx {
		d = &VolumeDriver{
			useMockEsx: true,
			ops:        vmdkops.VmdkOps{Cmd: vmdkops.NewMockCmd()},
			refCounts:  refcount.NewRefCountsMap(),
		}
	} else {
		d = &VolumeDriver{
			useMockEsx: false,
			ops: vmdkops.VmdkOps{
				Cmd: vmdkops.EsxVmdkCmd{
					Mtx: &sync.Mutex{},
				},
			},
			refCounts: refcount.NewRefCountsMap(),
		}
	}

	d.mountIDtoName = make(map[string]string)
	d.refCounts.Init(d, mountDir, driverName)

	log.WithFields(log.Fields{
		"version":  version,
		"port":     vmdkops.EsxPort,
		"mock_esx": useMockEsx,
	}).Info("Docker VMDK plugin started ")

	return d
}

// VolumesInRefMap - get list of volumes names from refmap
// names are in format volume@datastore
func (d *VolumeDriver) VolumesInRefMap() []string {
	return d.refCounts.GetVolumeNames()
}

// In following three operations on refcount, if refcount
// map hasn't been initialized, return 1 to prevent detach and remove.

// Return the number of references for the given volume
func (d *VolumeDriver) getRefCount(vol string) uint {
	if d.refCounts.IsInitialized() != true {
		return 1
	}
	return d.refCounts.GetCount(vol)
}

// Increment the reference count for the given volume
func (d *VolumeDriver) incrRefCount(vol string) uint {
	if d.refCounts.IsInitialized() != true {
		return 1
	}
	return d.refCounts.Incr(vol)
}

// Decrement the reference count for the given volume
func (d *VolumeDriver) decrRefCount(vol string) (uint, error) {
	if d.refCounts.IsInitialized() != true {
		return 1, nil
	}
	return d.refCounts.Decr(vol)
}

// Returns the given volume mountpoint
func getMountPoint(volName string) string {
	return filepath.Join(mountRoot, volName)
}

// Get info about a single volume
func (d *VolumeDriver) Get(r volume.Request) volume.Response {
	status, err := d.GetVolume(r.Name)
	if err != nil {
		return volume.Response{Err: err.Error()}
	}
	mountpoint := getMountPoint(r.Name)
	return volume.Response{Volume: &volume.Volume{Name: r.Name,
		Mountpoint: mountpoint,
		Status:     status}}
}

// List volumes known to the driver
func (d *VolumeDriver) List(r volume.Request) volume.Response {
	volumes, err := d.ops.List()
	if err != nil {
		return volume.Response{Err: err.Error()}
	}
	responseVolumes := make([]*volume.Volume, 0, len(volumes))
	for _, vol := range volumes {
		mountpoint := getMountPoint(vol.Name)
		responseVol := volume.Volume{Name: vol.Name, Mountpoint: mountpoint}
		responseVolumes = append(responseVolumes, &responseVol)
	}
	return volume.Response{Volumes: responseVolumes}
}

// GetVolume - return volume meta-data.
func (d *VolumeDriver) GetVolume(name string) (map[string]interface{}, error) {
	return d.ops.Get(name)
}

// MountVolume - Request attach and them mounts the volume.
// Actual mount - send attach to ESX and do the in-guest magic
// Returns mount point and  error (or nil)
func (d *VolumeDriver) MountVolume(name string, fstype string, id string, isReadOnly bool, skipAttach bool) (string, error) {
	mountpoint := getMountPoint(name)

	// First, make sure  that mountpoint exists.
	err := fs.Mkdir(mountpoint)
	if err != nil {
		log.WithFields(
			log.Fields{"name": name, "dir": mountpoint},
		).Error("Failed to make directory for volume mount ")
		return mountpoint, err
	}

	waitCtx, errWait := fs.DevAttachWaitPrep()
	if errWait != nil {
		log.WithFields(log.Fields{"name": name,
			"error": errWait}).Warning("Failed to initialize wait context, continuing however.. ")
	}

	if d.useMockEsx {
		dev, err := d.ops.RawAttach(name, nil)
		if err != nil {
			log.WithFields(log.Fields{"name": name, "error": err}).Error("Failed to attach volume ")
			return mountpoint, err
		}
		return mountpoint, fs.MountByDevicePath(mountpoint, fstype, string(dev[:]), false)
	}

	volDev, err := d.ops.Attach(name, nil)
	if err != nil {
		log.WithFields(log.Fields{"name": name, "error": err}).Error("Attach volume failed ")
		return mountpoint, err
	}

	if errWait != nil {
		fs.DevAttachWaitFallback()
		return mountpoint, fs.Mount(mountpoint, fstype, volDev, false)
	}

	fs.DevAttachWait(waitCtx, volDev)

	// May have timed out waiting for the attach to complete,
	// attempt the mount anyway.
	return mountpoint, fs.Mount(mountpoint, fstype, volDev, isReadOnly)
}

// UnmountVolume - Unmounts the volume and then requests detach
func (d *VolumeDriver) UnmountVolume(name string) error {
	mountpoint := getMountPoint(name)
	err := fs.Unmount(mountpoint)
	if err != nil {
		log.WithFields(
			log.Fields{"mountpoint": mountpoint, "error": err},
		).Error("Failed to unmount volume. Now trying to detach... ")
		// Do not return error. Continue with detach.
	}
	return d.ops.Detach(name, nil)
}

// private function that does the job of mounting volume in conjunction with refcounting
func (d *VolumeDriver) processMount(r volume.MountRequest) volume.Response {
	volumeInfo, err := plugin_utils.GetVolumeInfo(r.Name, "", d)
	if err != nil {
		log.Errorf("Unable to get volume info for volume %s. err:%v", r.Name, err)
		return volume.Response{Err: err.Error()}
	}
	r.Name = volumeInfo.VolumeName
	d.mountIDtoName[r.ID] = r.Name

	// If the volume is already mounted , just increase the refcount.
	// Note: for new keys, GO maps return zero value, so no need for if_exists.
	refcnt := d.incrRefCount(r.Name) // save map traversal
	log.Debugf("volume name=%s refcnt=%d", r.Name, refcnt)
	if refcnt > 1 {
		log.WithFields(
			log.Fields{"name": r.Name, "refcount": refcnt},
		).Info("Already mounted, skipping mount. ")
		return volume.Response{Mountpoint: getMountPoint(r.Name)}
	}

	if plugin_utils.AlreadyMounted(r.Name, mountRoot) {
		log.WithFields(log.Fields{"name": r.Name}).Info("Already mounted, skipping mount. ")
		return volume.Response{Mountpoint: getMountPoint(r.Name)}
	}

	// get volume metadata if required
	volumeMeta := volumeInfo.VolumeMeta
	if volumeMeta == nil {
		if volumeMeta, err = d.ops.Get(r.Name); err != nil {
			d.decrRefCount(r.Name)
			return volume.Response{Err: err.Error()}
		}
	}

	fstype := fs.FstypeDefault
	isReadOnly := false
	if err != nil {
		d.decrRefCount(r.Name)
		return volume.Response{Err: err.Error()}
	}
	// Check access type.
	value, exists := volumeMeta["access"].(string)
	if !exists {
		msg := fmt.Sprintf("Invalid access type for %s, assuming read-write access.", r.Name)
		log.WithFields(log.Fields{"name": r.Name, "error": msg}).Error("")
		isReadOnly = false
	} else if value == "read-only" {
		isReadOnly = true
	}

	// Check file system type.
	value, exists = volumeMeta["fstype"].(string)
	if !exists {
		msg := fmt.Sprintf("Invalid filesystem type for %s, assuming type as %s.",
			r.Name, fstype)
		log.WithFields(log.Fields{"name": r.Name, "error": msg}).Error("")
		// Fail back to a default version that we can try with.
		value = fs.FstypeDefault
	}
	fstype = value

	mountpoint, err := d.MountVolume(r.Name, fstype, "", isReadOnly, false)
	if err != nil {
		log.WithFields(
			log.Fields{"name": r.Name, "error": err.Error()},
		).Error("Failed to mount ")

		refcnt, _ := d.decrRefCount(r.Name)
		if refcnt == 0 {
			log.Infof("Detaching %s - it is not used anymore", r.Name)
			d.ops.Detach(r.Name, nil) // try to detach before failing the request for volume
		}
		return volume.Response{Err: err.Error()}
	}

	return volume.Response{Mountpoint: mountpoint}
}

// prepareCreateOptions sets default options for create request r.
func (d *VolumeDriver) prepareCreateOptions(r volume.Request) error {
	if r.Options == nil {
		r.Options = make(map[string]string)
	}

	// Use default fstype if both fstype and clone-from are not specified.
	_, fstypeRes := r.Options["fstype"]
	_, cloneFromRes := r.Options["clone-from"]
	if !fstypeRes && !cloneFromRes {
		log.WithFields(log.Fields{"req": r}).Debugf("Setting fstype to %s ", fs.FstypeDefault)
		r.Options["fstype"] = fs.FstypeDefault
	}

	// Check whether the fstype filesystem is supported.
	if _, fstypeRes = r.Options["fstype"]; fstypeRes {
		err := fs.VerifyFSSupport(r.Options["fstype"])
		if err != nil {
			log.WithFields(log.Fields{"name": r.Name, "fstype": r.Options["fstype"],
				"error": err}).Error("Not supported ")
			return err
		}
	}
	return nil
}

// cloneFrom clones an existing volume.
func (d *VolumeDriver) cloneFrom(r volume.Request) volume.Response {
	errClone := d.ops.Create(r.Name, r.Options)
	if errClone != nil {
		log.WithFields(log.Fields{"name": r.Name, "error": errClone}).Error("Clone volume failed ")
		return volume.Response{Err: errClone.Error()}
	}
	return volume.Response{Err: ""}
}

// detach detaches a volume, or prints a warning log on failure.
func (d *VolumeDriver) detach(name string) error {
	errDetach := d.ops.Detach(name, nil)
	if errDetach != nil {
		log.WithFields(log.Fields{"name": name, "error": errDetach}).Warning("Detach volume failed ")
	}
	return errDetach
}

// remove removes a volume, or prints a warning log on failure.
func (d *VolumeDriver) remove(name string) error {
	errRemove := d.ops.Remove(name, nil)
	if errRemove != nil {
		log.WithFields(log.Fields{"name": name, "error": errRemove}).Warning("Remove volume failed ")
	}
	return errRemove
}

// detachAndRemove detaches a volume and then removes it, ignoring any errors.
func (d *VolumeDriver) detachAndRemove(name string) {
	d.detach(name)
	d.remove(name)
}

// No need to actually manifest the volume on the filesystem yet
// (until Mount is called).
// Name and driver specific options passed through to the ESX host

// Create creates a volume.
func (d *VolumeDriver) Create(r volume.Request) volume.Response {
	err := d.prepareCreateOptions(r)
	if err != nil {
		log.WithFields(log.Fields{"name": r.Name, "error": err}).Error("Failed to prepare options ")
		return volume.Response{Err: err.Error()}
	}

	// If cloning a existent volume, create and return
	if _, result := r.Options["clone-from"]; result {
		return d.cloneFrom(r)
	}

	errCreate := d.ops.Create(r.Name, r.Options)
	if errCreate != nil {
		log.WithFields(log.Fields{"name": r.Name, "error": errCreate}).Error("Create volume failed ")
		return volume.Response{Err: errCreate.Error()}
	}

	// Handle filesystem creation
	log.WithFields(log.Fields{"name": r.Name,
		"fstype": r.Options["fstype"]}).Info("Attaching volume and creating filesystem ")

	waitCtx, errWait := fs.DevAttachWaitPrep()
	if errWait != nil {
		log.WithFields(log.Fields{"name": r.Name,
			"error": errWait}).Warning("Failed to initialize wait context, continuing however.. ")
	}

	volDev, errAttach := d.ops.Attach(r.Name, nil)
	if errAttach != nil {
		log.WithFields(log.Fields{"name": r.Name,
			"error": errAttach}).Error("Attach volume failed, removing the volume ")
		d.remove(r.Name)
		return volume.Response{Err: errAttach.Error()}
	}

	if errWait != nil {
		fs.DevAttachWaitFallback()
	} else {
		// Wait for the attach to complete, may timeout
		// in which case we continue creating the file system.
		errAttachWait := fs.DevAttachWait(waitCtx, volDev)
		if errAttachWait != nil {
			log.WithFields(log.Fields{"name": r.Name,
				"error": errAttachWait}).Error("Could not find attached device, removing the volume ")
			d.detachAndRemove(r.Name)
			return volume.Response{Err: errAttachWait.Error()}
		}
	}

	errMkfs := fs.Mkfs(r.Options["fstype"], r.Name, volDev)
	if errMkfs != nil {
		log.WithFields(log.Fields{"name": r.Name,
			"error": errMkfs}).Error("Create filesystem failed, removing the volume ")
		d.detachAndRemove(r.Name)
		return volume.Response{Err: errMkfs.Error()}
	}

	errDetach := d.ops.Detach(r.Name, nil)
	if errDetach != nil {
		log.WithFields(log.Fields{"name": r.Name, "error": errDetach}).Error("Detach volume failed ")
		return volume.Response{Err: errDetach.Error()}
	}

	log.WithFields(log.Fields{"name": r.Name,
		"fstype": r.Options["fstype"]}).Info("Volume and filesystem created ")
	return volume.Response{Err: ""}
}

// Remove - removes individual volume. Docker would call it only if is not using it anymore
func (d *VolumeDriver) Remove(r volume.Request) volume.Response {
	log.WithFields(log.Fields{"name": r.Name}).Info("Removing volume ")

	// Cannot remove volumes till plugin completely initializes (refcounting is complete)
	// because we don't know if it is being used or not
	if d.refCounts.IsInitialized() != true {
		msg := fmt.Sprintf(plugin_utils.PluginInitError+" Cannot remove volume=%s", r.Name)
		log.Error(msg)
		return volume.Response{Err: msg}
	}

	// Docker is supposed to block 'remove' command if the volume is used.
	if d.getRefCount(r.Name) != 0 {
		msg := fmt.Sprintf("Remove failure - volume is still mounted. "+
			" volume=%s, refcount=%d", r.Name, d.getRefCount(r.Name))
		log.Error(msg)
		return volume.Response{Err: msg}
	}

	err := d.ops.Remove(r.Name, r.Options)
	if err != nil {
		log.WithFields(
			log.Fields{"name": r.Name, "error": err},
		).Error("Failed to remove volume ")
		return volume.Response{Err: err.Error()}
	}

	return volume.Response{Err: ""}
}

// Path - give docker a reminder of the volume mount path
func (d *VolumeDriver) Path(r volume.Request) volume.Response {
	return volume.Response{Mountpoint: getMountPoint(r.Name)}
}

// Mount - Provide a volume to docker container - called once per container start.
// We need to keep refcount and unmount on refcount drop to 0
//
// The serialization of operations per volume is assured by the volume/store
// of the docker daemon.
// As long as the refCountsMap is protected is unnecessary to do any locking
// at this level during create/mount/umount/remove.
//
func (d *VolumeDriver) Mount(r volume.MountRequest) volume.Response {
	log.WithFields(log.Fields{"name": r.Name}).Info("Mounting volume ")

	// lock the state
	d.refCounts.StateMtx.Lock()
	defer d.refCounts.StateMtx.Unlock()

	// checked by refcounting thread until refmap initialized
	// useless after that
	d.refCounts.MarkDirty()

	return d.processMount(r)
}

// Unmount request from Docker. If mount refcount is drop to 0.
// Unmount and detach from VM
func (d *VolumeDriver) Unmount(r volume.UnmountRequest) volume.Response {
	log.WithFields(log.Fields{"name": r.Name}).Info("Unmounting Volume ")

	// lock the state
	d.refCounts.StateMtx.Lock()
	defer d.refCounts.StateMtx.Unlock()

	if d.refCounts.IsInitialized() != true {
		// if refcounting hasn't been succesful,
		// no refcounting, no unmount. All unmounts are delayed
		// until we succesfully populate the refcount map
		d.refCounts.MarkDirty()
		return volume.Response{Err: ""}
	}

	if fullVolName, exist := d.mountIDtoName[r.ID]; exist {
		r.Name = fullVolName
		delete(d.mountIDtoName, r.ID) //cleanup the map
	} else {
		volumeInfo, err := plugin_utils.GetVolumeInfo(r.Name, "", d)
		if err != nil {
			log.Errorf("Unable to get volume info for volume %s. err:%v", r.Name, err)
			return volume.Response{Err: err.Error()}
		}
		r.Name = volumeInfo.VolumeName
	}

	// if refcount has been succcessful, Normal flow
	// if the volume is still used by other containers, just return OK
	refcnt, err := d.decrRefCount(r.Name)
	if err != nil {
		// something went wrong - yell, but still try to unmount
		log.WithFields(
			log.Fields{"name": r.Name, "refcount": refcnt},
		).Error("Refcount error - still trying to unmount...")
	}
	log.Debugf("volume name=%s refcnt=%d", r.Name, refcnt)
	if refcnt >= 1 {
		log.WithFields(
			log.Fields{"name": r.Name, "refcount": refcnt},
		).Info("Still in use, skipping unmount request. ")
		return volume.Response{Err: ""}
	}

	// and if nobody needs it, unmount and detach
	err = d.UnmountVolume(r.Name)
	if err != nil {
		log.WithFields(
			log.Fields{"name": r.Name, "error": err.Error()},
		).Error("Failed to unmount ")
		return volume.Response{Err: err.Error()}
	}
	return volume.Response{Err: ""}
}

// Capabilities - Report plugin scope to Docker
func (d *VolumeDriver) Capabilities(r volume.Request) volume.Response {
	return volume.Response{Capabilities: volume.Capability{Scope: "global"}}
}
