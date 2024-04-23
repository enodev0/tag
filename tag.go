/*
	tag - Uniquely identify a file / folder by
	prepending its hash to its name

	License: MIT
*/

package main

import (
	"archive/zip"
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

func get_home_dir() string {
	hdir, err := os.UserHomeDir()
	if err != nil {
		return ("NA")
	}
	return (hdir)
}

func get_working_dir() string {
	wdir, err := os.Getwd()
	if err != nil {
		return ("NA")
	}
	return (wdir)
}

var HOME string = get_home_dir()
var WDIR string = get_working_dir()
var SYNC bool = true
var CONF_PATH string = HOME + "/.tag/config"
var SYNC_PATH string = HOME + "/.tag/sync"

type archive struct {
	dst *zip.Writer
}

func (a *archive) close() {
	a.dst.Close()
}

func (a *archive) pack(s string, d fs.DirEntry, err error) error {

	if err != nil {
		return (err)
	}
	s_fh, _ := os.Open(s)

	if !d.IsDir() {
		d_fh, _ := a.dst.Create(s)
		if _, e := io.Copy(d_fh, s_fh); e != nil {
			return (e)
		}
	}
	return (nil)
}

func archive_folder(path, name string) bool {

	fhout, _ := os.Create(
		fmt.Sprintf("%s.zip", name),
	)
	arx := &archive{dst: zip.NewWriter(fhout)}

	if e := filepath.WalkDir(path, arx.pack); e != nil {
		return (false)
	}
	arx.close()
	return (true)
}

func file_digest(f string) string {

	sha2 := sha256.New()

	fh, err1 := os.Open(f)
	if err1 != nil {
		fmt.Printf("E: File not found: %s\n", f)
		os.Exit(-1)
	}
	defer fh.Close()

	_, err2 := io.Copy(sha2, fh)
	if err2 != nil {
		fmt.Printf("E: Digest error: %s\n", f)
		os.Exit(-1)
	}

	h := hex.EncodeToString(sha2.Sum(nil))
	return (h[:12])
}

func copy_file(src, dst string) {

	fh_in, e1 := os.Open(src)
	if e1 != nil {
		fmt.Printf("E: Could not open: %s\n", src)
		os.Exit(-1)
	}
	fh_out, e2 := os.Create(dst)
	if e2 != nil {
		fmt.Printf("E: Could not create: %s\n", dst)
		os.Exit(-1)
	}

	defer fh_in.Close()
	defer fh_out.Close()

	_, err := io.Copy(fh_out, fh_in)
	if err != nil {
		fmt.Printf("E: Could not copy to: %s\n", dst)
		os.Exit(-1)
	}

	fh_out.Sync()
}

func hash_rename_in_place(name, digest string) string {

	dir, fn := filepath.Split(name)
	new_name := fmt.Sprintf("%s%s_%s", dir, digest, fn)

	e := os.Rename(name, new_name)
	if e != nil {
		fmt.Printf("E: Could not rename: %s\n", name)
		return ("NA")
	}

	println("> ", digest)
	return (new_name) /* for sync */
}

func verify_copy(original, copied string) {

	orig_sha2 := file_digest(original)
	copy_sha2 := file_digest(copied)

	if copy_sha2 != orig_sha2 {
		println("E: Checksum verification failed")
		e := os.Remove(copied)
		if e != nil {
			println("E: Corrupted file cleanup failed")
		} else {
			println("W: Corrupted file removed")
		}
		println("Abort")
		os.Exit(-1)
	}
}

func copy_then_hash_rename(name, digest string) string {

	dir, fn := filepath.Split(name)
	new_name := fmt.Sprintf("%s%s_%s", dir, digest, fn)

	copy_file(name, new_name)
	verify_copy(name, new_name)

	println("> ", digest)
	return (new_name) /* for sync */
}

func parse_configs(path, delim string) (map[string]string, bool) {

	ok := true
	conf := make(map[string]string)
	conf_fh, e := os.Open(path)
	if e != nil {
		println("E: Could not open: ", path)
		os.Exit(-1)
	}
	defer conf_fh.Close()

	scanner := bufio.NewScanner(conf_fh)

	for scanner.Scan() {
		line := scanner.Text()
		line = strings.ReplaceAll(line, " ", "")
		if strings.Contains(line, "#") {
			continue /* comment line */
		}
		chunks := strings.Split(line, delim)
		if len(chunks) != 2 {
			ok = false
		}
		conf[chunks[0]] = chunks[1]
	}

	return conf, ok
}

func sync_file(file string) {

	if !SYNC {
		println("W: Syncing disabled")
		return
	}
	sync_paths, ok := parse_configs(SYNC_PATH, ",")

	if !ok {
		println("E: Parse error: ", SYNC_PATH)
		SYNC = false
		return
	}
	i := 0
	len_s := len(sync_paths)

	println("\nSyncing, please wait ...")
	for ID, PATH := range sync_paths {
		i = i + 1
		fmt.Printf("Location (%d / %d):  %s \n", i, len_s, ID)
		new_name := filepath.Clean(PATH) + "/" + file
		copy_file(file, new_name)
		verify_copy(file, new_name)
	}
}

func main() {
	//
	blurb := "\nUsage: tag file/file-inplace/folder <filename/foldername>\n\n"

	if runtime.GOOS == "windows" {
		println("E: Platform not supported")
		os.Exit(0)
	}

	/* if we can't find $HOME, we won't be reading the config
	 * in the first place */
	if HOME == "NA" {
		println("W: Could not locate $HOME. Syncing will be disabled.")
		SYNC = false
	}

	conf, ok := parse_configs(CONF_PATH, "=")

	if !ok {
		fmt.Printf("E: Parse error: %s\n", CONF_PATH)
	} else {
		if status, ok := conf["sync"]; ok {
			switch status {
			case "enabled":
				SYNC = true
			case "disabled":
				println("W: Syncing is disabled globally")
				SYNC = false
			default:
				SYNC = true
			}
		}
	}

	if len(os.Args) != 3 {
		println(blurb)
		os.Exit(-1)
	}

	var mode string = os.Args[1]
	var target string = os.Args[2]

	handle_folders := func(target string) string {
		println("Archiving, please wait ...")
		zip_name := fmt.Sprintf("%s.zip", target)
		if !archive_folder(target, target) {
			fmt.Printf("E: Could not archive %s\n", target)
			os.Exit(-1)
		}
		hash := file_digest(zip_name)
		return (hash_rename_in_place(zip_name, hash))
	}

	handle_file := func(target string) string {
		hash := file_digest(target)
		return (copy_then_hash_rename(target, hash))
	}

	handle_file_inplace := func(target string) string {
		hash := file_digest(target)
		return (hash_rename_in_place(target, hash))
	}

	switch mode {
	case "folder":
		out := handle_folders(target)
		sync_file(out)
	case "folder-nosync":
		_ = handle_folders(target)
	case "file":
		out := handle_file(target)
		sync_file(out)
	case "file-nosync":
		_ = handle_file(target)
	case "file-inplace":
		out := handle_file_inplace(target)
		sync_file(out)
	case "file-inplace-nosync":
		_ = handle_file_inplace(target)
	default:
		println(blurb)
	}

	println("Done")
}
