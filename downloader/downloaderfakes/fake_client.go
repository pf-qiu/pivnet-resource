// This file was generated by counterfeiter
package downloaderfakes

import (
	"github.com/pivotal-cf/go-pivnet/v2/download"
	"io"
	"sync"
)

type FakeClient struct {
	DownloadProductFileStub        func(writer *download.FileInfo, productSlug string, releaseID int, productFileID int, progressWriter io.Writer) error
	downloadProductFileMutex       sync.RWMutex
	downloadProductFileArgsForCall []struct {
		writer         *download.FileInfo
		productSlug    string
		releaseID      int
		productFileID  int
		progressWriter io.Writer
	}
	downloadProductFileReturns struct {
		result1 error
	}
	invocations      map[string][][]interface{}
	invocationsMutex sync.RWMutex
}

func (fake *FakeClient) DownloadProductFile(writer *download.FileInfo, productSlug string, releaseID int, productFileID int, progressWriter io.Writer) error {
	fake.downloadProductFileMutex.Lock()
	fake.downloadProductFileArgsForCall = append(fake.downloadProductFileArgsForCall, struct {
		writer         *download.FileInfo
		productSlug    string
		releaseID      int
		productFileID  int
		progressWriter io.Writer
	}{writer, productSlug, releaseID, productFileID, progressWriter})
	fake.recordInvocation("DownloadProductFile", []interface{}{writer, productSlug, releaseID, productFileID, progressWriter})
	fake.downloadProductFileMutex.Unlock()
	if fake.DownloadProductFileStub != nil {
		return fake.DownloadProductFileStub(writer, productSlug, releaseID, productFileID, progressWriter)
	} else {
		return fake.downloadProductFileReturns.result1
	}
}

func (fake *FakeClient) DownloadProductFileCallCount() int {
	fake.downloadProductFileMutex.RLock()
	defer fake.downloadProductFileMutex.RUnlock()
	return len(fake.downloadProductFileArgsForCall)
}

func (fake *FakeClient) DownloadProductFileArgsForCall(i int) (*download.FileInfo, string, int, int, io.Writer) {
	fake.downloadProductFileMutex.RLock()
	defer fake.downloadProductFileMutex.RUnlock()
	return fake.downloadProductFileArgsForCall[i].writer, fake.downloadProductFileArgsForCall[i].productSlug, fake.downloadProductFileArgsForCall[i].releaseID, fake.downloadProductFileArgsForCall[i].productFileID, fake.downloadProductFileArgsForCall[i].progressWriter
}

func (fake *FakeClient) DownloadProductFileReturns(result1 error) {
	fake.DownloadProductFileStub = nil
	fake.downloadProductFileReturns = struct {
		result1 error
	}{result1}
}

func (fake *FakeClient) Invocations() map[string][][]interface{} {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.downloadProductFileMutex.RLock()
	defer fake.downloadProductFileMutex.RUnlock()
	return fake.invocations
}

func (fake *FakeClient) recordInvocation(key string, args []interface{}) {
	fake.invocationsMutex.Lock()
	defer fake.invocationsMutex.Unlock()
	if fake.invocations == nil {
		fake.invocations = map[string][][]interface{}{}
	}
	if fake.invocations[key] == nil {
		fake.invocations[key] = [][]interface{}{}
	}
	fake.invocations[key] = append(fake.invocations[key], args)
}
