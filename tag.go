/*
	tag - Uniquely identify a file / folder by
	prepending its hash to its name

	License: MIT	
*/


package main

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

type archive struct {
	dst *zip.Writer
}

func (a *archive) close() {
	a.dst.Close()
}

func (a *archive) pack(s string, d fs.DirEntry, err error) error {
	if err != nil {
		return(err)
	}
	s_fh, _ := os.Open(s)

	if !d.IsDir() {
		d_fh, _ := a.dst.Create(s)
		if _, e := io.Copy(d_fh, s_fh); e != nil {
			return(e)
		}
	}
	return(nil)
}

func archive_folder(path, name string) error {
	fhout, _ := os.Create(
		fmt.Sprintf("%s.zip", name),
	)
	arx := &archive{dst: zip.NewWriter(fhout)}

	if e := filepath.WalkDir(path, arx.pack); e != nil {
		return(e)
	}
	arx.close()
	return(nil)
}

func get_file_hash(f string) string {
	sha2 := sha256.New()

	fh, err1 := os.Open(f)
	if err1 != nil {
		fmt.Fprintf(os.Stdout, "File not found: %s\n", f)
		os.Exit(0) /* don't panic */
	}
	defer fh.Close()

	_, err2 := io.Copy(sha2, fh)
	if err2 != nil {
		fmt.Fprintf(os.Stdout, "Hashing error: %s\n", f)
		os.Exit(0)
	}

	h := hex.EncodeToString(sha2.Sum(nil))
	return(h[:12])
}

func copy_file(src, dst string) bool {

	fh_in, e1 := os.Open(src)
	fh_out, e2 := os.Create(dst)
	if (e1 != nil) || (e2 != nil) {
		return(false)
	}
	defer fh_in.Close()
	defer fh_out.Close()

	_, err := io.Copy(fh_out, fh_in)
	if err != nil {
		fmt.Fprintf(os.Stdout, "Copy error\n")
		return(false)
	}

	fh_out.Sync()
	return(true)
}

func hash_rename_in_place(name, digest string) {

	dir, fn := filepath.Split(name)
	new_name := fmt.Sprintf("%s%s_%s", dir, digest, fn)

	e := os.Rename(name, new_name)
	if e != nil {
		fmt.Fprintf(os.Stdout, "Could not rename: %s\n", name)
		os.Exit(-1)
	}

	println("> ", digest)
}

func copy_then_hash_rename(name, digest string) {

	errblurb := "Could not copy file\n"

	dir, fn := filepath.Split(name)
	new_name := fmt.Sprintf("%s%s_%s", dir, digest, fn)

	cp_ok := copy_file(name, new_name)

	if !cp_ok {
		fmt.Fprintf(os.Stdout, errblurb)
		return
	}

	cphash := get_file_hash(new_name)

	if cphash != digest {
		fmt.Fprintf(os.Stdout, "Post copy checksum mismatch\n")
		os.Exit(-1)
	}

	println("> ", digest)
}


func main() {
	usgblurb := "\nUsage: tag file/file-inplace/folder <filename/foldername>\n\n"

	if len(os.Args) != 3 {
		fmt.Fprintf(os.Stdout, usgblurb)
		os.Exit(0)
	}

	mode := os.Args[1]
	file := os.Args[2] /* can be a folder too */

	switch mode {
	case "folder":
		zip_name := fmt.Sprintf("%s.zip", file)
		if e := archive_folder(file, file); e != nil {
			println("Could not archive ", file)
		}
		hash := get_file_hash(zip_name)
		hash_rename_in_place(zip_name, hash)
	case "file":
		hash := get_file_hash(file)
		copy_then_hash_rename(file, hash)
	case "file-inplace":
		hash := get_file_hash(file)
		hash_rename_in_place(file, hash)
	default:
		fmt.Fprintf(os.Stdout, usgblurb)
	}
}
