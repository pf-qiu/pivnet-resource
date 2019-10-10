package release

import (
	"fmt"

	pivnet "github.com/pivotal-cf/go-pivnet/v2"
	"github.com/pivotal-cf/go-pivnet/v2/logger"
	"github.com/pivotal-cf/pivnet-resource/metadata"
)

type ReleaseFileGroupsAdder struct {
	logger      logger.Logger
	pivnet      releaseFileGroupsAdderClient
	metadata    metadata.Metadata
	productSlug string
}

func NewReleaseFileGroupsAdder(
	logger logger.Logger,
	pivnetClient releaseFileGroupsAdderClient,
	metadata metadata.Metadata,
	productSlug string,
) ReleaseFileGroupsAdder {
	return ReleaseFileGroupsAdder{
		logger:      logger,
		pivnet:      pivnetClient,
		metadata:    metadata,
		productSlug: productSlug,
	}
}

//go:generate counterfeiter --fake-name ReleaseFileGroupsAdderClient . releaseFileGroupsAdderClient
type releaseFileGroupsAdderClient interface {
	AddFileGroup(productSlug string, releaseID int, fileGroupID int) error
	CreateFileGroup(config pivnet.CreateFileGroupConfig) (pivnet.FileGroup, error)
	AddToFileGroup(productSlug string, fileGroupID int, productFileID int) error
	FileGroupsForRelease(productSlug string, releaseID int) ([]pivnet.FileGroup, error)
}

func (rf ReleaseFileGroupsAdder) AddReleaseFileGroups(release pivnet.Release) error {
	groups, err := rf.pivnet.FileGroupsForRelease(rf.productSlug, release.ID)
	if err != nil {
		return nil
	}

	groupMapping := make(map[string]int)
	for _, g := range groups {
		groupMapping[g.Name] = g.ID
	}

	for _, fileGroup := range rf.metadata.FileGroups {
		fileGroupID, ok := groupMapping[fileGroup.Name]
		if ok {
			rf.logger.Info(fmt.Sprintf(
				"Reuse file group with ID: %d",
				fileGroupID,
			))
		} else {
			rf.logger.Info(fmt.Sprintf(
				"Creating file group with name: %s",
				fileGroup.Name,
			))
			g, err := rf.pivnet.CreateFileGroup(pivnet.CreateFileGroupConfig{
				ProductSlug: rf.productSlug,
				Name:        fileGroup.Name,
			})

			if err != nil {
				return err
			}

			fileGroupID = g.ID

			rf.logger.Info(fmt.Sprintf(
				"Adding file group with ID: %d",
				fileGroupID,
			))

			err = rf.pivnet.AddFileGroup(rf.productSlug, release.ID, fileGroupID)
			if err != nil {
				return err
			}
		}

		for _, pf := range fileGroup.ProductFiles {
			rf.logger.Info(fmt.Sprintf(
				"Adding product file %d to file group with ID: %d",
				pf.ID,
				fileGroupID,
			))

			err := rf.pivnet.AddToFileGroup(rf.productSlug, fileGroupID, pf.ID)

			if err != nil {
				return err
			}
		}
	}

	return nil
}
