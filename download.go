package main

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"github.com/vbauerster/mpb/v7"
	"github.com/vbauerster/mpb/v7/decor"
)

func startCliDownload(url string, contentLength int64, outputFile *os.File) {
	var waitGroup sync.WaitGroup
	var mutex sync.Mutex
	var workerId int
	var offset int64
	progressContainer := mpb.New()
	for offset = 0; offset <= contentLength; offset += cliChunkSize {
		for activeWorkers >= int(cliWorkers) {
			time.Sleep(200 * time.Millisecond)
		}
		label := fmt.Sprintf("Worker %v/%v", workerId+1, int64(contentLength/cliChunkSize)+1)
		progressBar := progressContainer.New(
			100,
			mpb.BarStyle().Padding(" "),
			mpb.PrependDecorators(
				decor.Name(label, decor.WC{W: len(label), C: decor.DidentRight}),
			),
			mpb.AppendDecorators(decor.Percentage(decor.WC{W: 6, C: decor.DidentRight})),
		)
		go cliDownloadChunk(url, workerId, outputFile, offset, progressBar, &waitGroup, &mutex)
		waitGroup.Add(1)
		workerId++
		mutex.Lock()
		activeWorkers++
		mutex.Unlock()
	}
	waitGroup.Wait()
	progressContainer.Wait()
	downloading = false
	activeWorkers = 0
	fmt.Println("Your file has been successfully downloaded!")
}

func startDownload(url string, contentLength int64, outputFile *os.File) {
	if !downloading {
		enableDownloads()
		return
	}

	var waitGroup sync.WaitGroup
	var mutex sync.Mutex
	var workerId int
	var offset int64
	go refreshContainers()
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
		label := fmt.Sprintf("Worker %v/%v", workerId+1, int64(contentLength/chunkSize)+1)
		progressBar := widget.NewProgressBar()
		progressBarContainer := &ChunkContainer{
			label,
			progressBar,
			fyne.NewContainerWithLayout(layout.NewFormLayout(), widget.NewLabel(label), progressBar),
		}
		threadContainer.Add(progressBarContainer.container)
		go downloadChunk(url, workerId, outputFile, offset, progressBarContainer, &waitGroup, &mutex)
		waitGroup.Add(1)
		workerId++
		mutex.Lock()
		activeWorkers++
		mutex.Unlock()
	}
	waitGroup.Wait()
	if downloading {
		dialog.ShowInformation("Download Complete", "Your file has been successfully downloaded!", mainWindow)
	}
}

func cliDownloadChunk(url string, workerId int, outputFile *os.File, offset int64, progressBar *mpb.Bar, waitGroup *sync.WaitGroup, mutex *sync.Mutex) {
	success := false

	for !success {
		if !downloading {
			mutex.Lock()
			activeWorkers--
			mutex.Unlock()
			waitGroup.Done()
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
		cliChunkWriter := &CliChunkWriter{outputFile, int64(offset), int64(offset), progressBar}
		_, err = io.Copy(cliChunkWriter, response.Body)
		if err != nil {
			continue
		}
		percentage := float64(cliChunkWriter.offset-offset) / float64(cliChunkSize) * 100
		if int64(percentage) != 100 {
			progressBar.SetCurrent(100)
		}
		success = true
	}
	mutex.Lock()
	activeWorkers--
	mutex.Unlock()
	waitGroup.Done()
}

func downloadChunk(url string, workerId int, outputFile *os.File, offset int64, progressBarContainer *ChunkContainer, waitGroup *sync.WaitGroup, mutex *sync.Mutex) {
	success := false

	for !success {
		if !downloading {
			mutex.Lock()
			activeWorkers--
			mutex.Unlock()
			waitGroup.Done()
			return
		}

		request, err := http.NewRequest("GET", url, nil)
		if err != nil {
			if downloading {
				dialog.ShowInformation("Error", wrapText(err.Error()), mainWindow)
			}
			enableDownloads()
			waitGroup.Done()
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
				mutex.Lock()
				activeWorkers--
				mutex.Unlock()
				waitGroup.Done()
				return
			}
			dialog.ShowInformation("Error (retrying)", fmt.Sprintf("Worker %v has ran into an error:\n%v", workerId, wrapText(err.Error())), mainWindow)
			continue
		}
		success = true
	}

	mutex.Lock()
	activeWorkers--
	threadContainer.Remove(progressBarContainer.container)
	mutex.Unlock()
	waitGroup.Done()
}
