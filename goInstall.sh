
#!/bin/bash

goBinFolder='/usr/local/go'

installDir=$(mktemp)
goPackage="$(curl -s https://go.dev/dl/ | awk -F[\>\<] '/linux-arm64/ && !/beta/ {print $5;exit}')"
wget "https://go.dev/dl/${goPackage}" -P "${installDir}"

# remove previous installation
if [ -d "$goBinFolder" ]; then
  sudo rm -rf "$goBinFolder"
fi

if [ ! -d "/usr/local" ]; then
    sudo mkdir -p /usr/local
fi

sudo tar -C /usr/local -xzf "${installDir}/${goPackage}"

rm -fr "${installDir}"

# Update ~/.profile file
# PATH=$PATH:$goBinFolder/bin
# GOPATH=$HOME/golang

go version
