package release

import (
	"fmt"
	"path/filepath"
	"time"

	pivnet "github.com/pivotal-cf/go-pivnet/v2"
	"github.com/pivotal-cf/go-pivnet/v2/logger"
	"github.com/pivotal-cf/pivnet-resource/metadata"
)

type ReleaseUploader struct {
	s3            s3Client
	pivnet        uploadClient
	logger        logger.Logger
	sha256Summer  sha256Summer
	md5Summer     md5Summer
	metadata      metadata.Metadata
	sourcesDir    string
	productSlug   string
	asyncTimeout  time.Duration
	pollFrequency time.Duration
	skipPolling   bool

	remoteProductFilesMapping map[string]pivnet.ProductFile
}

type ProductFileMetadata struct {
	description        string
	fileVersion        string
	docsURL            string
	systemRequirements []string
	platforms          []string
	includedFiles      []string
	uploadAs           string
	fileType           string
}

//go:generate counterfeiter --fake-name UploadClient . uploadClient
type uploadClient interface {
	FindProductForSlug(slug string) (pivnet.Product, error)
	CreateProductFile(pivnet.CreateProductFileConfig) (pivnet.ProductFile, error)
	AddProductFile(productSlug string, releaseID int, productFileID int) error
	ProductFiles(productSlug string) ([]pivnet.ProductFile, error)
	ProductFile(productSlug string, productFileID int) (pivnet.ProductFile, error)
	DeleteProductFile(productSlug string, releaseID int) (pivnet.ProductFile, error)
}

//go:generate counterfeiter --fake-name S3Client . s3Client
type s3Client interface {
	ComputeAWSObjectKey(string) (string, string, error)
	UploadFile(string) error
}

//go:generate counterfeiter --fake-name Sha256Summer . sha256Summer
type sha256Summer interface {
	SumFile(filepath string) (string, error)
}

//go:generate counterfeiter --fake-name Md5Summer . md5Summer
type md5Summer interface {
	SumFile(filepath string) (string, error)
}

func NewReleaseUploader(
	s3 s3Client,
	pivnet uploadClient,
	logger logger.Logger,
	sha256Summer sha256Summer,
	md5Summer md5Summer,
	metadata metadata.Metadata,
	sourcesDir,
	productSlug string,
	asyncTimeout time.Duration,
	pollFrequency time.Duration,
	skipPolling bool,
) ReleaseUploader {
	return ReleaseUploader{
		s3:            s3,
		pivnet:        pivnet,
		logger:        logger,
		sha256Summer:  sha256Summer,
		md5Summer:     md5Summer,
		metadata:      metadata,
		sourcesDir:    sourcesDir,
		productSlug:   productSlug,
		asyncTimeout:  asyncTimeout,
		pollFrequency: pollFrequency,
		skipPolling:   skipPolling,
	}
}

func (u ReleaseUploader) UploadSingleFile(release pivnet.Release, f *metadata.ProductFile, standalone bool) error {
	awsObjectKey, _, err := u.s3.ComputeAWSObjectKey(f.File)
	if err != nil {
		return err
	}

	productFile, ok := u.remoteProductFilesMapping[awsObjectKey]

	if ok {
		matched, err := u.hasSameFileContent(f.File, productFile)
		if err != nil {
			return err
		}

		if !matched {
			return fmt.Errorf("File conflict: the file '%s' could not be uploaded and associated to this release."+
				"  A different file with the same name already exists on S3.  Please recreate the release using a different"+
				" filename for this file or upload the file to this release manually", f.File)
		} else {
			f.AlreadyExist = true
			if standalone {
				u.logger.Info(fmt.Sprintf("An identical file was found on S3, skipping file upload. The existing file %s "+
					"will be associated to this release.", awsObjectKey))
				u.pivnet.AddProductFile(u.productSlug, release.ID, productFile.ID)
			} else {
				u.logger.Info(fmt.Sprintf("An identical file was found on S3, skipping file upload. The existing file %s "+
					"belongs to a file group, no further actions.", awsObjectKey))
			}
		}
	} else {
		u.logger.Info(fmt.Sprintf(
			"Creating product file with remote name: '%s'",
			f.UploadAs,
		))

		err := u.s3.UploadFile(f.File)
		if err != nil {
			return err
		}

		fd := ProductFileMetadata{
			uploadAs:           f.UploadAs,
			description:        f.Description,
			fileVersion:        f.FileVersion,
			docsURL:            f.DocsURL,
			systemRequirements: f.SystemRequirements,
			platforms:          f.Platforms,
			includedFiles:      f.IncludedFiles,
			fileType:           f.FileType,
		}
		productFileConfig, err := u.getProductFileConfig(f.File, awsObjectKey, fd, release)
		if err != nil {
			return err
		}

		productFile, err = u.pivnet.CreateProductFile(productFileConfig)
		if err != nil {
			return err
		}

		if standalone {
			u.logger.Info(fmt.Sprintf(
				"Adding product file: '%s' with ID: %d",
				f.UploadAs,
				f.ID,
			))

			err = u.pivnet.AddProductFile(u.productSlug, release.ID, productFile.ID)
			if err != nil {
				return err
			}
		}
	}

	f.ID = productFile.ID
	u.logger.Info(fmt.Sprintf("FileID: %v", f.ID))

	err = u.pollForProductFile(productFile)
	if err != nil {
		return fmt.Errorf("error while polling: %s", err)
	}

	return nil
}

