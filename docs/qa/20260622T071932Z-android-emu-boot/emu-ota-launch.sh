#!/bin/bash
export LD_LIBRARY_PATH=/home/milosvasic/.local/lib:$LD_LIBRARY_PATH
export ANDROID_SDK_ROOT=/home/milosvasic/Android/Sdk
export ANDROID_HOME=/home/milosvasic/Android/Sdk
export PATH=/home/milosvasic/Android/Sdk/emulator:/home/milosvasic/Android/Sdk/platform-tools:$PATH
cd /tmp
exec /home/milosvasic/Android/Sdk/emulator/emulator \
    -avd CZ_API36_Phone \
    -no-window -no-audio \
    -gpu swiftshader_indirect \
    -memory 3072 \
    -cores 2 \
    -port 5554 \
    -no-snapshot -no-cache -wipe-data \
    -verbose
