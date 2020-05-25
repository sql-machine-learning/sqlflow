#!/bin/bash

# Copyright 2020 The SQLFlow Authors. All rights reserved.
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# (TODO:lhw) Put below things in usage and make this script more 'standalone'.
#
# This script is totally optional, it guesses whether we are in China.
# It will do nothing if the test fails, otherwise it will export the
# WE_ARE_IN_CHINA=true evn variable and do other things the user directed
# in the script param.

# Get the very small tool to figure out if we are in China
# This package can't be installed by apt-get
# c.f. https://unixmen.com/find-fastest-mirror-debian-derivatives/
if ! which netselect >/dev/null; then
    wget --progress=bar -O netselect.deb 'http://ftp.debian.org/debian/pool/main/n/netselect/netselect_0.3.ds1-28+b1_amd64.deb'
    sudo dpkg -i netselect.deb
    rm netselect.deb
fi

cn_mirrors=(
    mirrors.aliyun.com
    mirrors.ustc.edu.cn
    mirrors.163.com
)
en_mirrors=(
    archive.ubuntu.com
	ftp.de.debian.org
    packages.debian.org
)

function get_speed_score() {
    sudo netselect ${@} | awk '{print $1}'
}

echo "Guessing if we are in China..."
if [[ $(get_speed_score ${cn_mirrors[@]}) -lt $(get_speed_score ${en_mirrors[@]} ) ]]; then
    echo "Assume we are in China."
    export WE_ARE_IN_CHINA=true
fi

if [[ ! "${WE_ARE_IN_CHINA}" ]]; then
    exit 0
fi

function set_apt_mirror() {
    sudo cat <<EOF >/etc/apt/sources.list
deb http://mirrors.aliyun.com/ubuntu/ bionic main restricted universe multiverse
deb http://mirrors.aliyun.com/ubuntu/ bionic-security main restricted universe multiverse
deb http://mirrors.aliyun.com/ubuntu/ bionic-updates main restricted universe multiverse
deb http://mirrors.aliyun.com/ubuntu/ bionic-proposed main restricted universe multiverse
deb http://mirrors.aliyun.com/ubuntu/ bionic-backports main restricted universe multiverse
deb-src http://mirrors.aliyun.com/ubuntu/ bionic main restricted universe multiverse
deb-src http://mirrors.aliyun.com/ubuntu/ bionic-security main restricted universe multiverse
deb-src http://mirrors.aliyun.com/ubuntu/ bionic-updates main restricted universe multiverse
deb-src http://mirrors.aliyun.com/ubuntu/ bionic-proposed main restricted universe multiverse
deb-src http://mirrors.aliyun.com/ubuntu/ bionic-backports main restricted universe multiverse
EOF
	sudo apt-get -qq update
}

function set_go_proxy() {
    # other option: export GOPROXY=https://mirrors.aliyun.com/goproxy/
	export GOPROXY=https://goproxy.cn
}

function set_maven_repo() {
	mirror="<mirrors>
      <mirror>
        <id>alimaven</id>
        <mirrorOf>central</mirrorOf>
        <name>aliyun maven</name>
        <url>http://maven.aliyun.com/nexus/content/repositories/central/</url>
      </mirror>
	</mirrors>"
	mkdir -p "$HOME/.m2"
	settings="$HOME/.m2/settings.xml"
	if [[ ! -f "${settings}" ]]; then
		cat <<EOF > "${settings}"
<?xml version="1.0" encoding="UTF-8"?>
<settings xmlns="http://maven.apache.org/SETTINGS/1.0.0"
    xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
    xsi:schemaLocation="http://maven.apache.org/SETTINGS/1.0.0 http://maven.apache.org/xsd/settings-1.0.0.xsd">
	${mirror}
</settings>
EOF
	else
		mirror=`echo "${mirror}" | sed -e 's/"/\\"/g' -e 's^\\/^\\\\/^g'`
		mirror=${mirror//$'\n'/'\n'}
		if grep -q -o "<mirrors>" "${settings}"; then
			sed -i "/<mirrors>/,/<\/mirrors>/c${mirror}" "${settings}"
		else
			sed -i "s^</settings>^${mirror}\n&^" "${settings}"
		fi
	fi
}

args=`getopt -o "" -l"set-apt-mirror,set-go-proxy,set-maven-repo" -- "$@"`
eval set -- "${args}"
while true; do
    case "$1" in
		--set-apt-mirror)
		set_apt_mirror
		shift
		;;
		--set-go-proxy)
		set_go_proxy
		shift
		;;
		--set-maven-repo)
		set_maven_repo
		shift
		;;
		--)
		shift
		break
	esac
done

