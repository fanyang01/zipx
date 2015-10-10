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
	uncompress = flag.Bool("x", false, "uncompress")
	dir        = flag.String("d", "", "create a `dir`ectory and change to it")
	charset    = flag.String("c", "UTF-8", "`charset` of zip file, support UTF-8 and GBK")
	verbose    = flag.Bool("v", false, "verbosely")
	logv       *logger
)

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
	flag.Usage = func() {
		name := os.Args[0]
		fmt.Fprintf(os.Stderr, "%s - compress/uncompress\n\n", name)
		fmt.Fprintf(os.Stderr, "Usage:\n")
		fmt.Fprintf(os.Stderr, "\t%s path [ZIPFILE]\n", name)
		fmt.Fprintf(os.Stderr, "\t%s -x [-d DIR] ZIPFILE\n\n", name)
		flag.PrintDefaults()
	}
	flag.Parse()
	log.SetFlags(log.Lshortfile)
	logv = &logger{
		V: *verbose,
	}

	if !*uncompress {
		switch flag.NArg() {
		case 1:
			abs, err := filepath.Abs(flag.Arg(0))
			if err != nil {
				log.Fatal(err)
			}
			zipDir(flag.Arg(0), filepath.Base(abs)+".zip")
		case 2:
			zipDir(flag.Arg(0), flag.Arg(1))
		default:
			flag.Usage()
			os.Exit(1)
		}
		return
	}

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

	logv.Info("Encoding: %s\n", strings.ToUpper(*charset))
	switch strings.ToLower(*charset) {
	case "utf-8", "gbk":
	default:
		log.Fatal("unsupported charset: " + *charset)
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
		switch strings.ToLower(*charset) {
		case "utf-8":
		case "gbk":
			name, _, err = transform.String(simplifiedchinese.GBK.NewDecoder(), name)
			if err != nil {
				log.Fatal(err)
			}
		}

		path := filepath.Join(dest, name)
		if f.FileInfo().IsDir() {
			logv.Info("Create directory %s\n", path)
			err := os.MkdirAll(path, f.Mode())
			if err != nil {
				log.Fatal(err)
			}
		} else {
			logv.Info("Write to file %s\n", path)
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

func zipDir(pth, dst string) {
	file, err := os.OpenFile(
		dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		log.Fatal(err)
	}
	dstAbs, err := filepath.Abs(dst)
	if err != nil {
		log.Fatal(err)
	}
	buf := bufio.NewWriter(file)

	var gbk bool
	switch strings.ToLower(*charset) {
	case "utf-8":
	case "gbk":
		gbk = true
	default:
		log.Fatal("unsupported charset: " + *charset)
	}
	// Create a new zip archive.
	w := zip.NewWriter(buf)

	var walk func(string, string)
	walk = func(p, pname string) {
		logv.Info("%s\n", p)
		f, err := os.Open(p)
		if err != nil {
			log.Fatal(err)
		}
		fi, err := f.Stat()
		if err != nil {
			log.Fatal(err)
		}
		if gbk {
			pname, _, err = transform.String(simplifiedchinese.GBK.NewEncoder(), pname)
			if err != nil {
				log.Fatal(err)
			}
		}
		header, err := zip.FileInfoHeader(fi)
		if err != nil {
			log.Fatal(err)
		}
		header.Name = pname
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
				abs, err := filepath.Abs(filepath.Join(p, name))
				if err != nil {
					log.Fatal(err)
				}
				if abs == dstAbs {
					continue
				}
				walk(filepath.Join(p, name), path.Join(pname, name))
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
	file.Close()
}
