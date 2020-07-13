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
# function find_fastest_apt_source()  echos fastest apt-get sources
# function find_fastest_maven_repo()  echos fastest maven repo
# function find_fastest_go_proxy()    echos fastest go proxy
# function find_fastest_docker_url()  echos fastest docker download url
# function find_fastest_docker_registry() echos fastest docker mirror url
# function find_fastest_pip_mirror()  echos fastest pip mirror config


# Returns the domain name of an URL.
function get_domain_from_url() {
    # cut extract the domain, domain is the third field split by '/'
    echo "$1" | cut -d '/' -f 3
}

# Find the fastest URL. The parameter consists of URLS separated by whitespace.
function find_fastest_url() {
    local speed=99999
    # shellcheck disable=SC2068
    for i in $@; do
        local domain
        domain=$(get_domain_from_url "$i")

        # c.f. https://stackoverflow.com/a/9634982/724872
        # redirect log output to stderr
        local cur_speed
        cur_speed=$(ping -c 4 -W 2 "$domain" | tail -1 \
                           | grep "/avg/" | awk '{print $4}'\
                           | cut -d '/' -f 2)
        cur_speed=${cur_speed:-99999}
        cur_speed=${cur_speed/.*/}

        # c.f. https://stackoverflow.com/a/31087503/724872
        if [[ $cur_speed -lt $speed ]]; then
            local best_domain="$i"
            speed="$cur_speed"
        fi
    done
    echo "$best_domain"
}

# Find fastest apt-get source, you can add mirrors in the 'apt_sources'
function find_fastest_apt_source() {
    # We need to specify \t as the terminate indicator character; otherwise, the
    # read command would return an non-zero exit code.
    read -r -d '\t' apt_sources <<EOM
http://mirrors.163.com
http://archive.ubuntu.com
\t
EOM

    # Find the fastest APT source using ping.
    local fastest
    # shellcheck disable=SC2086
    fastest=$(find_fastest_url $apt_sources)/ubuntu/

    # The default Ubuntu version is 18.04, code named bionic.
    local codename
    codename=${ubuntu_codename-"bionic"}

    # Write APT source lists.
    cat <<EOF
deb $fastest $codename main restricted universe multiverse
deb $fastest $codename-security main restricted universe multiverse
deb $fastest $codename-updates main restricted universe multiverse
deb $fastest $codename-proposed main restricted universe multiverse
deb $fastest $codename-backports main restricted universe multiverse
deb-src $fastest $codename main restricted universe multiverse
deb-src $fastest $codename-security main restricted universe multiverse
deb-src $fastest $codename-updates main restricted universe multiverse
deb-src $fastest $codename-proposed main restricted universe multiverse
deb-src $fastest $codename-backports main restricted universe multiverse
EOF
}

function find_fastest_maven_repo() {
    read -r -d '\t' maven_repos <<EOM
https://repo1.maven.org/maven2/
https://maven-central.storage-download.googleapis.com/maven2/
https://maven.aliyun.com/repository/central
\t
EOM

    local best_maven_repo
    # shellcheck disable=SC2086
    best_maven_repo=$(find_fastest_url $maven_repos)
    local domain
    domain=$(get_domain_from_url "$best_maven_repo")

    cat <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<settings xmlns="http://maven.apache.org/SETTINGS/1.0.0"
    xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
    xsi:schemaLocation="http://maven.apache.org/SETTINGS/1.0.0 http://maven.apache.org/xsd/settings-1.0.0.xsd">
    <mirrors>
        <mirror>
            <id>$domain</id>
            <mirrorOf>central</mirrorOf>
            <name>$domain</name>
            <url>$best_maven_repo</url>
        </mirror>
    </mirrors>
</settings>
EOF
}

function find_fastest_go_proxy() {
    # This is not really a proxy, we just want to compare the access speed with other
    # proxies like 'proxygo.cn', if it is faster, we do not even need a proxy
    local default
    default="https://pkg.go.dev/"
    read -r -d '\t' go_proxies <<EOM
$default
https://goproxy.cn/,direct
\t
EOM

    local best
    # shellcheck disable=SC2086
    best=$(find_fastest_url $go_proxies)

    if [[ "$best" == "$default" ]]; then
        # We do not need a proxy if we can access https://pkg.go.dev/ fast
        echo ""
    else
        echo "$best"
    fi
}

# Find fastest docker download URL
# through which we get the install script
function find_fastest_docker_url() {
    read -r -d '\t' download_urls <<EOM
https://get.daocloud.io/docker
https://get.docker.com
\t
EOM
    # shellcheck disable=SC2086
    find_fastest_url $download_urls
}

# Find docker-ce install apt mirror
function find_fastest_docker_ce_mirror() {
    read -r -d '\t' download_urls <<EOM
https://mirrors.aliyun.com/docker-ce
https://download.docker.com
\t
EOM
    # shellcheck disable=SC2086
    find_fastest_url $download_urls
}

# Find fastest docker mirror url
function find_fastest_docker_registry() {
    local url="https://www.docker.com/"
    read -r -d '\t' mirror_urls <<EOM
$url
https://hub-mirror.c.163.com
https://registry.docker-cn.com
https://docker.mirrors.ustc.edu.cn
\t
EOM
    local best
    # shellcheck disable=SC2086
    best=$(find_fastest_url $mirror_urls)
    if [[ "$best" == "$url" ]]; then
        echo ""
    else
        echo "$best"
    fi
}

# Find pip mirror and echo a config if needed
function find_fastest_pip_mirror() {
    local url="https://pypi.org/"
    read -r -d '\t' mirror_urls <<EOM
$url
https://mirrors.aliyun.com/pypi/simple/
\t
EOM
    local best
    # shellcheck disable=SC2086
    best=$(find_fastest_url $mirror_urls)
    if [[ "$best" == "$url" ]]; then
        echo ""
    else
        cat <<EOF
[global]
index-url = https://mirrors.aliyun.com/pypi/simple/
[install]
trusted-host=mirrors.aliyun.com
EOF
    fi
}

# All find_xxx functions need ping and some needs bc.
function install_requirements_if_not() {
    install="false"
    # shellcheck disable=SC2230
    if ! which ping >/dev/null; then
        install="true"
    fi
    # shellcheck disable=SC2230
    if ! which bc >/dev/null; then
        install="true"
    fi

    if [[ "$install" == "true" ]]; then
        apt-get -qq update
        apt-get -qq install -y iputils-ping bc > /dev/null
    fi
}

function choose_fastest_apt_source() {
    install_requirements_if_not
    echo "Find fastest APT mirror ..."
    find_fastest_apt_source > /etc/apt/sources.list
    apt-get -qq update
}

function choose_fastest_pip_source() {
    install_requirements_if_not
    echo "Find fastest PIP mirror ..."
    mkdir -p "$HOME"/.pip
    find_fastest_pip_mirror > "$HOME"/.pip/pip.conf
}

function choose_fastest_alpine_source() {
    default="http://dl-cdn.alpinelinux.org"
    read -r -d '\t' urls <<EOM
$default
http://mirrors.tuna.tsinghua.edu.cn
\t
EOM

    best=$(find_fastest_url "$urls")
    if [[ "$best" != "$default" ]]; then
        sed -i "s=http://dl-cdn.alpinelinux.org=$best=g" /etc/apk/repositories
    fi
}

