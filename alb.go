package main

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/golang-commonmark/markdown"
	//	"github.com/pkg/profile"
)

var img_extensions = map[string]int{".png": 1, ".jpg": 1, ".jpeg": 1}
var vid_extensions = map[string]int{".mp4": 1}

const MEDIA_TYPE_IMG = 0
const MEDIA_TYPE_VID = 1

type MediaFile struct {
	path       string
	media_type int
	mtime      time.Time
}

// We create a collection type MediaFiles, as array of MediaFile structs
// Then we implement the Sort interface: Len(), Swap(), Less() - to sort by
// mtime
type MediaFiles []MediaFile

func (m MediaFiles) Len() int {
	return len(m)
}

func (m MediaFiles) Swap(i, j int) {
	m[i], m[j] = m[j], m[i]
}

func (m MediaFiles) Less(i, j int) bool {
	return m[i].mtime.Before(m[j].mtime)
}

// turn list into map[basename] -> MediaFile
func (m MediaFiles) ToMap() map[string]MediaFile {
	var ret map[string]MediaFile
	ret = make(map[string]MediaFile)

	for _, mf := range m {
		_, fn := filepath.Split(mf.path)
		ret[fn] = mf
	}
	return ret
}

func get_exe_folder() string {
	exe, _ := os.Executable()
	path, _ := filepath.Split(exe)
	return path
}

func abort(msg string, exit_code int) {
	fmt.Println(msg)
	os.Exit(exit_code)
}

func help() {
	fmt.Printf(usage, os.Args[0])
	os.Exit(0)
}

func main() {
	//	defer profile.Start().Stop()
	if len(os.Args) == 1 {
		help()
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	switch cmd {
	case "make-template":
		make_template(args)
	case "generate":
		generate(args)
	default:
		help()
	}
}

func get_lower_extension(path string) string {
	return filepath.Ext(strings.ToLower(path))
}

func make_template(args []string) {
	if len(args) < 1 {
		abort("Please specify a media folder and an output filename", 1)
	}
	if len(args) < 2 {
		abort("Please specify an output filename", 1)
	}

	folder := args[0]
	outfile := args[1]
	css := filepath.Join(get_exe_folder(), "default.css")
	num_cols := 3
	order := "asc"

	if len(args) > 2 {
		n, err := strconv.Atoi(args[2])
		num_cols = n
		if err != nil {
			num_cols = 3
		}
	}

	if len(args) > 3 {
		order = args[3]
	}

	if len(args) > 4 {
		css = args[4]
	}

	all_media, err := get_all_media(folder)
	if order == "asc" {
		sort.Sort(MediaFiles(all_media))
	} else {
		sort.Sort(sort.Reverse(MediaFiles(all_media)))
	}
	if err != nil {
		abort(err.Error(), 1)
	}

	var media_body string = ""
	var line_len int = 0

	for _, m := range all_media {
		_, fn := filepath.Split(m.path)
		if m.media_type == MEDIA_TYPE_VID {
			if line_len > 0 {
				media_body += "\n"
			}
			media_body += fmt.Sprintf("\n%s\n\n", fn)
			line_len = 0
		} else {
			if line_len > 0 {
				media_body += "   "
			}
			media_body += fn
			line_len += 1
			if line_len == num_cols {
				media_body += "\n"
				line_len = 0
			}
		}
	}

	abs_folder, err := filepath.Abs(folder)
	_, title := filepath.Split(abs_folder)

	f, err := os.Create(outfile)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	w := bufio.NewWriter(f)
	_, err = w.WriteString(fmt.Sprintf(":folder %s\n:show_filenames\n:use %s\n\n# %s\n\n%s\n", folder, css, title, media_body))
	if err != nil {
		panic(err)
	}
	w.Flush()
}

func generate(args []string) {
	if len(args) < 1 {
		abort("Please specify an input file!", 1)
	}

	input_file := args[0]
	f, err := os.Open(input_file)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)

	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		panic(err)
	}

	var folder string
	var css string
	//	show_filenames := false
	var all_media map[string]MediaFile

	var html_bodies []string
	var html_head string

	markdown_parser := markdown.New()
	lc := 0
	lc_max := len(lines)

	for lc < lc_max {
		line := lines[lc]
		lc += 1

		if len(line) == 0 {
			continue
		}
		if line[0] == ':' {
			// we have a control line
			cols := strings.Fields(line)
			switch cols[0] {
			case ":folder":
				folder = cols[1]
				all_media_list, err := get_all_media(folder)
				if err != nil {
					panic(err)
				}
				all_media = all_media_list.ToMap()
			case ":show_filenames":
				//				show_filenames = true
			case ":use":
				css = cols[1]
				css_text, err := ioutil.ReadFile(css)
				if err == nil {
					html_head = fmt.Sprintf("<style>%s</style>",
						string(css_text))
				}
			} // end switch
		} else {
			// we have a media or markdown line
			cols := strings.Fields(line)
			if _, ok := all_media[cols[0]]; ok {
				// we have a media line
				num_cols := len(cols)
				percent := int(100 / num_cols)
				html := `<div align="center"><table><tr>`
				for _, col := range cols {
					html += fmt.Sprintf(`<td style="width:%d%%;">`, percent)
					if media_file, ok := all_media[col]; ok {
						var data string
						switch media_file.media_type {
						case MEDIA_TYPE_IMG:
							data = img_to_html(folder, col)
						case MEDIA_TYPE_VID:
							data = vid_to_html(folder, col)
						}
						html += data
						html += `</td><td width="10px"></td>`
					}
				}
				html += `</tr></table></div>`
				html_bodies = append(html_bodies, html)
			} else {
				// markdown block
				markdown_lines := line
				for lc < lc_max {
					line = lines[lc]
					lc += 1
					if len(line) == 0 {
						markdown_lines += "\n" + line
						continue
					}
					cols = strings.Fields(line)
					if _, ok := all_media[cols[0]]; ok {
						// we have a media line -> end of markdown, put it back
						lc -= 1
						break
					}
					markdown_lines += "\n" + line
				}
				html := markdown_parser.RenderToString([]byte(markdown_lines))
				html_bodies = append(html_bodies, html)
			}
		}
	}

	ext := filepath.Ext(input_file)
	out_file := strings.Replace(input_file, ext, ".html", 1)
	of, err := os.Create(out_file)
	if err != nil {
		panic(err)
	}
	defer of.Close()

	w := bufio.NewWriter(of)
	_, err = w.WriteString(fmt.Sprintf("<!DOCTYPE html><html><head>%s</head>\n<body>", html_head))
	if err != nil {
		panic(err)
	}
	for _, html_body := range html_bodies {
		_, err = w.WriteString(html_body)
		if err != nil {
			panic(err)
		}
	}
	_, err = w.WriteString("</body>\n</html>")
	if err != nil {
		panic(err)
	}
	w.Flush()
}

