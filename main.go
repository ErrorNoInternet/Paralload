package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/vbauerster/mpb/v7"
)

var (
	version         string = "1.1.1"
	application     fyne.App
	mainWindow      fyne.Window
	optionWindow    fyne.Window
	downloadButton  *widget.Button
	threadContainer *fyne.Container
	activeWorkers   int
	downloading     bool

	workers                                     int = 32
	cliWorkers                                  int
	chunkSize                                   int64 = 1048576
	cliChunkSize                                int64
	timeout                                     int = 10
	cliTimeout                                  int
	userAgent                                   string = "go-http-client/paralload"
	cliDownloadURL, cliUserAgent, cliOutputFile string
)

type ChunkContainer struct {
	label       string
	progressBar *widget.ProgressBar
	container   *fyne.Container
}

type ChunkWriter struct {
	io.WriterAt
	originalOffset       int64
	offset               int64
	progressBarContainer *ChunkContainer
}

func (chunkWriter *ChunkWriter) Write(bytes []byte) (int, error) {
	if downloading {
		count, err := chunkWriter.WriteAt(bytes, chunkWriter.offset)
		chunkWriter.offset += int64(count)
		chunkWriter.progressBarContainer.progressBar.SetValue(
			float64(chunkWriter.offset-chunkWriter.originalOffset) / float64(chunkSize),
		)
		return count, err
	} else {
		return 0, errors.New("cancelled")
	}
}

type CliChunkWriter struct {
	io.WriterAt
	originalOffset int64
	offset         int64
	progressBar    *mpb.Bar
}

func (cliChunkWriter *CliChunkWriter) Write(bytes []byte) (int, error) {
	if downloading {
		count, err := cliChunkWriter.WriteAt(bytes, cliChunkWriter.offset)
		cliChunkWriter.offset += int64(count)
		cliChunkWriter.progressBar.SetCurrent(int64(
			float64(cliChunkWriter.offset-cliChunkWriter.originalOffset) / float64(cliChunkSize) * 100,
		))
		return count, err
	} else {
		return 0, errors.New("cancelled")
	}
}

func main() {
	flag.StringVar(&cliDownloadURL, "url", "", "The URL of the file you want to download")
	flag.StringVar(&cliUserAgent, "userAgent", userAgent, "The user agent to use when making requests")
	flag.StringVar(&cliOutputFile, "output", "", "The file that should store the downloaded data")
	flag.IntVar(&cliWorkers, "workers", workers, "The amount of workers to use when downloading")
	flag.Int64Var(&cliChunkSize, "chunkSize", int64(chunkSize), "The amount of workers to use when downloading")
	flag.IntVar(&cliTimeout, "timeout", timeout, "The amount of seconds to wait before timing out")
	displayVersion := flag.Bool("version", false, "Display the current version of Paralload")
	flag.Parse()
	if *displayVersion {
		fmt.Printf("Paralload %v\n", version)
		return
	}
	if cliDownloadURL != "" {
		if cliOutputFile == "" {
			fmt.Println("Please provide an output file!")
			return
		}
		if cliWorkers < 1 {
			fmt.Printf("\"%v\" is an invalid number!\n", cliWorkers)
			return
		}
		if cliChunkSize < 1 {
			fmt.Printf("\"%v\" is an invalid number!\n", cliChunkSize)
			return
		}
		fmt.Printf("Workers: %v, Chunk Size: %v bytes, Timeout: %vs. Starting download...\n", cliWorkers, cliChunkSize, cliTimeout)
		result := startCliDownloadManager(cliDownloadURL, cliOutputFile, cliWorkers, cliChunkSize, cliTimeout, cliUserAgent)
		if result != 0 {
			return
		}
	} else {
		application = app.New()
		mainWindow = application.NewWindow("Paralload " + version)
		mainWindow.SetIcon(resourceIconPng)

		urlLabel := widget.NewLabel("Download URL")
		urlEntry := widget.NewEntry()
		urlContainer := fyne.NewContainerWithLayout(layout.NewFormLayout(), urlLabel, urlEntry)
		pathLabel := widget.NewLabel("Output File")
		pathEntry := widget.NewEntry()
		pathBrowseButton := widget.NewButtonWithIcon("", theme.FileIcon(), func() {
			dialog.ShowFileSave(func(uri fyne.URIWriteCloser, err error) {
				if err != nil {
					dialog.ShowInformation("Error", wrapText(err.Error()), mainWindow)
					return
				}
				if uri != nil {
					pathEntry.SetText(uri.URI().Path())
				}
			}, mainWindow)
		})
		pathOptionsContainer := fyne.NewContainerWithLayout(layout.NewFormLayout(), pathLabel, pathEntry)
		pathContainer := fyne.NewContainerWithLayout(layout.NewBorderLayout(nil, nil, nil, pathBrowseButton), pathOptionsContainer, pathBrowseButton)
		advancedOptionsButton := widget.NewButtonWithIcon("Advanced Options", theme.SettingsIcon(), showAdvancedOptions)
		downloadButton = widget.NewButtonWithIcon("Download", theme.DownloadIcon(), func() {
			go startDownloadManager(urlEntry, pathEntry)
		})
		optionContainer := fyne.NewContainerWithLayout(layout.NewVBoxLayout(), urlContainer, pathContainer, advancedOptionsButton, downloadButton)
		threadContainer = fyne.NewContainerWithLayout(
			layout.NewVBoxLayout(),
			layout.NewSpacer(),
			fyne.NewContainerWithLayout(layout.NewCenterLayout(), widget.NewLabel("There are no active workers...")),
			layout.NewSpacer(),
		)

		mainWindow.Resize(fyne.Size{Width: 600, Height: 500})
		mainWindow.SetContent(
			fyne.NewContainerWithLayout(
				layout.NewBorderLayout(optionContainer, nil, nil, nil),
				optionContainer,
				container.NewVScroll(threadContainer),
			),
		)
		mainWindow.ShowAndRun()
	}
}

