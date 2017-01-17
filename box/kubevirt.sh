#!/bin/bash
set -x

# Only build master
unset VAGRANT_NUM_NODES
unset VAGRANT_CACHE_DOCKER
unset VAGRANT_CACHE_RPM
# Create a new image
# (cd .. && vagrant destroy && vagrant up && cluster/sync.sh && vagrant halt)
# (cd .. && vagrant up && cluster/sync.sh && vagrant halt)

# Download script for building a libvirt box
curl -LkO https://raw.githubusercontent.com/vagrant-libvirt/vagrant-libvirt/master/tools/create_box.sh

# Create the box
sudo sh create_box.sh /var/lib/libvirt/images/kubevirt_master.img kubevirt.box
sudo chown `whoami` kubevirt.box
