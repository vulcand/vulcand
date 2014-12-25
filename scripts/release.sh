#!/bin/sh -e

# how to release stuff:
#
# launch it in docker 
# sudo docker run -i -t -v /tmp:/tmp golang:1.3.3-onbuild /bin/bash
# cd /tmp && curl https://raw.githubusercontent.com/mailgun/vulcand/master/scripts/release.sh > relase.sh
# bash release.sh v0.8.0-alpha
# /tmp/releases 

VER=$1
PROJ="vulcand"
RELEASE_DIR="/tmp/release"
PROJ_DIR=${GOPATH}/src/github.com/mailgun/${PROJ}

if [ -z "$1" ]; then
	echo "Usage: ${0} VERSION" >> /dev/stderr
	exit 255
fi

set -u

function setup_env {
	local proj=${1}
	local ver=${2}
    local proj_dir=${3}

    mkdir -p $(dirname $proj_dir)

	if [ ! -d ${proj_dir} ]; then
		git clone https://github.com/mailgun/${proj} ${proj_dir}
	fi

	pushd ${proj_dir}
		git checkout master
		git fetch --all
		git reset --hard origin/master
		git checkout $ver
	popd
}


function package {
	local target=${1}
	local srcdir=${2}

	local ccdir="${srcdir}/${GOOS}_${GOARCH}"
	if [ -d ${ccdir} ]; then
		srcdir=${ccdir}
	fi
	for bin in vulcand vctl vbundle; do
		cp ${srcdir}/${bin} ${target}
	done

	cp vulcand/README.md ${target}/README.md
}

function main {
	setup_env ${PROJ} ${VER} ${PROJ_DIR}

	for os in linux; do
		export GOOS=${os}
		export GOARCH="amd64"

		pushd $PROJ_DIR
			make install
		popd

		TARGET="${RELEASE_DIR}/vulcand-${VER}-${GOOS}-${GOARCH}"
		mkdir -p ${TARGET}
		package ${TARGET} "${GOPATH}/bin"

		if [ ${GOOS} == "linux" ]; then
			tar cfz ${TARGET}.tar.gz ${TARGET}
			echo "Wrote ${TARGET}.tar.gz"
		else
			zip -qr ${TARGET}.zip ${TARGET}
			echo "Wrote ${TARGET}.zip"
		fi
	done
}

main
