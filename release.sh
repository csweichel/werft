#!/bin/bash

leeway build //:release -Dcommit=$(git rev-parse HEAD) -Ddate="$(date)" $*
