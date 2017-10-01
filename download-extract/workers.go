package download_extract

import (
	"os"

	"strings"

	log "github.com/sirupsen/logrus"
)

func DownloadAndExtract(zipFiles []ZipFile, nbWorkers int) ([]csvFile, error) {
	//Progress
	downloadProgressChan := make(chan map[string]float64)
	unzipProgressChan := make(chan map[string]float64)
	go progress(len(zipFiles), downloadProgressChan, unzipProgressChan)
	defer close(downloadProgressChan)
	defer close(unzipProgressChan)

	errorsChan := make(chan error)
	endErrorsChan := make(chan bool)
	defer close(errorsChan)
	defer close(endErrorsChan)
	go listenErrors(errorsChan, endErrorsChan)

	//Workers
	nbZipFiles := len(zipFiles)
	workerChan := make(chan ZipFile)
	resultChan := make(chan []csvFile, nbZipFiles)
	defer close(workerChan)
	defer close(resultChan)
	for id := 1; id <= nbWorkers; id++ {
		go startWorker(id, workerChan, resultChan, downloadProgressChan, unzipProgressChan, errorsChan)
	}

	//Send Zip files
	go func() {
		for _, zipFile := range zipFiles {
			workerChan <- zipFile
		}
	}()

	//Waiting CSV files
	var csvFiles []csvFile
	for i := 1; i <= nbZipFiles; i++ {
		files := <-resultChan
		for _, f := range files {
			csvFiles = append(csvFiles, f)
		}
	}

	endErrorsChan <- true

	return csvFiles, nil
}

func listenErrors(errorsChan chan error, end chan bool) {
	var errors []string
loop:
	for {
		select {
		case error := <-errorsChan:
			errors = append(errors, error.Error())
		case <-end:
			break loop
		default:
		}
	}

	log.WithFields(log.Fields{
		"Number": len(errors),
		"Errors": strings.Join(errors, ", "),
	}).Error("Errors during processing files")
}

func startWorker(id int, workerChan <-chan ZipFile, resultChan chan<- []csvFile, downloadProgressChan,
	unzipProgressChan chan map[string]float64, errorsChan chan error) {
	for zipFile := range workerChan {
		err := zipFile.download(downloadProgressChan, errorsChan)
		if err != nil {
			unzipProgressChan <- map[string]float64{zipFile.filename: 100}
			resultChan <- []csvFile{}
			return
		}

		zipFile.csvFiles, err = zipFile.unzip(unzipProgressChan)
		if err != nil {
			log.Fatal(err)
			return
		}

		err = os.Remove(zipFile.path)
		if err != nil {
			log.Fatal(err)
			return
		}

		resultChan <- zipFile.csvFiles
	}
}