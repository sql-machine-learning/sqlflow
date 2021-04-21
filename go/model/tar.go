package model

import (
	"archive/tar"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func tarFile(filesource string, sfileInfo os.FileInfo, tarwriter *tar.Writer) error {
	sfile, err := os.Open(filesource)
	if err != nil {
		fmt.Println(err)
		return err
	}
	defer sfile.Close()
	header, err := tar.FileInfoHeader(sfileInfo, "")
	if err != nil {
		fmt.Println(err)
		return err
	}
	header.Name = filesource
	err = tarwriter.WriteHeader(header)
	if err != nil {
		fmt.Println(err)
		return err
	}
	if _, err = io.Copy(tarwriter, sfile); err != nil {
		fmt.Println(err)
		return err
	}
	return nil
}

func tarFolder(directory string, tarwriter *tar.Writer) error {
	return filepath.Walk(directory, func(targetpath string, file os.FileInfo, err error) error {
		//read the file failure
		if file == nil {
			return err
		}
		if file.IsDir() {
			if directory == targetpath {
				return nil
			}
			header, err := tar.FileInfoHeader(file, "")
			if err != nil {
				return err
			}
			header.Name = filepath.Join(directory, strings.TrimPrefix(targetpath, directory))
			if err = tarwriter.WriteHeader(header); err != nil {
				return err
			}
			os.Mkdir(strings.TrimPrefix(directory, file.Name()), os.ModeDir)
			//如果压缩的目录里面还有目录，则递归压缩
			return tarFolder(targetpath, tarwriter)
		}
		return tarFile(targetpath, file, tarwriter)
	})
}