func refreshContainers() {
	for downloading || activeWorkers > 0 {
		threadContainer.Refresh()
		time.Sleep(100 * time.Millisecond)
	}
}

func enableDownloads() {
	downloadButton.SetText("Download")
	downloadButton.Enable()
	downloading = false
	activeWorkers = 0
	threadContainer.RemoveAll()
	threadContainer.Add(layout.NewSpacer())
	threadContainer.Add(fyne.NewContainerWithLayout(layout.NewCenterLayout(), widget.NewLabel("There are no active workers...")))
	threadContainer.Add(layout.NewSpacer())
}

func wrapText(text string) string {
	output := ""
	counter := 0
	for _, letter := range text {
		output += string(letter)
		counter++
		if counter > 40 {
			counter = 0
			output += "\n"
		}
	}
	return output
}

func startCliDownloadManager(url string, path string, workers int, chunkSize int64, timeout int, userAgent string) int {
	fmt.Println("Sending HEAD request to " + url + "...")
	request, err := http.NewRequest("HEAD", url, nil)
	if err != nil {
		fmt.Println("Error: " + err.Error())
		return 1
	}
	outputFile, err := os.Create(path)
	if err != nil {
		fmt.Println("The output file could not be created: " + err.Error())
		return 1
	}
	request.Header.Set("User-Agent", userAgent)
	client := &http.Client{Timeout: time.Duration(timeout) * time.Second}
	response, err := client.Do(request)
	if err != nil {
		fmt.Println("Error: " + err.Error())
		return 1
	}
	if response.Header.Get("Accept-Ranges") != "bytes" {
		fmt.Println("Error: This server does not support HTTP byte ranges")
		return 1
	}
	if response.Header.Get("Content-Length") == "" {
		fmt.Println("Error: This server does not provide the Content-Length header")
		return 1
	}
	contentLength, err := strconv.ParseInt(response.Header.Get("Content-Length"), 10, 64)
	if err != nil {
		fmt.Println("Error: This server does not provide a valid Content-Length header")
		return 1
	}

	downloading = true
	startCliDownload(url, contentLength, outputFile)
	return 0
}

