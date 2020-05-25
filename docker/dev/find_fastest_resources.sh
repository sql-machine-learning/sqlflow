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

# This script find kinds of fastest mirrors for speeding up
# our build process. You should use this script as follows:
#
# Example: set apt-get source
# source find_fastest_resources.sh
# sudo (find_fastest_apt_source > /etc/apt/sources.list)
#
# Supported resources:
# function find_fastest_apt_source() echos fastest apt-get sources
# function find_fastest_maven_repo() echos fastest maven repo
# function find_fastest_go_proxy()   echos fastest go proxy
#


# Accept a url and output it's domain
function get_domain_from_url {
	# cut extract the domain, domain is the third field split by '/'
	echo "$1" | cut -d '/' -f 3
}

# Find fastest url, params should be urls separated by space
function find_fastest_url() {
	local speed=99999.9
	for i in $@; do
		local domain=$(get_domain_from_url "$i")
		# c.f. https://stackoverflow.com/a/9634982/724872
		# redirect log output to stderr
		echo "Testig speed of $domain ..." >&2
		local cur_speed=$(ping -c 4 "$domain" | tail -1| awk '{print $4}' | cut -d '/' -f 2)
		echo "$cur_speed" >&2

		# c.f. https://stackoverflow.com/a/31087503/724872
		if (( $(echo "$cur_speed < $speed" | bc -l) )); then
			local best_domain="$i"
			speed="$cur_speed"
		fi
	done
	echo $best_domain
}

# Find fastest apt-get source, you can add mirrors in the 'apt_sources'
function find_fastest_apt_source() {
	# Define a list of mirrors without using Bash arrays.
	# c.f. https://stackoverflow.com/a/23930212/724872
	read -r -d '' apt_sources <<- EOM
		http://mirrors.aliyun.com
		http://mirrors.ustc.edu.cn
		http://mirrors.163.com
		http://archive.ubuntu.com
	EOM

	# Find the fastest APT source using ping.
	local best_apt_source=$(find_fastest_url $apt_sources)

	# The default Ubuntu version is 18.04, code named bionic.
	local ubuntu_codename=${ubuntu_codename-"bionic"}

	# Write APT source lists.
	cat <<-EOF
		deb $best_apt_source/ubuntu/ $ubuntu_codename main restricted universe multiverse
		deb $best_apt_source/ubuntu/ $ubuntu_codename-security main restricted universe multiverse
		deb $best_apt_source/ubuntu/ $ubuntu_codename-updates main restricted universe multiverse
		deb $best_apt_source/ubuntu/ $ubuntu_codename-proposed main restricted universe multiverse
		deb $best_apt_source/ubuntu/ $ubuntu_codename-backports main restricted universe multiverse
		deb-src $best_apt_source/ubuntu/ $ubuntu_codename main restricted universe multiverse
		deb-src $best_apt_source/ubuntu/ $ubuntu_codename-security main restricted universe multiverse
		deb-src $best_apt_source/ubuntu/ $ubuntu_codename-updates main restricted universe multiverse
		deb-src $best_apt_source/ubuntu/ $ubuntu_codename-proposed main restricted universe multiverse
		deb-src $best_apt_source/ubuntu/ $ubuntu_codename-backports main restricted universe multiverse
	EOF
}

function find_fastest_maven_repo() {
	# c.f. https://unix.stackexchange.com/questions/353076
	# we can indent EOM wiht tab if we use the "<<-" form
	read -r -d '' maven_repos <<-EOM
		https://repo1.maven.org/maven2/
		https://maven.aliyun.com/repository/central
	EOM

	local best_maven_repo=$(find_fastest_url ${maven_repos})
	local domain=$(get_domain_from_url $best_maven_repo)

	cat <<-EOF
	<?xml version="1.0" encoding="UTF-8"?>
	<settings xmlns="http://maven.apache.org/SETTINGS/1.0.0"
	    xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
	    xsi:schemaLocation="http://maven.apache.org/SETTINGS/1.0.0 http://maven.apache.org/xsd/settings-1.0.0.xsd">
	    <mirrors>
	        <mirror>
	            <id>${domain}</id>
	            <mirrorOf>central</mirrorOf>
	            <name>${best_maven_repo}</name>
	            <url>${domain}</url>
	        </mirror>
	    </mirrors>
	</settings>
	EOF
}

function find_fastest_go_proxy() {
	# This is not really a proxy, we just want to compare the access speed with other
	# proxies like 'proxygo.cn', if it is faster, we do not even need a proxy
	local direct_access_url="https://pkg.go.dev/"
	read -r -d '' go_proxies <<-EOM
		${direct_access_url}
		https://goproxy.cn/,direct
	EOM

	local best_url=$(find_fastest_url $go_proxies)

	if [[ "$best_url" == "$direct_access_url" ]]; then
		# We do not need a proxy if we can access https://pkg.go.dev/ fast
		echo ""
	else
		echo $best_url
	fi
}

