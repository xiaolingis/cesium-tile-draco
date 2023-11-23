package tools

import (
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/mfbonfigli/gocesiumtiler/internal/tiler"
)

type FileFinder interface {
	GetLasFilesToProcess(opts *tiler.TilerOptions) []string
	GetLasFilesToMerge(opts *tiler.TilerOptions) []string
}

type StandardFileFinder struct{}

func NewStandardFileFinder() FileFinder {
	return &StandardFileFinder{}
}

func (f *StandardFileFinder) GetLasFilesToProcess(opts *tiler.TilerOptions) []string {
	// If folder processing is not enabled then las file is given by -input flag, otherwise look for las in -input folder
	// eventually excluding nested folders if Recursive flag is disabled
	if !opts.FolderProcessing {
		return []string{opts.Input}
	}

	return f.getLasFilesFromInputFolder(opts)
}

func (f *StandardFileFinder) getLasFilesFromInputFolder(opts *tiler.TilerOptions) []string {
	var lasFiles = make([]string, 0)

	baseInfo, _ := os.Stat(opts.Input)
	err := filepath.Walk(
		opts.Input,
		func(path string, info os.FileInfo, err error) error {
			if info.IsDir() && !opts.Recursive && !os.SameFile(info, baseInfo) {
				return filepath.SkipDir
			} else {
				if strings.ToLower(filepath.Ext(info.Name())) == ".las" {
					lasFiles = append(lasFiles, path)
				}
			}
			return nil
		},
	)

	if err != nil {
		log.Fatal(err)
	}

	return lasFiles
}

func (f *StandardFileFinder) GetLasFilesToMerge(opts *tiler.TilerOptions) []string {
	// If folder processing is not enabled then las file is given by -input flag, otherwise look for las in -input folder
	// eventually excluding nested folders if Recursive flag is disabled

	return f.getLasFilesFromInputSubFolder(opts)
}

func (f *StandardFileFinder) getLasFilesFromInputSubFolder(opts *tiler.TilerOptions) []string {
	var lasFiles = make([]string, 0)

	rootDir := strings.TrimSuffix(filepath.Join(opts.Input, "/"), "/") + "/"
	lasFileDepth := 1

	log.Println(opts.Input, rootDir)

	baseInfo, _ := os.Stat(opts.Input)
	err := filepath.Walk(
		rootDir,
		func(path string, info os.FileInfo, err error) error {
			pathDepth := strings.Count(strings.TrimPrefix(path, rootDir), string("/"))
			// log.Println("walk_path:", path, ", pathDepth:", pathDepth)

			if info.IsDir() && pathDepth >= lasFileDepth && !os.SameFile(info, baseInfo) {
				return filepath.SkipDir
			} else {
				if strings.ToLower(filepath.Ext(info.Name())) == ".las" {
					lasFiles = append(lasFiles, path)
				}
			}
			return nil
		},
	)

	if err != nil {
		log.Fatal(err)
	}

	return lasFiles
}
