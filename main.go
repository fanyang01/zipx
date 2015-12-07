package main

import (
	"archive/zip"
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

var (
	compress bool
	dir      string
	charset  string
	verbose  bool
	logv     *logger
)

func init() {
	flag.BoolVar(&compress, "z", false, "compress")
	flag.StringVar(&dir, "d", ".", "create a `dir`ectory to place uncompressed files")
	flag.StringVar(&charset, "c", "UTF-8", "`charset` of zip file, UTF-8 or GBK")
	flag.BoolVar(&verbose, "v", false, "verbosely")
	flag.Usage = func() {
		name := os.Args[0]
		fmt.Fprintf(os.Stderr, "USAGE\n")
		fmt.Fprintf(os.Stderr, "  uncompress: %s [OPTIONS] [ZIPFILE]\n", name)
		fmt.Fprintf(os.Stderr, "    compress: %s -z [OPTIONS] [ZIPFILE] FILE...\n", name)
		fmt.Fprintf(os.Stderr, "\nOPTIONS\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	logv = &logger{
		V: verbose,
	}
	log.SetFlags(log.Lshortfile)

	charset = strings.ToUpper(charset)
	switch charset {
	case "UTF-8", "GBK":
	default:
		log.Fatal("unsupported charset: " + charset)
	}
}

type logger struct {
	V bool
}

func (l *logger) Info(format string, args ...interface{}) {
	if l.V {
		fmt.Printf(format, args...)
	}
}

func fatalIf(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func main() {
	if compress {
		Zip()
		return
	}
	Unzip()
}

func Unzip() {
	var r io.Reader
	switch flag.NArg() {
	case 0:
		r = os.Stdin
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

	logv.Info("Charset: %s\n", charset)

	// TODO: handle big file
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

	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Fatal(err)
	}

	extract := func(f *zip.File) {
		rc, err := f.Open()
		if err != nil {
			log.Fatal(err)
		}
		defer rc.Close()

		name := f.Name
		// convert to UTF-8 from other charset.
		switch charset {
		case "GBK":
			name, _, err = transform.String(simplifiedchinese.GBK.NewDecoder(), name)
			if err != nil {
				log.Fatal(err)
			}
		}

		path := filepath.Join(dir, name)
		if f.FileInfo().IsDir() {
			logv.Info("Create directory: %s\n", path)
			if err := os.MkdirAll(path, f.Mode()); err != nil {
				log.Fatal(err)
			}
		} else {
			logv.Info("Write to file: %s\n", path)
			file, err := os.OpenFile(
				path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
			if err != nil {
				log.Fatal(err)
			}
			defer file.Close()

			if _, err = io.Copy(file, rc); err != nil {
				log.Fatal(err)
			}
		}
	}

	for _, f := range zr.File {
		extract(f)
	}
}

func Zip() {
	var pth, dst string
	switch flag.NArg() {
	case 0:
		pth = "."
	case 1:
		pth = flag.Arg(0)
	case 2:
		pth, dst = flag.Arg(0), flag.Arg(1)
	default:
		flag.Usage()
		os.Exit(1)
	}
	if dst == "" {
		abs, err := filepath.Abs(flag.Arg(0))
		if err != nil {
			log.Fatal(err)
		}
		dst = filepath.Base(abs) + ".zip"
	}
	zzip(pth, dst)
	return

}

func zzip(pth, dst string) {
	zf, err := os.OpenFile(
		dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		log.Fatal(err)
	}
	defer zf.Close()

	zstat, err := zf.Stat()
	if err != nil {
		log.Fatal(err)
	}

	buf := bufio.NewWriter(zf)
	// Create a new zip archive.
	w := zip.NewWriter(buf)

	var walk func(string, string)

	walk = func(full, base string) {
		f, err := os.Open(full)
		if err != nil {
			log.Fatal(err)
		}
		// this function may be called recursively
		// defer f.Close()

		fi, err := f.Stat()
		if err != nil {
			log.Fatal(err)
		}
		// could NOT include the zip file itself.
		if os.SameFile(zstat, fi) {
			f.Close()
			return
		}
		logv.Info("Add %s\n", full)

		header, err := zip.FileInfoHeader(fi)
		if err != nil {
			log.Fatal(err)
		}
		switch charset {
		case "GBK":
			if base, _, err = transform.String(
				simplifiedchinese.GBK.NewEncoder(), base); err != nil {
				log.Fatal(err)
			}
		}
		header.Name = base

		if fi.IsDir() {
			_, err := w.CreateHeader(header)
			if err != nil {
				log.Fatal(err)
			}
			names, err := f.Readdirnames(0)
			if err != nil {
				log.Fatal(err)
			}
			f.Close()

			for _, name := range names {
				if name == "." || name == ".." {
					continue
				}
				walk(filepath.Join(full, name), path.Join(base, name))
			}
		} else {
			reader := bufio.NewReader(f)
			writer, err := w.CreateHeader(header)
			if err != nil {
				log.Fatal(err)
			}
			_, err = io.Copy(writer, reader)
			if err != nil {
				log.Fatal(err)
			}
			f.Close()
		}
	}

	walk(pth, filepath.Base(pth))
	err = w.Close()
	if err != nil {
		log.Fatal(err)
	}
}