func (u ReleaseUploader) Upload(release pivnet.Release) (err error) {
	remoteProductFiles, err := u.pivnet.ProductFiles(u.productSlug)
	if err != nil {
		return err
	}
	u.remoteProductFilesMapping = make(map[string]pivnet.ProductFile)
	for _, pf := range remoteProductFiles {
		u.remoteProductFilesMapping[pf.AWSObjectKey] = pf
	}

	for i := range u.metadata.ProductFiles {
		err = u.UploadSingleFile(release, &u.metadata.ProductFiles[i], true)
		if err != nil {
			return err
		}
	}

	for i := range u.metadata.FileGroups {
		fg := u.metadata.FileGroups[i]
		for j := range fg.ProductFiles {
			err = u.UploadSingleFile(release, &fg.ProductFiles[j], false)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (u ReleaseUploader) pollForProductFile(productFile pivnet.ProductFile) error {
	if u.skipPolling {
		u.logger.Info(fmt.Sprintf(
			"Skipping polling for product file: '%s'",
			productFile.Name,
		))
		return nil
	}

	u.logger.Info(fmt.Sprintf(
		"Polling product file: '%s' for async transfer - will wait up to %v",
		productFile.Name,
		u.asyncTimeout,
	))

	timeoutTimer := time.NewTimer(u.asyncTimeout)
	pollTicker := time.NewTicker(u.pollFrequency)

	for {
		select {
		case <-timeoutTimer.C:
			return fmt.Errorf("timed out")
		case <-pollTicker.C:
			pf, err := u.pivnet.ProductFile(u.productSlug, productFile.ID)
			if err != nil {
				return err
			}

			if pf.FileTransferStatus != "in_progress" {
				u.logger.Info(fmt.Sprintf(
					"Product file: '%s' async transfer complete",
					productFile.Name,
				))

				timeoutTimer.Stop()
				pollTicker.Stop()

				if pf.FileTransferStatus != "complete" {
					return fmt.Errorf("file_transfer_status: %s", pf.FileTransferStatus)
				} else {
					return nil
				}
			}

			u.logger.Info(fmt.Sprintf(
				"Product file: '%s' async transfer incomplete",
				productFile.Name,
			))
		}
	}
}

func (u ReleaseUploader) hasSameFileContent(fileName string, productFile pivnet.ProductFile) (bool, error) {
	fileContentsSHA256, _, err := u.calculateHashes(fileName)
	if err != nil {
		return false, err
	}

	if productFile.SHA256 == fileContentsSHA256 {
		u.logger.Debug(fmt.Sprintf(
			"Found an existing product file (AWSObjectKey: '%s') that exactly matches the upload file. Skipping deletion and creation",
			productFile.AWSObjectKey,
		))
		return true, nil
	}
	return false, nil
}

func (u ReleaseUploader) getProductFileConfig(exactGlob string, awsObjectKey string, fileData ProductFileMetadata, release pivnet.Release) (pivnet.CreateProductFileConfig, error) {
	fileContentsSHA256, fileContentsMD5, err := u.calculateHashes(exactGlob)
	if err != nil {
		return pivnet.CreateProductFileConfig{}, err
	}

	fileVersion := release.Version
	if fileData.fileVersion != "" {
		fileVersion = fileData.fileVersion
	}
	productFileConfig := pivnet.CreateProductFileConfig{
		ProductSlug:        u.productSlug,
		Name:               fileData.uploadAs,
		AWSObjectKey:       awsObjectKey,
		FileVersion:        fileVersion,
		SHA256:             fileContentsSHA256,
		MD5:                fileContentsMD5,
		Description:        fileData.description,
		FileType:           fileData.fileType,
		DocsURL:            fileData.docsURL,
		SystemRequirements: fileData.systemRequirements,
		Platforms:          fileData.platforms,
		IncludedFiles:      fileData.includedFiles,
	}
	return productFileConfig, err
}

func (u ReleaseUploader) getFileData(exactGlob string) ProductFileMetadata {
	var fileData ProductFileMetadata

	fileData.uploadAs = filepath.Base(exactGlob)
	fileData.fileType = "Software"

	for _, f := range u.metadata.ProductFiles {
		if f.File == exactGlob {
			u.logger.Info(fmt.Sprintf(
				"exact glob '%s' matches metadata file: '%s'",
				exactGlob,
				f.File,
			))

			if f.UploadAs != "" {
				u.logger.Info(fmt.Sprintf(
					"uploading '%s' to remote filename: '%s' instead",
					exactGlob,
					f.UploadAs,
				))
				fileData.uploadAs = f.UploadAs
			}

			fileData.description = f.Description

			if f.FileType != "" {
				fileData.fileType = f.FileType
			}

			if f.FileVersion != "" {
				fileData.fileVersion = f.FileVersion
			}

			if f.DocsURL != "" {
				fileData.docsURL = f.DocsURL
			}

			if len(f.SystemRequirements) > 0 {
				fileData.systemRequirements = f.SystemRequirements
			}

			if len(f.Platforms) > 0 {
				fileData.platforms = f.Platforms
			}

			if len(f.IncludedFiles) > 0 {
				fileData.includedFiles = f.IncludedFiles
			}
		} else {
			u.logger.Info(fmt.Sprintf(
				"exact glob '%s' does not match metadata file: '%s'",
				exactGlob,
				f.File,
			))
		}
	}
	return fileData
}

func (u ReleaseUploader) calculateHashes(fileName string) (string, string, error) {
	fullFilepath := filepath.Join(u.sourcesDir, fileName)
	fileContentsSHA256, err := u.sha256Summer.SumFile(fullFilepath)
	if err != nil {
		return "", "", err
	}

	fileContentsMD5, err := u.md5Summer.SumFile(fullFilepath)
	if err != nil {
		return "", "", err
	}
	return fileContentsSHA256, fileContentsMD5, nil
}
