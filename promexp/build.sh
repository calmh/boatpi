#!/bin/bash

GOOS=linux GOARCH=arm go build -o sensehat-promexp-linux-arm -ldflags '-w -s'
