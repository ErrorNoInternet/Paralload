package main

import (
	"errors"
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
)

var (
	version         string = "1.0.3"
	application     fyne.App
	mainWindow      fyne.Window
	optionWindow    fyne.Window
	downloadButton  *widget.Button
	threadContainer *fyne.Container
	usedContainers  []*fyne.Container
	activeWorkers   int
	downloading     bool

	workers   int    = 32
	chunkSize int64  = 512000
	timeout   int    = 10
	userAgent string = "go-http-client/paralload"
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
			float64(chunkWriter.offset-chunkWriter.originalOffset) / float64(chunkWriter.originalOffset+chunkSize-chunkWriter.originalOffset),
		)
		return count, err
	} else {
		return 0, errors.New("cancelled")
	}
}

func main() {
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

func cleanContainers() {
	for downloading || activeWorkers >= 1 {
		for _, container := range usedContainers {
			threadContainer.Remove(container)
		}
		threadContainer.Refresh()
		time.Sleep(50 * time.Millisecond)
	}
}

func enableDownloads() {
	downloadButton.SetText("Download")
	downloadButton.Enable()
	downloading = false
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
			dialog.ShowInformation("Unsupported", "This server does not provide Content-Length", mainWindow)
		}
		enableDownloads()
		return
	}
	contentLength, err := strconv.ParseInt(response.Header.Get("Content-Length"), 10, 64)
	if err != nil {
		if downloading {
			dialog.ShowInformation("Unsupported", "This server does not provide a valid Content-Length", mainWindow)
		}
		enableDownloads()
		return
	}

	startDownload(url, path, contentLength, outputFile)
	if downloading {
		enableDownloads()
	}
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
		if err != nil {
			dialog.ShowInformation("Workers", fmt.Sprintf("\"%v\" is an invalid number!", workersEntry.Text), optionWindow)
			return
		}
		chunkSizeCount, err := strconv.ParseInt(chunkSizeEntry.Text, 10, 64)
		if err != nil {
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
