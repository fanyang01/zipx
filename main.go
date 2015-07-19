package main

import (
	"archive/zip"
	"bytes"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

var (
	dir      = flag.String("d", "", "create a `directory` and change to it")
	encoding = flag.String("e", "UTF-8", "encoding of zip file, support UTF-8 and GBK")
	verbose  = flag.Bool("v", false, "verbosely process")
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "%s - uncompress zip file\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Usage: %s [OPTIONS] ZIPFILE\n\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()

	log.SetFlags(log.Lshortfile)

	var r io.Reader
	switch flag.NArg() {
	// case 0:
	// 	r = os.Stdin
	case 1:
		f, err := os.Open(flag.Arg(0))
		if err != nil {
			log.Fatal(err)
		}
		defer f.Close()
		r = f
	default:
		flag.Usage()
		os.Exit(1)
	}

	if *verbose {
		fmt.Printf("Encoding: %s\n", strings.ToUpper(*encoding))
	}
	switch strings.ToLower(*encoding) {
	case "utf-8", "gbk":
	default:
		log.Fatal("unsupported encoding: " + *encoding)
	}

	// TODO: how to handle big file?
	b, err := ioutil.ReadAll(r)
	if err != nil {
		log.Fatal(err)
	}

	// bytes.Reader implements io.ReaderAt
	ra := bytes.NewReader(b)
	zr, err := zip.NewReader(ra, int64(len(b)))
	if err != nil {
		log.Fatal(err)
	}

	dest := "."
	if *dir != "" {
		err := os.MkdirAll(*dir, 0755)
		if err != nil {
			log.Fatal(err)
		}
		dest = *dir
	}

	extract := func(f *zip.File) {
		rc, err := f.Open()
		if err != nil {
			log.Fatal(err)
		}
		defer rc.Close()

		name := f.Name
		switch strings.ToLower(*encoding) {
		case "utf-8":
		case "gbk":
			name, _, err = transform.String(simplifiedchinese.GBK.NewDecoder(), name)
			if err != nil {
				log.Fatal(err)
			}
		}

		path := filepath.Join(dest, name)
		if f.FileInfo().IsDir() {
			if *verbose {
				fmt.Printf("Create directory %s\n", path)
			}
			err := os.MkdirAll(path, f.Mode())
			if err != nil {
				log.Fatal(err)
			}
		} else {
			if *verbose {
				fmt.Printf("Write to file %s\n", path)
			}
			file, err := os.OpenFile(
				path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
			if err != nil {
				log.Fatal(err)
			}
			_, err = io.Copy(file, rc)
			if err != nil {
				log.Fatal(err)
			}
			file.Close()
		}
	}

	for _, f := range zr.File {
		extract(f)
	}
}
