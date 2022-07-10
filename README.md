# Paralload
A download tool that uses multiple HTTP(S) connections and byte ranges
![Screenshot](https://raw.githubusercontent.com/ErrorNoInternet/Paralload/main/screenshots/0.png)

## Compiling
- Requirements
  - Go (1.18 recommended)
```
git clone https://github.com/ErrorNoInternet/Paralload
cd Paralload
go build
```

## Usage
Running the executable without any arguments (`./paralload`) will launch the GUI, but there is command-line support
```
# Show all arguments
./paralload -help

# Download a file with 16 workers and a timeout of 3 seconds
./paralload -url https://speedtest-ny.turnkeyinternet.net/100mb.bin -output 100mb.bin -workers 16 -timeout 3

# Download a file with 4 workers and a chunk size of 8 MB
./paralload -url https://speedtest-ny.turnkeyinternet.net/100mb.bin -output 100mb.bin -workers 4 -chunkSize 8192000

# Download a file with a custom user agent
./paralload -url https://speedtest-ny.turnkeyinternet.net/100mb.bin -output 100mb.bin -userAgent "hello world"
```

<sub>If you would like to modify or use this repository (including its code) in your own project, please be sure to credit!</sub>

