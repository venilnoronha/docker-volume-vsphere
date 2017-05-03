# Supporting shared volumes with vSphere Docker Volumes


## Requirements
* Developer should able to use externally managed NFS share as “shared docker volume” using “vsphere” volume driver.
* This allows them to manage NFS directories as docker volumes using Docker APIs, Compose etc 
* Extend the vSphere Docker Volume plugin to support shared volumes
* Volumes are accessible on multiple docker hosts running on  multiple ESX hosts in a Docker swarm
* Allow user to specify what remote dirs are managed by the plugin
* Support all current docker volume plugin API for shared volumes

##  Assumptions
* Administrator sets up shares on NFS server(s) and ensures Docker Hosts has permissions to mount and modify the remote dirs.
* Developer responsible for (one-time) plugin configuration.
* 


## vSphere Docker Volume Plugin Drivers – User experience

  1. Setup
      a. Set up Plugin Config
      b. Restart plugin
  2. Using shared volumes
      a. docker volume create …
      b. docker volume list …
      c. docker volume inspect …
      d. docker run …
      e. docker volume rm …


# vSphere Docker Volume Plugin Drivers - Background

* Docker Volume Plugin supports a framework to implement storage specific drivers
* Drivers implement a common interface
   - Supports the volume plugin API of Docker volume plugins
   - Mount, Unmount, Get, Capabilities, Create, Remove
* Selection of driver via plugin configuration
* Easily extendable to any storage backing for container volumes
* A new driver can be written to support a specific storage type
* Presently, two drivers are supported
    -  Provide VMDK backed Docker volumes on platform specific storage
    - vSphere – provide Docker volumes vSAN, VVOL, VMFS, NFS
    - Photon – vSAN
* Drivers work with endpoints (currently local-ESX and Photon) to manage volumes
    - vSphere – works with the local ESX host to provision VMDKs
    - Photon – works with a Photon Controller endpoint to provision VMDKs


# vSphere-cloud driver

* Add a new “vsphere-cloud” (or better name) driver
* Implements the driver interface to provision “volumes” on a network mounted endpoints - NFS/EFS(phase-2)/CIFS(phase-2)
* Network share/FS coordinates are specified via aconfiguration to the driver at startup and later at volume create
* A single instance of the driver (plugin) supports multiple shares/FS

Note:
1. EFS support is enabled as a result of using NFS 4.0/4.1 to access the EFS instance. How authentication is done will
be addressed in a separate spec.
2. Docker allows passing secrets to plugins and as such username/passwords can be passed to
the plugin to handle authentication with EFS.

## Alternative - Support shared volumes as a common module
* Support shared volumes as a module used by both the vSphere and Photon drivers

### Pros
* Retains the functionality within the existing drivers - user doesn't need to specify a new driver
* Extends the scope of either driver to support more backends as sources for volumes - VMDK(vSphere or Photon), Networked FS/shares, VVOLs (later?)

### Cons
* Docker volumes with NFS are not related to VMDK based volumes and may confuse the user
* Volumes with the same syntax (volume@datastore) can procduce vmdk backed volumes or NFS volumes depending on the datastore used
     - User can determine volune type via "docker volume inspect" (any of VMDK, NFS, EFS)
     - Add type of volume to data returned on Get() API of the driver

# vSphere-cloud driver - Configuration

* Network share/FS configuration
* Two ways of supplying shares/FS to manage by the plugin
* Static - User provides a configuration at plugin startup
* Dynamic - “docker volume create –driver=vsphere-cloud –o “<network FS config options>”, user provides driver specific options for the datastore to use
* Current managed plugin has its configuration located in the host – /etc/docker-volume-vsphere.conf
* Existing config is modified to allow users to specify remote dirs used by the plugin
* Or create a new configuration passed to the plugin on startup
* Existing config is modified to include the location of the config that lists the remote dirs.
* Either approach is ok to use and the latter is preferred, allowing the user to use/replace a configuration of remote dirs without much editing of the plugin config.
* Plugin doesn’t keep any extra state of datastores or volumes in memory
* Add new “RemoteDirs” (or better name) entry as below
* Or create a /etc/remotedirs.conf (or other name) and set RetmoteDirs to that path.
* /etc/docker-volume-vsphere.conf

