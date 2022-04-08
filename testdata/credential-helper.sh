#!/bin/bash

echo $GITHUB_TOKEN
timeout 1s cat - > /tmp/werft-debug.json