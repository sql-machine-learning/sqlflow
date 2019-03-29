#! /bin/bash
set -x
GRP_ID=$(id -g)
USER_ID=$(id -u)
if [[ $USER_ID == 0 ]]; then
    USER_ID=${SUDO_UID}
    GRP_ID=${SUDO_GID}
fi

if [[ $OSTYPE == "darwin"* ]]; then
    GRP_NAME=$(dscl . -search /Groups PrimaryGroupID ${GRP_ID} | head -1 | cut -f1)
    USER_NAME=$(dscl . -search /Users UniqueID ${USER_ID} | head -1 | cut -f1)
else
    GRP_NAME=$(getent group ${GRP_ID} | cut -d: -f1)
    USER_NAME=$(getent passwd ${USER_ID} | cut -d: -f1)
fi

HOME_DIR=$(eval echo ~${USER_NAME})

docker run -h localhost --rm -it --net=host \
    -v ${HOME_DIR}:/home/${USER_NAME}  \
    -e REAL_GID=${GRP_ID} \
    -e REAL_GRP=${GRP_NAME} \
    -e REAL_UID=${USER_ID} \
    -e REAL_USER=${USER_NAME} \
    go_client /bin/bash 
