#!/bin/sh

echo "Making output directory..."
mkdir bin

echo "Compiling Linux 64-bit..."
go build -ldflags "-s -w" -o bin/paralload_linux-amd64 -v

echo "Compiling Windows 64-bit..."
CC="zig cc -target x86_64-windows-gnu" CXX="zig c++ -target x86_64-windows-gnu" GOOS=windows GOARCH=amd64 CGO_ENABLED=1 go build -ldflags "-s -w" -o bin/paralload_windows-amd64.exe -v
