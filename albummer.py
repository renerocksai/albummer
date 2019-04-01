import sys
import os
import markdown
import base64


img_extensions = ['.jpg', '.jpeg', '.png']
vid_extensions = ['.mpg', '.mpeg', '.mp4']

def help():
    print(f"""
    Usage: {sys.argv[0]} command options
    Where command can be:
    - make-template media_folder output_album_file.alb [num_cols=3] [order=asc] [custom.css=default.css]
        - media_folder : the folder containing images and videos
        - this will create the album file, ready for editing
    
    - generate album_file
        - generates single-file HTML from album_file, with extension .html
    """)
    sys.exit(0)


def main():
    if len(sys.argv) == 1:
        help()

    cmd = sys.argv[1]
    argv = sys.argv[2:]

    if cmd == 'make-template':
        make_template(argv)
    elif cmd == 'generate':
        generate(argv)
    else:
        help()


def abort(msg, exit_code=1):
    print(msg)
    sys.exit(exit_code)

def log(msg):
    print(msg)

def make_template(argv):
    def get_next_n_from_list(n, l):
        i = 0
        ret = []
        while l and i < n:
            ret.append(l.pop())
            i += 1
        return ret

    if len(argv) < 1:
        abort('Please specify a media folder and an output file name')
    elif len(argv) < 2:
        abort('Please specify an output file name')
    
    folder = argv[0]
    outfile = argv[1]
    css = 'default.css'
    num_cols = 3
    order = 'asc'
    if len(argv) > 2:
        num_cols = int(argv[2])
    if len(argv) > 3:
        order = argv[3]
    if len(argv) > 4:
        css = argv[4]
    
    all_media = []
    for f in os.listdir(folder):
        _, ext = os.path.splitext(f)
        if ext in img_extensions or ext in vid_extensions:
            f = os.path.join(folder, f)
            if not os.path.isfile(f):
                continue
            all_media.append(f)
    reverse = True                  # list needs to be reversed for asc, since we are going to pop
    if order != 'asc':
        reverse = False
    all_media.sort(key=os.path.getmtime, reverse=reverse)
    all_media = [os.path.basename(f) for f in all_media]

    # prepare media lines, video files will always be put on a single line
    media_body = ''
    line_len = 0
    while all_media:
        m = all_media.pop()
        _, ext = os.path.splitext(m)
        if ext in vid_extensions:
            # make a new line
            if line_len > 0:
                media_body += '\n'
            media_body += f'\n{m}\n\n'
            line_len = 0
        else:
            if line_len > 0:
                media_body += '   '
            media_body += m
            line_len += 1
            if line_len == num_cols:
                media_body += '\n'
                line_len = 0

    # create template
    title = os.path.basename(folder)

    with open(outfile, 'wt') as f:
        f.write(f""":folder {folder}
:show_filenames
:use default.css

# {title}

{media_body}
""")
    print(f'Generated {outfile}')


def generate(argv):
    if len(argv) < 1:
        abort('Please specify input file!')

    with open(argv[0], 'rt') as f:
        lines = [l.strip() for l in f.readlines()]
    
    lines.reverse()

    folder = None
    show_filenames = False
    css = 'default.css'
    all_media = []


    html_body = ''
    html_head = ''

    while lines:
        line = lines.pop()
        if not line:
            continue
        if line.startswith(':'):
            cols = line.split()
            if cols[0] == ':folder':
                folder = cols[1]
                all_media = []
                for f in os.listdir(folder):
                    _, ext = os.path.splitext(f)
                    if ext in img_extensions or ext in vid_extensions:
                        f = os.path.join(folder, f)
                        if not os.path.isfile(f):
                            continue
                        all_media.append(os.path.basename(f))
            elif cols[0] == ':show_filenames':
                show_filenames = True
            elif cols[0] == ':use':
                css = cols[1]
                with open(css, 'rt') as f:
                    css = f.read()
                    html_head = f'<style>{css}</style>'
        else:
            cols = line.split()
            if cols[0] in all_media:
                # we have a media line
                print('MEDIA     : ', line)
                num_cols = len(cols)
                percent = int(100 / num_cols)
                html = '<table><tr style="width:100%">'
                for col in cols:
                    html += f'<td style="width:{percent}%">'
                    _, ext = os.path.splitext(col)
                    if ext in img_extensions:
                        data = img_to_html(folder, col)
                    else:
                        data = vid_to_html(folder, col)
                    html += data
                    html += '</td>'
                html += '</tr></table>\n'
                html_body += html
            else:
                # we have a markdown block beginning
                markdown_lines = [line]
                while lines:
                    line = lines.pop()
                    if not line:
                        markdown_lines.append(line)
                        continue
                    cols = line.split()
                    if cols[0] in all_media:
                        # we have a media line -> end of markdown, put it back
                        lines.append(line)
                        break
                    markdown_lines.append(line)
                print('--')
                for line in markdown_lines:
                    print('MARKDOWN  : ', line)
                print('--')
                
                md = '\n'.join(markdown_lines)
                html = markdown.markdown(md)
                html_body += html + '\n'

    ofile = argv[0]
    ofile, ext = os.path.splitext(ofile)
    ofile += '.html'
    print('Writing html...')
    with open(ofile, 'wt') as f:
        f.write(f'<!DOCTYPE html><html><head>{html_head}</head>\n<body>{html_body}</body>\n</html>')
    print(f'Generated {ofile}')


def img_to_html(folder, img):
    f = os.path.join(folder, img)
    if img.lower().endswith('.png'):
        imgformat = 'png'
    else:
        imgformat = 'jpeg'
    data = base64.b64encode(open(f, 'rb').read()).decode('utf-8').replace('\n', '')  
    return f'<img width="100%" src="data:image/{imgformat};base64,{data}"></img>'


def vid_to_html(folder, vid):
    f = os.path.join(folder, vid)
    data = base64.b64encode(open(f, 'rb').read()).decode('utf-8').replace('\n', '')  
    return f'<video width="100%" controls src="data:video/mp4;base64,{data}"></video>'


if __name__ == '__main__':
    main()
