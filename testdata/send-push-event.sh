#!/bin/bash

curl -XPOST \
    -H"content-type: application/json" \
    -H"Expect: " \
    -H"User-Agent: GitHub-Hookshot/b6210f6" \
    -H"X-GitHub-Delivery: 5529067a-14f1-11ea-8f35-75cb7053287b" \
    -H "X-GitHub-Event: push" \
    -H "X-Hub-Signature: sha1=f6b0ccbd7dbe39d2a807668670e60bd07dbd6b6a" \
    -d @push-event-payload.json \
    http://localhost:8080/plugins/github-trigger