func startDownloadManager(urlEntry *widget.Entry, pathEntry *widget.Entry) {
	if downloading {
		downloading = false
		downloadButton.SetText("Cancelling...")
		downloadButton.Disable()
		return
	}

	url := strings.TrimSpace(urlEntry.Text)
	if url == "" {
		dialog.ShowInformation("No URL", "Please specify a download URL", mainWindow)
		return
	}
	path := strings.TrimSpace(pathEntry.Text)
	if path == "" {
		dialog.ShowInformation("No Path", "Please specify a file path", mainWindow)
		return
	}
	outputFile, err := os.Create(path)
	if err != nil {
		dialog.ShowInformation("Error", "The output file could not be created:\n"+wrapText(err.Error()), mainWindow)
		return
	}
	downloadButton.SetText("Cancel Download")
	downloading = true
	threadContainer.RemoveAll()

	request, err := http.NewRequest("HEAD", url, nil)
	if err != nil {
		if downloading {
			dialog.ShowInformation("Error", wrapText(err.Error()), mainWindow)
		}
		enableDownloads()
		return
	}
	request.Header.Set("User-Agent", userAgent)
	client := &http.Client{Timeout: time.Duration(timeout) * time.Second}
	response, err := client.Do(request)
	if err != nil {
		if downloading {
			dialog.ShowInformation("Error", wrapText(err.Error()), mainWindow)
		}
		enableDownloads()
		return
	}
	if response.Header.Get("Accept-Ranges") != "bytes" {
		if downloading {
			dialog.ShowInformation("Unsupported", "This server does not support HTTP byte ranges", mainWindow)
		}
		enableDownloads()
		return
	}
	if response.Header.Get("Content-Length") == "" {
		if downloading {
			dialog.ShowInformation("Unsupported", "This server does not provide the Content-Length header", mainWindow)
		}
		enableDownloads()
		return
	}
	contentLength, err := strconv.ParseInt(response.Header.Get("Content-Length"), 10, 64)
	if err != nil {
		if downloading {
			dialog.ShowInformation("Unsupported", "This server does not provide a valid Content-Length header", mainWindow)
		}
		enableDownloads()
		return
	}

	startDownload(url, contentLength, outputFile)
	enableDownloads()
}

func showAdvancedOptions() {
	if optionWindow != nil {
		optionWindow.Close()
		optionWindow = nil
		return
	}

	optionWindow = application.NewWindow("Advanced Options")
	optionWindow.SetIcon(resourceIconPng)
	optionWindow.SetOnClosed(func() {
		optionWindow = nil
	})

	workersLabel := widget.NewLabel("Workers")
	workersEntry := widget.NewEntry()
	workersEntry.SetText(strconv.Itoa(workers))
	workersContainer := fyne.NewContainerWithLayout(layout.NewFormLayout(), workersLabel, workersEntry)
	chunkSizeLabel := widget.NewLabel("Chunk Size")
	chunkSizeEntry := widget.NewEntry()
	chunkSizeEntry.SetText(strconv.FormatInt(chunkSize, 10))
	chunkSizeContainer := fyne.NewContainerWithLayout(layout.NewFormLayout(), chunkSizeLabel, chunkSizeEntry)
	timeoutLabel := widget.NewLabel("Timeout")
	timeoutEntry := widget.NewEntry()
	timeoutEntry.SetText(strconv.Itoa(timeout))
	timeoutContainer := fyne.NewContainerWithLayout(layout.NewFormLayout(), timeoutLabel, timeoutEntry)
	userAgentLabel := widget.NewLabel("User Agent")
	userAgentEntry := widget.NewEntry()
	userAgentEntry.SetText(userAgent)
	userAgentContainer := fyne.NewContainerWithLayout(layout.NewFormLayout(), userAgentLabel, userAgentEntry)

	saveButton := widget.NewButtonWithIcon("Save", theme.DocumentSaveIcon(), func() {
		if downloading {
			dialog.ShowInformation("Download In Progress", "There is an active download in the background!", optionWindow)
			return
		}

		workersCount, err := strconv.Atoi(workersEntry.Text)
		if err != nil || workersCount < 1 {
			dialog.ShowInformation("Workers", fmt.Sprintf("\"%v\" is an invalid number!", workersEntry.Text), optionWindow)
			return
		}
		chunkSizeCount, err := strconv.ParseInt(chunkSizeEntry.Text, 10, 64)
		if err != nil || chunkSizeCount < 1 {
			dialog.ShowInformation("Chunk Size", fmt.Sprintf("\"%v\" is an invalid number!", chunkSizeEntry.Text), optionWindow)
			return
		}
		timeoutTime, err := strconv.Atoi(timeoutEntry.Text)
		if err != nil {
			dialog.ShowInformation("Timeout", fmt.Sprintf("\"%v\" is an invalid number!", timeoutEntry.Text), optionWindow)
			return
		}
		workers = workersCount
		chunkSize = chunkSizeCount
		timeout = timeoutTime
		userAgent = userAgentEntry.Text
		optionWindow.Close()
		optionWindow = nil
	})
	advancedOptionsContainer := fyne.NewContainerWithLayout(
		layout.NewVBoxLayout(),
		workersContainer,
		chunkSizeContainer,
		timeoutContainer,
		userAgentContainer,
		saveButton,
	)

	optionWindow.SetContent(advancedOptionsContainer)
	optionWindow.Resize(fyne.Size{Width: 500, Height: 0})
	optionWindow.SetFixedSize(true)
	optionWindow.Show()
}
