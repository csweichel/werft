#!/bin/bash
# This script was adapted from https://github.com/inlets/inlets/blob/master/get.sh

export OWNER=32leaves
export REPO=werft
export SUCCESS_CMD="$REPO version"
export BINLOCATION="/usr/bin"

export platform="$(uname -s | tr '[:upper:]' '[:lower:]')"
export binary="werft-client-$platform-amd64"
latestRelease=$(curl --silent -s https://api.github.com/repos/$OWNER/$REPO/releases/latest \
    | grep browser_download_url \
    | grep $platform \
    | cut -d : -f 2,3 \
    | tr -d '"')

if [ ! $latestRelease ]; then
    echo "Failed while attempting to install $REPO. Please manually install:"
    echo ""
    echo "1. Open your web browser and go to https://github.com/$OWNER/$REPO/releases"
    echo "2. Download the latest release for your platform. Call it '$REPO'."
    echo "3. chmod +x ./$REPO"
    echo "4. mv ./$REPO $BINLOCATION"
    exit 1
fi

hasCli() {

    hasCurl=$(which curl)
    if [ "$?" = "1" ]; then
        echo "You need curl to use this script."
        exit 1
    fi
}

getPackage() {
    targetFile="/tmp/$REPO"

    if [ "$userid" != "0" ]; then
        targetFile="$(pwd)/$REPO"
    fi

    if [ -e $targetFile ]; then
        rm $targetFile
    fi

    echo "Downloading package $latestRelease as $targetFile"

    tmpdir=$(mktemp -d)
    (cd $tmpdir; curl -L $latestRelease | tar xz)
    if [ "$?" = "0" ]; then
        mv $tmpdir/$binary $targetFile
        chmod +x $targetFile
        echo "Download complete."

        if [ ! -w "$BINLOCATION" ]; then
            echo
            echo "============================================================"
            echo "  The script was run as a user who is unable to write"
            echo "  to $BINLOCATION. To complete the installation the"
            echo "  following commands may need to be run manually."
            echo "============================================================"
            echo
            echo "  sudo cp $REPO $BINLOCATION/$REPO"
            echo
        else
            echo
            echo "Running with sufficient permissions to attempt to move $REPO to $BINLOCATION"

            if [ ! -w "$BINLOCATION/$REPO" ] && [ -f "$BINLOCATION/$REPO" ]; then
                echo
                echo "================================================================"
                echo "  $BINLOCATION/$REPO already exists and is not writeable"
                echo "  by the current user.  Please adjust the binary ownership"
                echo "  or run sh/bash with sudo." 
                echo "================================================================"
                echo
                exit 1
            fi

            mv $targetFile $BINLOCATION/$REPO

            if [ "$?" = "0" ]; then
                echo "New version of $REPO installed to $BINLOCATION"
            fi

            if [ -e $targetFile ]; then
                rm $targetFile
            fi

           ${SUCCESS_CMD}
        fi
    fi
}

hasCli
getPackage
