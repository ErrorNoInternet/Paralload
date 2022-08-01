#!/bin/sh

echo "Making output directory..."
mkdir bin

echo "Compiling Linux 64-bit..."
CC="gcc" CXX="g++" GOOS=linux GOARCH=amd64 CGO_ENABLED=1 go build -ldflags "-s -w" -o bin/paralload_linux-amd64 -v

echo "Compiling Linux 32-bit..."
CC="gcc -m32" CXX="g++ -m32" GOOS=linux GOARCH=386 CGO_ENABLED=1 go build -ldflags "-s -w" -o bin/paralload_linux-386 -v

echo "Compiling Windows 64-bit..."
CC="zig cc -target x86_64-windows-gnu" CXX="zig c++ -target x86_64-windows-gnu" GOOS=windows GOARCH=amd64 CGO_ENABLED=1 go build -ldflags "-s -w" -o bin/paralload_windows-amd64.exe -v

echo "Compiling Windows 32-bit..."
CC="zig cc -target i386-windows-gnu" CXX="zig c++ -target i386-windows-gnu" GOOS=windows GOARCH=386 CGO_ENABLED=1 go build -ldflags "-s -w" -o bin/paralload_windows-386.exe -v

