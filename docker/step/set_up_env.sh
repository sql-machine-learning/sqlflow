#!/bin/bash

. /find_fastest_resources.sh

echo "Find fastest apt-get mirror ..."
$(find_fastest_apt_source > /etc/apt/sources.list)

apt-get -qq update

echo "Setup pip mirro ..."
mkdir -p ~/.pip
$(find_fastest_pip_mirror > ~/.pip/pip.conf)

