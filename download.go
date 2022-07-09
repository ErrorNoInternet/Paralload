package main

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"github.com/vbauerster/mpb/v7"
	"github.com/vbauerster/mpb/v7/decor"
)

func startCliDownload(url string, contentLength int64, outputFile *os.File) {
	var workerId int
	var offset int64
	progressContainer := mpb.New()
	for offset = 0; offset <= contentLength; offset += cliChunkSize {
		for activeWorkers >= int(cliWorkers) {
			time.Sleep(200 * time.Millisecond)
		}
		label := fmt.Sprintf("Worker %v/%v", workerId, int64(contentLength/cliChunkSize))
		progressBar := progressContainer.AddBar(
			100,
			nil,
			mpb.BarFillerClearOnComplete(),
			mpb.PrependDecorators(
				decor.Name(label, decor.WC{W: len(label), C: decor.DidentRight}),
			),
		)
		go cliDownloadChunk(url, workerId, outputFile, offset, progressBar)
		activeWorkers++
		workerId++
	}
	for activeWorkers > 0 {
		time.Sleep(1 * time.Second)
	}
	downloading = false
	activeWorkers = 0
	fmt.Println("Your file has been successfully downloaded!")
}

func startDownload(url string, contentLength int64, outputFile *os.File) {
	if !downloading {
		enableDownloads()
		return
	}

	var workerId int
	var offset int64
	go cleanContainers()
	for offset = 0; offset <= contentLength; offset += chunkSize {
		if !downloading {
			for activeWorkers > 0 {
				time.Sleep(1 * time.Second)
			}
			enableDownloads()
			return
		}
		for activeWorkers >= workers {
			time.Sleep(200 * time.Millisecond)
		}
		label := fmt.Sprintf("Worker %v/%v", workerId, int64(contentLength/chunkSize))
		progressBar := widget.NewProgressBar()
		progressBarContainer := &ChunkContainer{
			label,
			progressBar,
			fyne.NewContainerWithLayout(layout.NewFormLayout(), widget.NewLabel(label), progressBar),
		}
		threadContainer.Add(progressBarContainer.container)
		go downloadChunk(url, workerId, outputFile, offset, progressBarContainer)
		activeWorkers++
		workerId++
	}
	for activeWorkers > 0 {
		time.Sleep(1 * time.Second)
	}
	if downloading {
		dialog.ShowInformation("Download Complete", "Your file has been successfully downloaded!", mainWindow)
	}
}

func cliDownloadChunk(url string, workerId int, outputFile *os.File, offset int64, progressBar *mpb.Bar) {
	success := false

	for !success {
		if !downloading {
			return
		}

		request, err := http.NewRequest("GET", url, nil)
		if err != nil {
			fmt.Println("Error: " + err.Error())
			downloading = false
		}
		request.Header.Set("User-Agent", cliUserAgent)
		request.Header.Set("Range", fmt.Sprintf("bytes=%v-%v", offset, offset+cliChunkSize-1))
		client := &http.Client{
			Transport: &http.Transport{
				Dial: (&net.Dialer{
					Timeout:   time.Duration(cliTimeout) * time.Second,
					KeepAlive: time.Duration(cliTimeout) * time.Second,
				}).Dial,
				TLSHandshakeTimeout:   time.Duration(cliTimeout) * time.Second,
				ResponseHeaderTimeout: time.Duration(cliTimeout) * time.Second,
				IdleConnTimeout:       time.Duration(cliTimeout) * time.Second,
			},
		}
		response, err := client.Do(request)
		if err != nil {
			continue
		}
		defer response.Body.Close()
		written, err = io.Copy(
			&CliChunkWriter{
				outputFile,
				int64(offset),
				int64(offset),
				progressBar,
			},
			response.Body,
		)
		if err != nil {
			continue
		}
		success = true
	}
	activeWorkers--
}

func downloadChunk(url string, workerId int, outputFile *os.File, offset int64, progressBarContainer *ChunkContainer) {
	success := false

	for !success {
		if !downloading {
			activeWorkers--
			return
		}

		request, err := http.NewRequest("GET", url, nil)
		if err != nil {
			if downloading {
				dialog.ShowInformation("Error", wrapText(err.Error()), mainWindow)
			}
			enableDownloads()
			return
		}
		request.Header.Set("User-Agent", userAgent)
		request.Header.Set("Range", fmt.Sprintf("bytes=%v-%v", offset, offset+chunkSize-1))
		client := &http.Client{
			Transport: &http.Transport{
				Dial: (&net.Dialer{
					Timeout:   time.Duration(timeout) * time.Second,
					KeepAlive: time.Duration(timeout) * time.Second,
				}).Dial,
				TLSHandshakeTimeout:   time.Duration(timeout) * time.Second,
				ResponseHeaderTimeout: time.Duration(timeout) * time.Second,
				IdleConnTimeout:       time.Duration(timeout) * time.Second,
			},
		}
		response, err := client.Do(request)
		if err != nil {
			dialog.ShowInformation("Error (retrying)", fmt.Sprintf("Worker %v has ran into an error:\n%v", workerId, wrapText(err.Error())), mainWindow)
			continue
		}
		defer response.Body.Close()
		_, err = io.Copy(
			&ChunkWriter{
				outputFile,
				int64(offset),
				int64(offset),
				progressBarContainer,
			},
			response.Body,
		)
		if err != nil {
			if err.Error() == "cancelled" {
				activeWorkers--
				return
			}
			dialog.ShowInformation("Error (retrying)", fmt.Sprintf("Worker %v has ran into an error:\n%v", workerId, wrapText(err.Error())), mainWindow)
			continue
		}
		success = true
	}

	activeWorkers--
	usedContainers = append(usedContainers, progressBarContainer.container)
}
