package logic

import (
	"io/ioutil"
	"os"
	"regexp"

	"github.com/jasonlvhit/gocron"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"github.com/jclebreton/opensirene/conf"
	"github.com/jclebreton/opensirene/database"
	"github.com/jclebreton/opensirene/opendata/gouvfr/sirene"
)

type Crontab struct {
	PgxClient *database.PgxClient
	Config    conf.Crontab
}

// getDatabaseStatus lists the last successful updates in the database
// and returns the slice of filenames which were successfuly imported.
// Returns an empty slice otherwise.
func (ct *Crontab) getDatabaseStatus() ([]string, error) {
	rows, err := ct.PgxClient.Conn.Query("SELECT filename FROM history WHERE is_success=true")
	if err != nil {
		return nil, err
	}

	var result []string
	for rows.Next() {
		var filename string
		if err := rows.Scan(&filename); err != nil {
			return nil, err
		}
		result = append(result, filename)
	}

	return result, nil
}

// getFilesToImport is a function used to filter a slice of RemoteFile with a slice of
// string. If the file name is present in the RemoteFiles, they are evicted from
// the returned slice
func (ct *Crontab) getFilesToImport(localFiles []string, remoteFiles sirene.RemoteFiles) sirene.RemoteFiles {
	result := sirene.RemoteFiles{}
	for _, remoteFile := range remoteFiles {
		if !ct.isAlreadyImported(localFiles, remoteFile.FileName) {
			result = append(result, remoteFile)
		}
	}
	return result
}

func (ct *Crontab) isAlreadyImported(localFiles []string, remoteFile string) bool {
	for _, localFile := range localFiles {
		if remoteFile == localFile {
			return true
		}
	}
	return false
}

//// removeUselessFiles removes from disk old update files
func (ct *Crontab) removeUselessFiles(keepList []string) error {
	var files []os.FileInfo
	var result []string
	var err error
	var keepIt bool

	if files, err = ioutil.ReadDir(ct.Config.DownloadPath); err != nil {
		return errors.Wrap(err, "couldn't remove useless files")
	}

	for _, f := range files {
		for _, keepFile := range keepList {
			keepIt = true
			match, err := regexp.MatchString("/\\.(zip|csv)$/", f.Name())
			if f.Name() != keepFile && err == nil && match {
				keepIt = false
				break
			}
		}
		if !keepIt {
			f := ct.Config.DownloadPath + "/" + f.Name()
			result = append(result, f)
			if err = os.Remove(f); err != nil {
				return errors.Wrap(err, "couldn't remove useless files")
			}

		}
	}

	if len(result) > 0 {
		logrus.WithField("files", result).Info("Some useless files has been removed")
	}

	return nil
}

// Daily is the cron task that runs every few hours to get and apply the latest
// updates
func (ct *Crontab) startUpdate() {
	var err error
	var remoteFiles sirene.RemoteFiles
	var dbFiles []string

	// Retrieve the list of update files stored in database
	if dbFiles, err = ct.getDatabaseStatus(); err != nil {
		logrus.WithError(err).Error("Could not retrieve current database status")
		return
	}

	// Remove other ZIP and CSV files
	if err = ct.removeUselessFiles(dbFiles); err != nil {
		logrus.WithError(err).Error("Could not remove useless files")
		return

	}

	// Search updates
	if remoteFiles, err = sirene.GrabLatestFull(ct.Config.DownloadPath); err != nil {
		logrus.WithError(err).Error("Could not grab latest index from gov")
		return
	}

	// Files to grab
	toDownload := ct.getFilesToImport(dbFiles, remoteFiles)

	logrus.
		WithField("remoteFiles", remoteFiles).
		WithField("dbFiles", dbFiles).
		WithField("toDownload", toDownload).Info("Crontab status")

	// Start upgrade
	if err = ImportRemoteFiles(ct.PgxClient, toDownload, ct.Config.DownloadPath); err != nil {
		logrus.WithError(err).Error("Could not update database with latest files")
		return
	}
}

// StartCrontab run Daily function every few hours
func (ct *Crontab) Start() {
	gocron.Every(ct.Config.EveryXHours).Hours().Do(ct.startUpdate)
	gocron.RunAll() // Execute the task at startup
	_, t := gocron.NextRun()
	logrus.WithField("next", t).Info("Started cron background task")
	<-gocron.Start()
}