{
"MaxLogAgeDays": 28,
"MaxLogSizeMb": 100,
"LogPath": "/var/log/docker-volume-vsphere.log",
"LogLevel": "debug",
“RemoteDirs”: {
     “<dir-A>” :   {
          “addr”: <IP>,
          “args: <mount args>,
          “path”: <remote path to mount>,
          “type”: <FS type – nfs3, nfs4, efs, cifs, etc>,
          “vol-path”: <path name to locate volumes>
     },
     “<dir-B>” :   {
          ……..
     },
  }
  }


# vSphere-cloud driver – directory configuration and placement

* “RemoteDirs”:  Map of remote dirs.
* “<dir-labelA>” – must be unique and user defined and identifies a specific remote dir.
* “addr” – IP or fully qualified host name of server
* “args – args to mount remote dir.
* “path” – path on server to mount
* “type” – FS type of remote dir.
* “vol-path” – relative path that is a default location on this remote dir. to locate volumes (see volume groups later), if not specified then volumes are located under path above. Allows user to separate multiple stacks using volumes on a single remote dir

• Dirs are mounted only when user refers a remote dir when creating or attaching a volume
• Unique labels for remote dirs
• Assigned by the user and identifies a remote dir uniquely on a host
• Selects a remote dir to create volumes
• Using the same syntax as today – docker volume create –driver=vsphere-cloud –name=cloud-vol@dir_label …..
• Creates the volume under /mnt/vmdk/dir_label/cloud_vol
• The unique label is needed only if multiple remote dirs have been specified in the configuration
• Plugin defaults to the one specified remote dir otherwise
• There is no explicitly identified default remote dir to use if user doesn’t specify one


# Mounting Remote Directories

## Option #1
– Plugin is provided the share/FS details via configuration
• IP address of the server, path on server to mount, user/password, protocol, etc.
– Plugin mounts the FS on a path (under /mnt/vmdk) that it uses to manage all datastores - /mnt/vmdk

## Option #2
– User mounts the share/FS on the VM and provides the path to the plugin via the config file or dynamically at docker volume create time
– Plugin manages the provided path to create the volumes

## Option #1 is preferred

### Pros
* Admin actions are reduced - doesn’t need to manage the remote directories on each host in a swarm
* Plugin mounts remote dirs. as needed and only if volumes are referenced on those

### Cons
* Plugin needs to keep remote-dir configuration (in-memory) and possibly manage persistence


# Handling remote dir configuration

* On startup, plugin reads config and creates an in-memory map of datastores listed in the config
* See sample config for what data is specified to mount a network share/FS
* Each remote dir is identified by a unique label
* User can provide a new remote dir configuration (via volume create opts) at volume creation
* Plugin adds new the new dir to its in-memory map
* User can refer the new dir via its unique label in subsequent volume ops
* Does plugin need to persist dynamic entries in the config?
* User identifies volumes by specifying <volume-name>@<remote-dir>


# Volumes

* Provisioned as a folder on the named network FS
* /mnt/vmdk/<remote—dir-label>/<vol-path>/<vol-name>/
* Volume create mkdir’s the volume name folder if not already present

## Volume meta-data
* Plugin maintains no in-memory state for either volumes or remote dirs. (except the configuration)
* Volume met-data listed below are generated when requested via a Get() from Docker
* Location – remote-dir, path within remote-dir
* Type - volume type (NFS, EFS)
* Capacity used in FS blocks
* Current usage count
* Create date/time
* Modified time
* Access time
* Access permissions


# Volume options and Tenancy

## Volume create options
   - Access perms, volume group, remote-dir (see remote dir config)
## Tenancy
* User manages multi-tenancy
* User controls which VMs have access to what datastores to implement multiple tenants
* Remote dirs configuration determine what datastores are visible per docker host

# Protocol drivers

* Similar to ESX driver that VMDK driver uses to handle the interface to the ESX (via VMCI)
* Protocol specific drivers will be defined for the FS types supported – NFS/EFS/CIFS
   - Note: EFS uses the NFS v4.1 protocol and will be supported in phase-2
* API similar to the existing vmdkops
   - Create, Attach, Detach, List, Get, Remove, InitFS (to initialize for a specific network FS instance), etc.
* To start with one driver that supports NFS 4.0/4.1 will be supported (covers NFS/EFS shares)
* Support for EFS and CIFS will be handled in phase-2

# Data mover APIs for cloudops – phase 2 of shared volumes
* Data mover APIs allow user to specify backing for volumes and volume groups
* Individual volumes and volume groups can be migrated or replicated to backing datastores
* Any datastore in the config can be specified as a backing for another datastore

