package main

import (
	"bufio"
	"encoding/base64"
	"errors"
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
	html       string
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

// turn list into map[basename] -> *MediaFile
func (m MediaFiles) ToMap() map[string]*MediaFile {
	var ret map[string]*MediaFile
	ret = make(map[string]*MediaFile)

	for _, mf := range m {
		_, fn := filepath.Split(mf.path)
		p := new(MediaFile)
		*p = mf
		ret[fn] = p
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
	fmt.Println("Generated", outfile)
}

func parse_folder(lines []string) (string, error) {
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		if line[0] == ':' {
			// we have a control line
			cols := strings.Fields(line)
			switch cols[0] {
			case ":folder":
				folder := cols[1]
				return folder, nil
			}
		}
	}
	return "", errors.New("No folder in album file")
}

func load_media(lines []string, folder string, all_media *map[string]*MediaFile) {
	c := make(chan int)
	num_media := 0
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		if line[0] == ':' {
			continue
		} else {
			// we have a media or markdown line
			cols := strings.Fields(line)
			if _, ok := (*all_media)[cols[0]]; ok {
				// we have a media line
				for _, col := range cols {
					if media_file, ok := (*all_media)[col]; ok {
						switch (*media_file).media_type {
						case MEDIA_TYPE_IMG:
							go func(media_file *MediaFile, col string, c chan int) {
								(*media_file).html = img_to_html(folder, col)
								c <- 1
							}(media_file, col, c)
							num_media++
						case MEDIA_TYPE_VID:
							go func(media_file *MediaFile, col string, c chan int) {
								(*media_file).html = vid_to_html(folder, col)
								c <- 1
							}(media_file, col, c)
							num_media++
						}
					}
				}
			}
		}
	}

	for i := 0; i < num_media; i++ {
		fmt.Print(fmt.Sprintf("\r  Loading image / video %4d of %-4d ", i+1, num_media))
		// wait for completion
		_ = <-c
	}
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
	var all_media map[string]*MediaFile

	var html_bodies []string
	var html_head string

	markdown_parser := markdown.New()
	lc := 0
	lc_max := len(lines)

	folder, err = parse_folder(lines)
	if err != nil {
		abort("No folder in album file!", 1)
	}

	all_media_list, err := get_all_media(folder)
	if err != nil {
		panic(err)
	}
	all_media = all_media_list.ToMap()

	fmt.Println("The Albummer is processing", input_file)
	load_media(lines, folder, &all_media)
	fmt.Println()

	for lc < lc_max {
		line := lines[lc]
		lc += 1

		fmt.Print(fmt.Sprintf("\r  Generating for line   %4d of %-4d ", lc, lc_max))
		if len(line) == 0 {
			continue
		}
		if line[0] == ':' {
			// we have a control line
			cols := strings.Fields(line)
			switch cols[0] {
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
						html += (*media_file).html
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
	fmt.Println()

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
	num_bodies := len(html_bodies)
	for index, html_body := range html_bodies {
		fmt.Print(fmt.Sprintf("\r  Writing HTML body     %4d of %-4d ", index+1, num_bodies))
		_, err = w.WriteString(html_body)
		if err != nil {
			panic(err)
		}
	}
	fmt.Println()
	_, err = w.WriteString("</body>\n</html>")
	if err != nil {
		panic(err)
	}
	w.Flush()
	fmt.Println("Generated", out_file)
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
				files = append(files, MediaFile{path, media_type, info.ModTime(), ""})
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
	return fmt.Sprintf(`<div align="center"><video width="auto"  max-width="100%%" height="640px" controls src="data:video/mp4;base64,%s"></video></div>`, base64.StdEncoding.EncodeToString(data))
}

var usage = `Usage: %s command options 
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
                     timestamp. If you specify anything other than asc, then 
                     descending order (newest first) will be used.
    - custom.css   : optional, default=default.css : for pros: specify your 
                     custom CSS file
   
  generate album_file
    Generates the single-file HTML from an album file, with extension .html

    Arguments:
    - album_file   : the album file to be converted. If album_file is 
                     my_fotos.alb, the generated HTML file will be named 
                     my_fotos.html
`
