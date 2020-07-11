#!/bin/bash

GOOS=linux GOARCH=arm64 go build -o sensehat-promexp-linux-arm64 -ldflags '-w -s'
