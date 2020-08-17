#!/bin/bash

GOOS=linux GOARCH=arm go build -o boatpi-promexp-linux-arm -ldflags '-w -s' ./cmd/promexp
