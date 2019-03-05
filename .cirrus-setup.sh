#!/bin/sh -x
pwd
env

# Create the directory where we're going to install binaries.
mkdir -p    ${GOPATH}/bin

# Move to a directory that fits our repository name, relative to $GOPATH.
mkdir -p    ${RUNDIR}
rmdir       ${RUNDIR}
cp -a `pwd` ${RUNDIR}
cd ${RUNDIR}

# Install distro-provided dependencies.
case `uname -o` in
    *Linux*)
        . /etc/os-release
        case "$ID":"$VERSION_ID" in
        centos:7*)
            rpm -Uvh https://dl.fedoraproject.org/pub/epel/7/x86_64/Packages/e/epel-release-7-11.noarch.rpm
            yum -y install which make bats btrfs-progs btrfs-progs-devel e2fsprogs xfsprogs device-mapper-devel ostree ostree-devel git curl gcc bzip2
            ;;
        fedora:*)
            dnf -y install which make bats btrfs-progs btrfs-progs-devel e2fsprogs xfsprogs device-mapper-devel ostree ostree-devel git curl gcc bzip2
            ;;
        ubuntu:*)
            apt-get update
            apt-get -y install which make bats btrfs-tools libdevmapper-dev ostree libostree-dev git curl gcc bzip2
            ;;
        esac
        ;;
esac

# Download gimme and use it to get the current version of the Go compiler.
curl -o ${GOPATH}/bin/gimme https://raw.githubusercontent.com/travis-ci/gimme/master/gimme
chmod +x ${GOPATH}/bin/gimme
gimme -V
eval `gimme`

# Install dependencies that we build from source.
make install.tools