func get_all_media(root string) (MediaFiles, error) {
	var files MediaFiles

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() {
			ext := get_lower_extension(path)
			_, is_img := img_extensions[ext]
			_, is_vid := vid_extensions[ext]

			var media_type int = MEDIA_TYPE_IMG
			if is_vid {
				media_type = MEDIA_TYPE_VID
			}
			if is_img || is_vid {
				files = append(files, MediaFile{path, media_type, info.ModTime()})
			}
		}
		return nil
	})
	return files, err
}

func img_to_html(folder string, img string) string {
	data, err := ioutil.ReadFile(filepath.Join(folder, img))
	if err != nil {
		return ""
	}
	ext := filepath.Ext(strings.ToLower(img))
	var img_format string
	if ext == ".png" {
		img_format = "png"
	} else {
		img_format = "jpeg"
	}
	return fmt.Sprintf(`<img width="100%%" src="data:image/%s;base64,%s"></img>`, img_format, base64.StdEncoding.EncodeToString(data))
}

func vid_to_html(folder string, vid string) string {
	data, err := ioutil.ReadFile(filepath.Join(folder, vid))
	if err != nil {
		return ""
	}
	return fmt.Sprintf(`<video width="100%%" controls src="data:video/mp4;base64,%s"></video>`, base64.StdEncoding.EncodeToString(data))
}

var usage = `Usage: %s command options [global flags]
Where command can be:
  make-template media_folder output.alb [num_cols] [order] [custom.css]
    This will create the album file, ready for editing, as the first step 
    of creating an HTML album.

    Arguments:
    - media_folder : the folder containing images and videos
    - output.alb   : the album file to be generated
    - num_cols     : optional, default=3. The number of columns to use when 
                     laying out images.  Videos will always be placed on a 
                     separate line.
    - order        : optional, default=asc : Sort order of the media, by file 
                     timestamp. If you specify anything other than asc, tben 
                     descending order (newest first) will be used.
    - custom.css   : optional, default=default.css : for pros: specify your 
                     custom CSS file
   
  generate album_file
    Generates the single-file HTML from an album file, with extension .html

    Arguments:
    - album_file   : the album file to be converted. If album_file is 
                     my_fotos.alb, the generated HTML file will be named 
                     my_fotos.html
Global Flags:
  -v               : If you pass in -v, verbose output will be displayed 
`
